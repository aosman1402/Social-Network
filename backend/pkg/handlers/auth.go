package handlers

import (
    "database/sql"
    "encoding/json"
    "net/http"
    "time"

    "social-network3/pkg/middleware"

    "github.com/google/uuid"
    "golang.org/x/crypto/bcrypt"
)

type AuthHandler struct {
    DB *sql.DB
}

type RegisterRequest struct {
    Email       string  `json:"email"`
    Password    string  `json:"password"`
    FirstName   string  `json:"first_name"`
    LastName    string  `json:"last_name"`
    DateOfBirth string  `json:"date_of_birth"`
    AvatarPath  *string `json:"avatar_path"`
    Nickname    *string `json:"nickname"`
    AboutMe     *string `json:"about_me"`
}

type LoginRequest struct {
    Email    string `json:"email"`
    Password string `json:"password"`
}

// dummyPasswordHash is a valid bcrypt hash of a fixed, unused password.
// Login compares against it when the given email doesn't exist, so a
// nonexistent-email attempt costs the same bcrypt time as a wrong-password
// one - see the timing note in Login.
var dummyPasswordHash []byte

func init() {
    dummyPasswordHash, _ = bcrypt.GenerateFromPassword(
        []byte("this-is-never-a-real-users-password"), bcrypt.DefaultCost,
    )
}

func (ah *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
    // Always set JSON content type
    w.Header().Set("Content-Type", "application/json")

    // Only accept POST requests
    if r.Method != "POST" {
        w.WriteHeader(http.StatusMethodNotAllowed)
        json.NewEncoder(w).Encode(map[string]string{
            "error": "Method not allowed",
        })
        return
    }

    var req RegisterRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        w.WriteHeader(http.StatusBadRequest)
        json.NewEncoder(w).Encode(map[string]string{
            "error": "Invalid request body: " + err.Error(),
        })
        return
    }

    // Validate required fields
    if req.Email == "" || req.Password == "" || req.FirstName == "" ||
        req.LastName == "" || req.DateOfBirth == "" {
        w.WriteHeader(http.StatusBadRequest)
        json.NewEncoder(w).Encode(map[string]string{
            "message": "Missing fields",
            "success": "false",
        })
        return
    }

    // Enforce the same minimum the frontend advertises - client-side
    // validation alone doesn't stop a direct API call with a weak password.
    if len(req.Password) < 8 {
        w.WriteHeader(http.StatusBadRequest)
        json.NewEncoder(w).Encode(map[string]string{
            "error": "Password must be at least 8 characters",
        })
        return
    }

    // Hash password
    hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
    if err != nil {
        w.WriteHeader(http.StatusInternalServerError)
        json.NewEncoder(w).Encode(map[string]string{
            "error": "Failed to hash password",
        })
        return
    }

    // Parse date of birth
    dob, err := time.Parse("2006-01-02", req.DateOfBirth)
    if err != nil {
        w.WriteHeader(http.StatusBadRequest)
        json.NewEncoder(w).Encode(map[string]string{
            "error": "Invalid date format. Use YYYY-MM-DD",
        })
        return
    }

    // Insert user
    result, err := ah.DB.Exec(
        `INSERT INTO users (email, password_hash, first_name, last_name, date_of_birth, avatar_path, nickname, about_me)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
        req.Email, hashedPassword, req.FirstName, req.LastName, dob,
        req.AvatarPath, req.Nickname, req.AboutMe,
    )
    if err != nil {
        w.WriteHeader(http.StatusInternalServerError)
        json.NewEncoder(w).Encode(map[string]string{
            "error": "Failed to create user: " + err.Error(),
        })
        return
    }

    userID, _ := result.LastInsertId()
    w.WriteHeader(http.StatusCreated)
    json.NewEncoder(w).Encode(map[string]interface{}{
        "message": "User registered successfully",
        "user_id": userID,
    })
}

func (ah *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")

    if r.Method != "POST" {
        w.WriteHeader(http.StatusMethodNotAllowed)
        json.NewEncoder(w).Encode(map[string]string{
            "error": "Method not allowed",
        })
        return
    }

    var req LoginRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        w.WriteHeader(http.StatusBadRequest)
        json.NewEncoder(w).Encode(map[string]string{
            "error": "Invalid request body",
        })
        return
    }

    var userID int64
    var passwordHash string
    err := ah.DB.QueryRow(
        "SELECT id, password_hash FROM users WHERE email = ?",
        req.Email,
    ).Scan(&userID, &passwordHash)
    userFound := err == nil

    // Always run a bcrypt compare, even for an email that doesn't exist, so
    // the response takes the same time either way - otherwise a fast reply
    // (no user, no bcrypt call) vs. a slow one (user found, bcrypt runs)
    // lets an attacker enumerate registered emails purely from timing.
    hashToCheck := passwordHash
    if !userFound {
        hashToCheck = string(dummyPasswordHash)
    }
    passwordMatches := bcrypt.CompareHashAndPassword([]byte(hashToCheck), []byte(req.Password)) == nil

    if !userFound || !passwordMatches {
        w.WriteHeader(http.StatusUnauthorized)
        json.NewEncoder(w).Encode(map[string]string{
            "error": "Invalid credentials",
        })
        return
    }

    sessionToken := uuid.New().String()
    expiresAt := time.Now().Add(24 * time.Hour)

    _, err = ah.DB.Exec(
        "INSERT INTO sessions (user_id, session_token, expires_at) VALUES (?, ?, ?)",
        userID, sessionToken, expiresAt,
    )
    if err != nil {
        w.WriteHeader(http.StatusInternalServerError)
        json.NewEncoder(w).Encode(map[string]string{
            "error": "Failed to create session",
        })
        return
    }

    // Set cookie. The token lives only here (HttpOnly, never in the response
    // body) - Secure is derived from the request itself so it's on whenever
    // the connection is actually HTTPS, without needing separate prod/dev
    // config.
    http.SetCookie(w, &http.Cookie{
        Name:     "session_token",
        Value:    sessionToken,
        Expires:  expiresAt,
        HttpOnly: true,
        Secure:   r.TLS != nil,
        Path:     "/",
        SameSite: http.SameSiteLaxMode,
    })

    json.NewEncoder(w).Encode(map[string]interface{}{
        "message": "Login successful",
        "user_id": userID,
    })
}

func (ah *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")

    cookie, err := r.Cookie("session_token")
    if err == nil {
        ah.DB.Exec("DELETE FROM sessions WHERE session_token = ?", cookie.Value)
    }

    http.SetCookie(w, &http.Cookie{
        Name:     "session_token",
        Value:    "",
        Expires:  time.Now().Add(-1 * time.Hour),
        HttpOnly: true,
        Secure:   r.TLS != nil,
        Path:     "/",
        SameSite: http.SameSiteLaxMode,
    })

    json.NewEncoder(w).Encode(map[string]string{
        "message": "Logout successful",
    })
}

// CheckAuth reports whether the request carries a valid session. It always
// responds 200 - "not logged in" is a normal outcome for this endpoint, not
// an error - and callers should branch on the "authenticated" field rather
// than the HTTP status.
func (ah *AuthHandler) CheckAuth(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")

    cookie, err := r.Cookie("session_token")
    if err != nil {
        json.NewEncoder(w).Encode(map[string]interface{}{"authenticated": false})
        return
    }

    var userID int64
    var expiresAt time.Time
    err = ah.DB.QueryRow(
        "SELECT user_id, expires_at FROM sessions WHERE session_token = ?",
        cookie.Value,
    ).Scan(&userID, &expiresAt)

    if err != nil {
        json.NewEncoder(w).Encode(map[string]interface{}{"authenticated": false})
        return
    }

    if time.Now().After(expiresAt) {
        ah.DB.Exec("DELETE FROM sessions WHERE session_token = ?", cookie.Value)
        json.NewEncoder(w).Encode(map[string]interface{}{"authenticated": false})
        return
    }

    // ✅ FIX: Include created_at in the query
    var user struct {
        Authenticated bool      `json:"authenticated"`
        ID            int64     `json:"id"`
        Email         string    `json:"email"`
        FirstName     string    `json:"first_name"`
        LastName      string    `json:"last_name"`
        Nickname      *string   `json:"nickname"`
        Avatar        *string   `json:"avatar_path"`
        IsPublic      bool      `json:"is_public"`
        CreatedAt     time.Time `json:"created_at"` // ✅ ADD THIS
    }
    user.Authenticated = true

    err = ah.DB.QueryRow(
        `SELECT id, email, first_name, last_name, nickname, avatar_path, is_public, created_at
         FROM users WHERE id = ?`,
        userID,
    ).Scan(
        &user.ID,
        &user.Email,
        &user.FirstName,
        &user.LastName,
        &user.Nickname,
        &user.Avatar,
        &user.IsPublic,
        &user.CreatedAt, // ✅ ADD THIS
    )

    if err != nil {
        w.WriteHeader(http.StatusInternalServerError)
        json.NewEncoder(w).Encode(map[string]string{"error": "User not found"})
        return
    }

    json.NewEncoder(w).Encode(user)
}

type ChangePasswordRequest struct {
    CurrentPassword string `json:"current_password"`
    NewPassword     string `json:"new_password"`
}

// ChangePassword lets an authenticated user rotate their own password. It
// requires the current password (so a hijacked session cookie alone isn't
// enough to lock the real owner out), and invalidates every session on the
// account - including the one making this request - once the new password
// is set, so a stolen token stops working the moment the real owner changes
// their password. The caller has to log in again afterwards.
func (ah *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")

    if r.Method != "POST" && r.Method != "PUT" {
        w.WriteHeader(http.StatusMethodNotAllowed)
        json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
        return
    }

    userID, ok := middleware.GetUserID(r)
    if !ok {
        w.WriteHeader(http.StatusUnauthorized)
        json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
        return
    }

    var req ChangePasswordRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        w.WriteHeader(http.StatusBadRequest)
        json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
        return
    }

    if len(req.NewPassword) < 8 {
        w.WriteHeader(http.StatusBadRequest)
        json.NewEncoder(w).Encode(map[string]string{"error": "New password must be at least 8 characters"})
        return
    }

    var currentHash string
    if err := ah.DB.QueryRow("SELECT password_hash FROM users WHERE id = ?", userID).Scan(&currentHash); err != nil {
        w.WriteHeader(http.StatusInternalServerError)
        json.NewEncoder(w).Encode(map[string]string{"error": "Failed to load user"})
        return
    }

    if err := bcrypt.CompareHashAndPassword([]byte(currentHash), []byte(req.CurrentPassword)); err != nil {
        w.WriteHeader(http.StatusUnauthorized)
        json.NewEncoder(w).Encode(map[string]string{"error": "Current password is incorrect"})
        return
    }

    newHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
    if err != nil {
        w.WriteHeader(http.StatusInternalServerError)
        json.NewEncoder(w).Encode(map[string]string{"error": "Failed to hash password"})
        return
    }

    if _, err := ah.DB.Exec(
        "UPDATE users SET password_hash = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
        newHash, userID,
    ); err != nil {
        w.WriteHeader(http.StatusInternalServerError)
        json.NewEncoder(w).Encode(map[string]string{"error": "Failed to update password"})
        return
    }

    ah.DB.Exec("DELETE FROM sessions WHERE user_id = ?", userID)

    // This request's own session is gone too - clear its cookie.
    http.SetCookie(w, &http.Cookie{
        Name:     "session_token",
        Value:    "",
        Expires:  time.Now().Add(-1 * time.Hour),
        HttpOnly: true,
        Secure:   r.TLS != nil,
        Path:     "/",
        SameSite: http.SameSiteLaxMode,
    })

    json.NewEncoder(w).Encode(map[string]string{
        "message": "Password changed successfully. Please log in again.",
    })
}