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
