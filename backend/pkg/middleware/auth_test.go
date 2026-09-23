package middleware

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"social-network/pkg/db/sqlite"
)

// newTestDB spins up a fresh, throwaway SQLite database with every
// migration applied. Mirrors pkg/handlers/testutil_test.go's helper of the
// same name - duplicated rather than shared, since the two packages have no
// existing common test-support package to hang a shared version off of.
func newTestDB(t *testing.T) *sql.DB {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "test.db")
	migrationsPath, err := filepath.Abs("../db/migrations/sqlite")
	if err != nil {
		t.Fatalf("failed to resolve migrations path: %v", err)
	}

	db, err := sqlite.NewDB(sqlite.Config{
		DBPath:            dbPath,
		MigrationsPath:    migrationsPath,
		EnableForeignKeys: true,
	})
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := db.ApplyMigrations(migrationsPath); err != nil {
		t.Fatalf("failed to apply migrations: %v", err)
	}

	return db.DB
}

// createTestUser inserts a minimal user row directly and returns its id -
// Authenticate only ever looks the id up by session, never touches most
// user fields, so a full Register round trip isn't needed here.
func createTestUser(t *testing.T, db *sql.DB, email string) int64 {
	t.Helper()
	res, err := db.Exec(
		`INSERT INTO users (email, password_hash, first_name, last_name, date_of_birth)
         VALUES (?, 'hash', 'Test', 'User', '1990-01-01')`,
		email,
	)
	if err != nil {
		t.Fatalf("createTestUser: %v", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("createTestUser: %v", err)
	}
	return id
}

func createTestSession(t *testing.T, db *sql.DB, userID int64, token string, expiresAt time.Time, ip, fingerprint string) {
	t.Helper()
	_, err := db.Exec(
		"INSERT INTO sessions (user_id, session_token, expires_at, ip_address, fingerprint) VALUES (?, ?, ?, ?, ?)",
		userID, token, expiresAt, ip, fingerprint,
	)
	if err != nil {
		t.Fatalf("createTestSession: %v", err)
	}
}

func sessionCount(t *testing.T, db *sql.DB, token string) int {
	t.Helper()
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM sessions WHERE session_token = ?", token).Scan(&count); err != nil {
		t.Fatalf("sessionCount: %v", err)
	}
	return count
}

// echoUserID is a stub "next" handler that reports the userID Authenticate
// attached to the request context, so tests can assert on what actually
// reached the wrapped handler rather than just the middleware's own
// pass/fail status code.
func echoUserID(w http.ResponseWriter, r *http.Request) {
	userID, ok := GetUserID(r)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{"user_id": userID, "ok": ok})
}

func decodeUnauthorized(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to parse response JSON: %v (body: %s)", err, w.Body.String())
	}
	return body["error"]
}

// TestAuthenticate_NoCookie covers a request with no session cookie at all -
// must be rejected before the wrapped handler ever runs.
func TestAuthenticate_NoCookie(t *testing.T) {
	db := newTestDB(t)
	am := &AuthMiddleware{DB: db}

	r := httptest.NewRequest(http.MethodGet, "/api/whatever", nil)
	w := httptest.NewRecorder()
	am.Authenticate(echoUserID)(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
	if msg := decodeUnauthorized(t, w); msg == "" {
		t.Error("expected a non-empty error message")
	}
}

// TestAuthenticate_InvalidToken covers a cookie whose value matches no
// session row.
func TestAuthenticate_InvalidToken(t *testing.T) {
	db := newTestDB(t)
	am := &AuthMiddleware{DB: db}

	r := httptest.NewRequest(http.MethodGet, "/api/whatever", nil)
	r.AddCookie(&http.Cookie{Name: "session_token", Value: "bogus"})
	w := httptest.NewRecorder()
	am.Authenticate(echoUserID)(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

// TestAuthenticate_ExpiredSession covers a session row that's past
// expires_at - must be rejected and the stale row deleted.
func TestAuthenticate_ExpiredSession(t *testing.T) {
	db := newTestDB(t)
	am := &AuthMiddleware{DB: db}
	userID := createTestUser(t, db, "expired-mw@example.com")

	token := "expired-mw-token"
	createTestSession(t, db, userID, token, time.Now().Add(-time.Hour), "1.2.3.4", "")

	r := httptest.NewRequest(http.MethodGet, "/api/whatever", nil)
	r.AddCookie(&http.Cookie{Name: "session_token", Value: token})
	w := httptest.NewRecorder()
	am.Authenticate(echoUserID)(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
	if got := sessionCount(t, db, token); got != 0 {
		t.Errorf("expired session row still present: count = %d, want 0", got)
	}
}

// TestAuthenticate_FingerprintMismatch covers a session used with a device
// fingerprint different from the one it was created with - invalidated
// immediately, not merely logged.
func TestAuthenticate_FingerprintMismatch(t *testing.T) {
	db := newTestDB(t)
	am := &AuthMiddleware{DB: db}
	userID := createTestUser(t, db, "fp-mismatch-mw@example.com")

	token := "fp-mismatch-mw-token"
	createTestSession(t, db, userID, token, time.Now().Add(time.Hour), "1.2.3.4", "device-A")

	r := httptest.NewRequest(http.MethodGet, "/api/whatever", nil)
	r.AddCookie(&http.Cookie{Name: "session_token", Value: token})
	r.Header.Set(DeviceFingerprintHeader, "device-B")
	w := httptest.NewRecorder()
	am.Authenticate(echoUserID)(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
	if got := sessionCount(t, db, token); got != 0 {
		t.Errorf("session row should have been deleted on fingerprint mismatch: count = %d, want 0", got)
	}
}

// TestAuthenticate_ValidSession covers the happy path: a live, matching
// session must reach the wrapped handler with the correct userID attached
// to the request context.
func TestAuthenticate_ValidSession(t *testing.T) {
	db := newTestDB(t)
	am := &AuthMiddleware{DB: db}
	userID := createTestUser(t, db, "valid-mw@example.com")

	token := "valid-mw-token"
	createTestSession(t, db, userID, token, time.Now().Add(time.Hour), "1.2.3.4", "device-A")

	r := httptest.NewRequest(http.MethodGet, "/api/whatever", nil)
	r.AddCookie(&http.Cookie{Name: "session_token", Value: token})
	r.Header.Set(DeviceFingerprintHeader, "device-A")
	w := httptest.NewRecorder()
	am.Authenticate(echoUserID)(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var resp struct {
		UserID int64 `json:"user_id"`
		OK     bool  `json:"ok"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response JSON: %v", err)
	}
	if !resp.OK {
		t.Error("GetUserID reported ok = false for an authenticated request")
	}
	if resp.UserID != userID {
		t.Errorf("user_id = %d, want %d", resp.UserID, userID)
	}
}

// TestGetUserID_NoValueInContext covers calling GetUserID on a request that
// never went through Authenticate - must report ok = false, not panic or
// return a zero value indistinguishable from a real user id of 0.
func TestGetUserID_NoValueInContext(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/whatever", nil)
	userID, ok := GetUserID(r)
	if ok {
		t.Error("ok = true for a request with no userID in context")
	}
	if userID != 0 {
		t.Errorf("userID = %d, want 0", userID)
	}
}

// TestGetUserID_ValueInContext covers the plumbing Authenticate relies on
// directly, independent of the HTTP layer.
func TestGetUserID_ValueInContext(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/whatever", nil)
	ctx := context.WithValue(r.Context(), UserIDKey, int64(42))
	r = r.WithContext(ctx)

	userID, ok := GetUserID(r)
	if !ok {
		t.Error("ok = false for a request with userID set in context")
	}
	if userID != 42 {
		t.Errorf("userID = %d, want 42", userID)
	}
}
