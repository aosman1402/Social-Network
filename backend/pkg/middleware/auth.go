package middleware

import (
    "context"
    "database/sql"
    "net/http"
    "time"
)

type contextKey string

const UserIDKey contextKey = "userID"

type AuthMiddleware struct {
    DB *sql.DB
}

func (am *AuthMiddleware) Authenticate(next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        cookie, err := r.Cookie("session_token")
        if err != nil {
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }

        var userID int64
        var expiresAt time.Time
        err = am.DB.QueryRow(
            "SELECT user_id, expires_at FROM sessions WHERE session_token = ?",
            cookie.Value,
        ).Scan(&userID, &expiresAt)

        if err != nil {
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }

        if time.Now().After(expiresAt) {
            // Delete expired session
            am.DB.Exec("DELETE FROM sessions WHERE session_token = ?", cookie.Value)
            http.Error(w, "Session expired", http.StatusUnauthorized)
            return
        }

        ctx := context.WithValue(r.Context(), UserIDKey, userID)
        next(w, r.WithContext(ctx))
    }
}

func GetUserID(r *http.Request) (int64, bool) {
    userID, ok := r.Context().Value(UserIDKey).(int64)
    return userID, ok
}