package middleware

import (
    "context"
    "database/sql"
    "encoding/json"
    "log"
    "net/http"
    "time"
)

type contextKey string

const UserIDKey contextKey = "userID"

// DeviceFingerprintHeader carries a client-computed device fingerprint
// (e.g. from FingerprintJS). Only requests that actually send it participate
// in the mismatch check below - most existing API calls don't send it, and
// are simply not checked, rather than being penalized for its absence.
const DeviceFingerprintHeader = "X-Device-Fingerprint"

// writeUnauthorized writes a JSON error body, not the stdlib http.Error's
// plain text - the frontend's apiFetch always calls response.json() on
// every response, and a plain-text body fails that parse, silently
// becoming `data: null` and crashing any caller that does
// `data.error || "fallback"` on it. This middleware wraps nearly every
// authenticated endpoint, so it's the most-hit instance of that bug.
func writeUnauthorized(w http.ResponseWriter, message string) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusUnauthorized)
    json.NewEncoder(w).Encode(map[string]string{"error": message})
}

type AuthMiddleware struct {
    DB *sql.DB
}

func (am *AuthMiddleware) Authenticate(next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        cookie, err := r.Cookie("session_token")
        if err != nil {
            writeUnauthorized(w, "Unauthorized")
            return
        }

        var userID int64
        var expiresAt time.Time
        var storedIP, storedFingerprint sql.NullString
        err = am.DB.QueryRow(
            "SELECT user_id, expires_at, ip_address, fingerprint FROM sessions WHERE session_token = ?",
            cookie.Value,
        ).Scan(&userID, &expiresAt, &storedIP, &storedFingerprint)

        if err != nil {
            writeUnauthorized(w, "Unauthorized")
            return
        }

        if time.Now().After(expiresAt) {
            // Delete expired session
            am.DB.Exec("DELETE FROM sessions WHERE session_token = ?", cookie.Value)
            writeUnauthorized(w, "Session expired")
            return
        }

        // A fingerprint mismatch is a strong signal the cookie is being used
        // from a different device than the one that logged in - invalidate
        // immediately.
        if fp := r.Header.Get(DeviceFingerprintHeader); fp != "" &&
            storedFingerprint.Valid && storedFingerprint.String != "" &&
            fp != storedFingerprint.String {
            am.DB.Exec("DELETE FROM sessions WHERE session_token = ?", cookie.Value)
            writeUnauthorized(w, "Session invalidated")
            return
        }

        // IP changes are common and often legitimate (mobile networks,
        // Wi-Fi switching) - logged for visibility rather than invalidated.
        if ip := ClientIP(r); storedIP.Valid && storedIP.String != "" && ip != storedIP.String {
            log.Printf("[security] session for user %d: IP changed from %s to %s", userID, storedIP.String, ip)
        }

        ctx := context.WithValue(r.Context(), UserIDKey, userID)
        next(w, r.WithContext(ctx))
    }
}

func GetUserID(r *http.Request) (int64, bool) {
    userID, ok := r.Context().Value(UserIDKey).(int64)
    return userID, ok
}