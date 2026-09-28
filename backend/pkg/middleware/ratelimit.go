package middleware

import (
    "encoding/json"
    "net"
    "net/http"
    "sync"
    "time"
)

// RateLimiter throttles repeated requests per client IP using a sliding
// window: at most `max` requests within `window`. Used to slow down
// brute-force password guessing against /api/login.
type RateLimiter struct {
    mu       sync.Mutex
    attempts map[string][]time.Time
    max      int
    window   time.Duration
}

func NewRateLimiter(max int, window time.Duration) *RateLimiter {
    return &RateLimiter{
        attempts: make(map[string][]time.Time),
        max:      max,
        window:   window,
    }
}

// ClientIP extracts the request's remote IP, stripping the port. Exported
// so other packages (e.g. session-creation code) can tag data with the same
// IP the rate limiter itself keys on.
func ClientIP(r *http.Request) string {
    host, _, err := net.SplitHostPort(r.RemoteAddr)
    if err != nil {
        return r.RemoteAddr
    }
    return host
}

func (rl *RateLimiter) Limit(next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        ip := ClientIP(r)
        now := time.Now()
        cutoff := now.Add(-rl.window)

        rl.mu.Lock()
        var recent []time.Time
        for _, t := range rl.attempts[ip] {
            if t.After(cutoff) {
                recent = append(recent, t)
            }
        }
        if len(recent) >= rl.max {
            rl.attempts[ip] = recent
            rl.mu.Unlock()
            w.Header().Set("Content-Type", "application/json")
            w.WriteHeader(http.StatusTooManyRequests)
            json.NewEncoder(w).Encode(map[string]string{
                "error": "Too many attempts, please try again later",
            })
            return
        }
        rl.attempts[ip] = append(recent, now)
        rl.mu.Unlock()

        next(w, r)
    }
}

// statusRecorder remembers the status code the wrapped handler wrote, so
// LimitFailures can tell a failed request from a successful one.
type statusRecorder struct {
    http.ResponseWriter
    status int
}

func (sr *statusRecorder) WriteHeader(code int) {
    sr.status = code
    sr.ResponseWriter.WriteHeader(code)
}

// LimitFailures is like Limit, but only requests the wrapped handler
// answers with failStatus (e.g. 401 for a wrong password) count towards
// the per-IP limit. Successful requests pass freely, so a user who logs in
// correctly never locks themselves out, while repeated failures from one
// IP (e.g. password spraying across many accounts) are still stopped.
func (rl *RateLimiter) LimitFailures(failStatus int, next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        ip := ClientIP(r)
        if rl.Blocked(ip) {
            w.Header().Set("Content-Type", "application/json")
            w.WriteHeader(http.StatusTooManyRequests)
            json.NewEncoder(w).Encode(map[string]string{
                "error": "Too many attempts, please try again later",
            })
            return
        }

        rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
        next(rec, r)
        if rec.status == failStatus {
            rl.Record(ip)
        }
    }
}

// recentLocked returns key's attempts still inside the window, dropping
// older ones. rl.mu must be held.
func (rl *RateLimiter) recentLocked(key string) []time.Time {
    cutoff := time.Now().Add(-rl.window)
    var recent []time.Time
    for _, t := range rl.attempts[key] {
        if t.After(cutoff) {
            recent = append(recent, t)
        }
    }
    if len(recent) == 0 {
        delete(rl.attempts, key)
    } else {
        rl.attempts[key] = recent
    }
    return recent
}

// Blocked reports whether key has already reached max attempts within the
// window, without recording a new one.
func (rl *RateLimiter) Blocked(key string) bool {
    rl.mu.Lock()
    defer rl.mu.Unlock()
    return len(rl.recentLocked(key)) >= rl.max
}

// Record records one attempt for key.
func (rl *RateLimiter) Record(key string) {
    rl.mu.Lock()
    defer rl.mu.Unlock()
    rl.attempts[key] = append(rl.recentLocked(key), time.Now())
}

// Reset forgets every attempt recorded for key (e.g. after a successful
// login, so earlier typos don't count towards a later lockout).
func (rl *RateLimiter) Reset(key string) {
    rl.mu.Lock()
    defer rl.mu.Unlock()
    delete(rl.attempts, key)
}

// RecordAndCheck records one attempt for an arbitrary key (as opposed to
// Limit, which always keys on the request's IP) and reports whether the
// number of attempts within the window has now reached max. Used to track
// failed login attempts per-email, so a distributed brute-force attempt
// (many IPs, one target account) can be caught even though each individual
// IP stays under Limit's per-IP threshold.
func (rl *RateLimiter) RecordAndCheck(key string) bool {
    now := time.Now()
    cutoff := now.Add(-rl.window)

    rl.mu.Lock()
    defer rl.mu.Unlock()

    var recent []time.Time
    for _, t := range rl.attempts[key] {
        if t.After(cutoff) {
            recent = append(recent, t)
        }
    }
    recent = append(recent, now)
    rl.attempts[key] = recent
    return len(recent) >= rl.max
}
