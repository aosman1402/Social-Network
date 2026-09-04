package handlers

import (
    "database/sql"
    "encoding/json"
    "net/http"
    "time"

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
    json.NewEncoder(w).Encode(map[string]string{
        "message": "Missing fields",
        "success": "false",
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

    if err != nil {
        w.WriteHeader(http.StatusUnauthorized)
        json.NewEncoder(w).Encode(map[string]string{
            "error": "Invalid credentials",
        })
        return
    }

    if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
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

    // Set cookie
    http.SetCookie(w, &http.Cookie{
        Name:     "session_token",
        Value:    sessionToken,
        Expires:  expiresAt,
        HttpOnly: true,
        Path:     "/",
        SameSite: http.SameSiteLaxMode,
    })

    json.NewEncoder(w).Encode(map[string]interface{}{
        "message": "Login successful",
        "user_id": userID,
        "session_token": sessionToken,
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
        Path:     "/",
    })

    json.NewEncoder(w).Encode(map[string]string{
        "message": "Logout successful",
    })
}

func (ah *AuthHandler) CheckAuth(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")

    cookie, err := r.Cookie("session_token")
    if err != nil {
        w.WriteHeader(http.StatusUnauthorized)
        json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized - No token"})
        return
    }

    var userID int64
    var expiresAt time.Time
    err = ah.DB.QueryRow(
        "SELECT user_id, expires_at FROM sessions WHERE session_token = ?",
        cookie.Value,
    ).Scan(&userID, &expiresAt)

    if err != nil {
        w.WriteHeader(http.StatusUnauthorized)
        json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized - Invalid token"})
        return
    }

    if time.Now().After(expiresAt) {
        ah.DB.Exec("DELETE FROM sessions WHERE session_token = ?", cookie.Value)
        w.WriteHeader(http.StatusUnauthorized)
        json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized - Token expired"})
        return
    }

    // ✅ FIX: Include created_at in the query
    var user struct {
        ID        int64     `json:"id"`
        Email     string    `json:"email"`
        FirstName string    `json:"first_name"`
        LastName  string    `json:"last_name"`
        Nickname  *string   `json:"nickname"`
        Avatar    *string   `json:"avatar_path"`
        IsPublic  bool      `json:"is_public"`
        CreatedAt time.Time `json:"created_at"` // ✅ ADD THIS
    }

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