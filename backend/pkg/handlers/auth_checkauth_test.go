package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"social-network/pkg/middleware"
)

// insertTestSession inserts a session row directly (bypassing Login), so
// tests can control fields Login wouldn't otherwise let them set directly -
// an already-expired expires_at, or a specific stored fingerprint.
func insertTestSession(t *testing.T, db *sql.DB, userID int64, token string, expiresAt time.Time, ip, fingerprint string) {
	t.Helper()
	_, err := db.Exec(
		"INSERT INTO sessions (user_id, session_token, expires_at, ip_address, fingerprint) VALUES (?, ?, ?, ?, ?)",
		userID, token, expiresAt, ip, fingerprint,
	)
	if err != nil {
		t.Fatalf("insertTestSession: %v", err)
	}
}

// sessionExists reports whether a session row for token is still present.
func sessionExists(t *testing.T, db *sql.DB, token string) bool {
	t.Helper()
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM sessions WHERE session_token = ?", token).Scan(&count); err != nil {
		t.Fatalf("sessionExists: %v", err)
	}
	return count > 0
}

func checkAuthRequest(cookie *http.Cookie, fingerprint string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/api/check-auth", nil)
	if cookie != nil {
		r.AddCookie(cookie)
	}
	if fingerprint != "" {
		r.Header.Set(middleware.DeviceFingerprintHeader, fingerprint)
	}
	return r
}

func decodeCheckAuth(t *testing.T, w *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("CheckAuth: failed to parse response JSON: %v (body: %s)", err, w.Body.String())
	}
	return resp
}

// TestCheckAuthNoCookie covers the "logged out" case: no session cookie at
// all must report authenticated:false with a 200, not an error status -
// this endpoint is polled on every page load, so "not logged in" is a
// normal, expected outcome rather than a failure.
func TestCheckAuthNoCookie(t *testing.T) {
	db := newTestDB(t)
	auth := &AuthHandler{DB: db}

	w := httptest.NewRecorder()
	auth.CheckAuth(w, checkAuthRequest(nil, ""))

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	resp := decodeCheckAuth(t, w)
	if resp["authenticated"] != false {
		t.Errorf("authenticated = %v, want false", resp["authenticated"])
	}
}

// TestCheckAuthInvalidToken covers a cookie whose value matches no session
// row at all (garbage token, or one from a database that's since been
// wiped) - must fail closed, not error out.
func TestCheckAuthInvalidToken(t *testing.T) {
	db := newTestDB(t)
	auth := &AuthHandler{DB: db}

	cookie := &http.Cookie{Name: "session_token", Value: "does-not-exist"}
	w := httptest.NewRecorder()
	auth.CheckAuth(w, checkAuthRequest(cookie, ""))

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	resp := decodeCheckAuth(t, w)
	if resp["authenticated"] != false {
		t.Errorf("authenticated = %v, want false", resp["authenticated"])
	}
}

// TestCheckAuthExpiredSession covers a session row that exists but is past
// expires_at - must report logged-out and clean up the stale row rather
// than leaving it to accumulate.
func TestCheckAuthExpiredSession(t *testing.T) {
	db := newTestDB(t)
	auth := &AuthHandler{DB: db}
	userID := registerTestUser(t, db, "expired-checkauth@example.com")

	token := "expired-token"
	insertTestSession(t, db, userID, token, time.Now().Add(-time.Hour), "1.2.3.4", "")

	cookie := &http.Cookie{Name: "session_token", Value: token}
	w := httptest.NewRecorder()
	auth.CheckAuth(w, checkAuthRequest(cookie, ""))

	resp := decodeCheckAuth(t, w)
	if resp["authenticated"] != false {
		t.Errorf("authenticated = %v, want false", resp["authenticated"])
	}
	if sessionExists(t, db, token) {
		t.Error("expired session row should have been deleted, but still exists")
	}
}

// TestCheckAuthFingerprintMismatchInvalidatesSession covers a session used
// with a device fingerprint different from the one it was created with -
// treated as a hijacked/shared cookie, invalidated immediately rather than
// merely logged.
func TestCheckAuthFingerprintMismatchInvalidatesSession(t *testing.T) {
	db := newTestDB(t)
	auth := &AuthHandler{DB: db}
	userID := registerTestUser(t, db, "fp-mismatch@example.com")

	token := "fp-token"
	insertTestSession(t, db, userID, token, time.Now().Add(time.Hour), "1.2.3.4", "device-A")

	cookie := &http.Cookie{Name: "session_token", Value: token}
	w := httptest.NewRecorder()
	auth.CheckAuth(w, checkAuthRequest(cookie, "device-B"))

	resp := decodeCheckAuth(t, w)
	if resp["authenticated"] != false {
		t.Errorf("authenticated = %v, want false", resp["authenticated"])
	}
	if sessionExists(t, db, token) {
		t.Error("session row should have been deleted on fingerprint mismatch, but still exists")
	}
}

// TestCheckAuthFingerprintHeaderOmittedStillAuthenticates covers the
// deliberate asymmetry in the fingerprint check: it only runs when the
// caller actually sends the header. A session created with a fingerprint
// must still authenticate a request that simply doesn't participate in
// fingerprinting, rather than being penalized for the header's absence.
func TestCheckAuthFingerprintHeaderOmittedStillAuthenticates(t *testing.T) {
	db := newTestDB(t)
	auth := &AuthHandler{DB: db}
	userID := registerTestUser(t, db, "fp-omitted@example.com")

	token := "fp-omitted-token"
	insertTestSession(t, db, userID, token, time.Now().Add(time.Hour), "1.2.3.4", "device-A")

	cookie := &http.Cookie{Name: "session_token", Value: token}
	w := httptest.NewRecorder()
	auth.CheckAuth(w, checkAuthRequest(cookie, ""))

	resp := decodeCheckAuth(t, w)
	if resp["authenticated"] != true {
		t.Errorf("authenticated = %v, want true", resp["authenticated"])
	}
	if !sessionExists(t, db, token) {
		t.Error("session row should still exist - the header was never sent, so the check shouldn't have run")
	}
}

// TestCheckAuthBindsFingerprintToUnboundSession covers sessions created
// without a fingerprint (GitHub login): the first check-auth that sends one
// binds it, and from then on a different fingerprint invalidates the
// session exactly as for a password-login session.
func TestCheckAuthBindsFingerprintToUnboundSession(t *testing.T) {
	db := newTestDB(t)
	auth := &AuthHandler{DB: db}
	userID := registerTestUser(t, db, "fp-bind@example.com")

	token := "fp-bind-token"
	insertTestSession(t, db, userID, token, time.Now().Add(time.Hour), "1.2.3.4", "")
	cookie := &http.Cookie{Name: "session_token", Value: token}

	w := httptest.NewRecorder()
	auth.CheckAuth(w, checkAuthRequest(cookie, "device-A"))
	if resp := decodeCheckAuth(t, w); resp["authenticated"] != true {
		t.Fatalf("first check: authenticated = %v, want true", resp["authenticated"])
	}
	var stored string
	if err := db.QueryRow("SELECT fingerprint FROM sessions WHERE session_token = ?", token).Scan(&stored); err != nil {
		t.Fatalf("reading stored fingerprint: %v", err)
	}
	if stored != "device-A" {
		t.Fatalf("stored fingerprint = %q, want %q", stored, "device-A")
	}

	w = httptest.NewRecorder()
	auth.CheckAuth(w, checkAuthRequest(cookie, "device-B"))
	if resp := decodeCheckAuth(t, w); resp["authenticated"] != false {
		t.Errorf("second check from another device: authenticated = %v, want false", resp["authenticated"])
	}
	if sessionExists(t, db, token) {
		t.Error("session row should have been deleted on fingerprint mismatch after binding, but still exists")
	}
}

// TestCheckAuthValidSession covers the ordinary happy path: a live session
// with a matching fingerprint must authenticate and echo back the user's
// profile fields.
func TestCheckAuthValidSession(t *testing.T) {
	db := newTestDB(t)
	auth := &AuthHandler{DB: db}
	userID := registerTestUser(t, db, "valid-checkauth@example.com")

	token := "valid-token"
	insertTestSession(t, db, userID, token, time.Now().Add(time.Hour), "1.2.3.4", "device-A")

	cookie := &http.Cookie{Name: "session_token", Value: token}
	w := httptest.NewRecorder()
	auth.CheckAuth(w, checkAuthRequest(cookie, "device-A"))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	resp := decodeCheckAuth(t, w)
	if resp["authenticated"] != true {
		t.Errorf("authenticated = %v, want true", resp["authenticated"])
	}
	if resp["email"] != "valid-checkauth@example.com" {
		t.Errorf("email = %v, want valid-checkauth@example.com", resp["email"])
	}
	if int64(resp["id"].(float64)) != userID {
		t.Errorf("id = %v, want %d", resp["id"], userID)
	}
}
