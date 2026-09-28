package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestClientIP_StripsPort covers the ordinary case: RemoteAddr always
// carries a port, which callers (rate limiting, session IP tracking) don't
// want mixed into the key.
func TestClientIP_StripsPort(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "203.0.113.5:54321"

	if got := ClientIP(r); got != "203.0.113.5" {
		t.Errorf("ClientIP = %q, want %q", got, "203.0.113.5")
	}
}

// TestClientIP_FallsBackWhenNoPort covers a RemoteAddr with no port to
// strip (malformed, or set directly by a test/tool) - must return it as-is
// rather than erroring out.
func TestClientIP_FallsBackWhenNoPort(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "203.0.113.5"

	if got := ClientIP(r); got != "203.0.113.5" {
		t.Errorf("ClientIP = %q, want %q", got, "203.0.113.5")
	}
}

func okHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// TestRateLimiter_Limit_AllowsUpToMax covers requests within the allowed
// window: each of the first `max` requests from the same IP must pass
// through to the wrapped handler untouched.
func TestRateLimiter_Limit_AllowsUpToMax(t *testing.T) {
	rl := NewRateLimiter(3, time.Minute)

	for i := 0; i < 3; i++ {
		r := httptest.NewRequest(http.MethodPost, "/api/login", nil)
		r.RemoteAddr = "198.51.100.1:1111"
		w := httptest.NewRecorder()
		rl.Limit(okHandler)(w, r)

		if w.Code != http.StatusOK {
			t.Errorf("request %d: status = %d, want %d", i+1, w.Code, http.StatusOK)
		}
	}
}

// TestRateLimiter_Limit_BlocksOverMax covers the (max+1)th request from the
// same IP within the window - must be rejected with 429 and a JSON error
// body, and must not reach the wrapped handler.
func TestRateLimiter_Limit_BlocksOverMax(t *testing.T) {
	rl := NewRateLimiter(2, time.Minute)
	calls := 0
	handler := func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusOK)
	}

	for i := 0; i < 2; i++ {
		r := httptest.NewRequest(http.MethodPost, "/api/login", nil)
		r.RemoteAddr = "198.51.100.2:1111"
		rl.Limit(handler)(httptest.NewRecorder(), r)
	}

	r := httptest.NewRequest(http.MethodPost, "/api/login", nil)
	r.RemoteAddr = "198.51.100.2:1111"
	w := httptest.NewRecorder()
	rl.Limit(handler)(w, r)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want %d", w.Code, http.StatusTooManyRequests)
	}
	if calls != 2 {
		t.Errorf("wrapped handler called %d times, want 2 (3rd request should have been blocked before reaching it)", calls)
	}

	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to parse response JSON: %v (body: %s)", err, w.Body.String())
	}
	if body["error"] == "" {
		t.Error("expected a non-empty error message")
	}
}

// TestRateLimiter_Limit_TracksIPsIndependently covers two different client
// IPs sharing a limiter instance - one hitting the limit must not affect
// the other.
func TestRateLimiter_Limit_TracksIPsIndependently(t *testing.T) {
	rl := NewRateLimiter(1, time.Minute)

	r1 := httptest.NewRequest(http.MethodPost, "/api/login", nil)
	r1.RemoteAddr = "198.51.100.3:1111"
	rl.Limit(okHandler)(httptest.NewRecorder(), r1)

	// Same IP again - should now be blocked.
	r1again := httptest.NewRequest(http.MethodPost, "/api/login", nil)
	r1again.RemoteAddr = "198.51.100.3:1111"
	w1 := httptest.NewRecorder()
	rl.Limit(okHandler)(w1, r1again)
	if w1.Code != http.StatusTooManyRequests {
		t.Errorf("2nd request from same IP: status = %d, want %d", w1.Code, http.StatusTooManyRequests)
	}

	// Different IP, first request - should still pass.
	r2 := httptest.NewRequest(http.MethodPost, "/api/login", nil)
	r2.RemoteAddr = "198.51.100.4:1111"
	w2 := httptest.NewRecorder()
	rl.Limit(okHandler)(w2, r2)
	if w2.Code != http.StatusOK {
		t.Errorf("1st request from different IP: status = %d, want %d", w2.Code, http.StatusOK)
	}
}

// TestRateLimiter_RecordAndCheck covers the per-key counter used to detect
// distributed brute-force attempts (e.g. keyed by email rather than IP,
// see failedLoginTracker in auth.go) - must report false while under the
// threshold and true once it's reached, independently per key.
func TestRateLimiter_RecordAndCheck(t *testing.T) {
	rl := NewRateLimiter(3, time.Minute)

	for i := 0; i < 2; i++ {
		if got := rl.RecordAndCheck("victim@example.com"); got {
			t.Errorf("attempt %d: RecordAndCheck = true, want false (under threshold)", i+1)
		}
	}
	if got := rl.RecordAndCheck("victim@example.com"); !got {
		t.Error("3rd attempt: RecordAndCheck = false, want true (threshold reached)")
	}

	// A different key must be tracked independently, unaffected by the
	// first key already having reached its threshold.
	if got := rl.RecordAndCheck("someone-else@example.com"); got {
		t.Error("first attempt for a different key: RecordAndCheck = true, want false")
	}
}

// TestRateLimiter_LimitFailures_OnlyCountsFailures covers the login limiter:
// successful requests never count, so any number of them pass, while the
// (max+1)th failed request from the same IP is blocked with 429 before
// reaching the handler.
func TestRateLimiter_LimitFailures_OnlyCountsFailures(t *testing.T) {
	rl := NewRateLimiter(2, time.Minute)
	status := http.StatusOK
	calls := 0
	handler := func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(status)
	}
	send := func() int {
		r := httptest.NewRequest(http.MethodPost, "/api/login", nil)
		r.RemoteAddr = "198.51.100.5:1111"
		w := httptest.NewRecorder()
		rl.LimitFailures(http.StatusUnauthorized, handler)(w, r)
		return w.Code
	}

	for i := 0; i < 5; i++ {
		if got := send(); got != http.StatusOK {
			t.Fatalf("successful request %d: status = %d, want %d", i+1, got, http.StatusOK)
		}
	}

	status = http.StatusUnauthorized
	for i := 0; i < 2; i++ {
		if got := send(); got != http.StatusUnauthorized {
			t.Fatalf("failed request %d: status = %d, want %d", i+1, got, http.StatusUnauthorized)
		}
	}

	callsBefore := calls
	if got := send(); got != http.StatusTooManyRequests {
		t.Errorf("request after 2 failures: status = %d, want %d", got, http.StatusTooManyRequests)
	}
	if calls != callsBefore {
		t.Error("blocked request still reached the handler")
	}
}

// TestRateLimiter_BlockedRecordReset covers the per-key helpers behind the
// per-account login lockout: Blocked never records, Record counts towards
// max, and Reset clears the key.
func TestRateLimiter_BlockedRecordReset(t *testing.T) {
	rl := NewRateLimiter(2, time.Minute)
	key := "victim@example.com"

	for i := 0; i < 5; i++ {
		if rl.Blocked(key) {
			t.Fatal("Blocked = true before anything was recorded (Blocked must not record)")
		}
	}

	rl.Record(key)
	if rl.Blocked(key) {
		t.Error("Blocked = true after 1 of 2 attempts")
	}
	rl.Record(key)
	if !rl.Blocked(key) {
		t.Error("Blocked = false after reaching max")
	}
	if rl.Blocked("someone-else@example.com") {
		t.Error("a different key was blocked")
	}

	rl.Reset(key)
	if rl.Blocked(key) {
		t.Error("Blocked = true after Reset")
	}
}

// TestRateLimiter_Blocked_ExpiresWithWindow covers the lockout being
// temporary: once the recorded attempts are older than the window, the key
// is no longer blocked.
func TestRateLimiter_Blocked_ExpiresWithWindow(t *testing.T) {
	rl := NewRateLimiter(1, 50*time.Millisecond)
	rl.Record("k")
	if !rl.Blocked("k") {
		t.Fatal("Blocked = false right after reaching max")
	}
	time.Sleep(80 * time.Millisecond)
	if rl.Blocked("k") {
		t.Error("Blocked = true after the window passed")
	}
}
