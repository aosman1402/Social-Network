package handlers

import (
    "database/sql"
    "encoding/json"
    "log"
    "net/http"
    "strings"
    "time"

    "social-network/pkg/middleware"
    "social-network/pkg/websocket"

    "github.com/google/uuid"
    "golang.org/x/crypto/bcrypt"
)

// sessionDuration is how long a session (and the cookie carrying it) stays
// valid after login, absent any earlier invalidation (logout, password
// change, or a fingerprint mismatch).
const sessionDuration = 30 * 24 * time.Hour

// failedLoginTracker records failed login attempts per email - separately
// from the per-IP RateLimiter wrapping this handler - so a distributed
// brute-force attempt against one account (spread across many IPs, each
// individually under the per-IP threshold) still gets a response: any
// session that account currently has is invalidated as a precaution.
var failedLoginTracker = middleware.NewRateLimiter(8, 5*time.Minute)

// accountLockout blocks further login attempts for an email after 5 failed
// ones within 15 minutes, whichever IPs they came from - the per-IP limit in
// server.go alone can be dodged by an attacker rotating IPs. The block is
// temporary (it lifts as the failures age out of the window) so it can't be
// used to lock the real owner out for good, and a successful login clears
// the count. Failures are recorded for unknown emails too, so being blocked
// doesn't reveal whether an email is registered.
var accountLockout = middleware.NewRateLimiter(5, 15*time.Minute)

type AuthHandler struct {
    DB *sql.DB
    // Hub, when set, lets logout / password change close the live
    // connections of the sessions they end. May be nil (e.g. in tests).
    Hub *websocket.Hub
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
    req.Email = normalizeEmail(req.Email)

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

    if !isValidEmail(req.Email) {
        w.WriteHeader(http.StatusBadRequest)
        json.NewEncoder(w).Encode(map[string]string{
            "message": "Invalid email address",
            "success": "false",
        })
        return
    }

    taken, err := ah.emailTaken(req.Email)
    if err != nil {
        w.WriteHeader(http.StatusInternalServerError)
        json.NewEncoder(w).Encode(map[string]string{
            "error": "Failed to validate email",
        })
        return
    }
    if taken {
        w.WriteHeader(http.StatusConflict)
        json.NewEncoder(w).Encode(map[string]string{
            "error": "Email already in use",
        })
        return
    }

    // Enforce the same policy the frontend advertises and checks live -
    // client-side validation alone doesn't stop a direct API call with a
    // weak password.
    if !isValidPassword(req.Password) {
        w.WriteHeader(http.StatusBadRequest)
        json.NewEncoder(w).Encode(map[string]string{
            "error": "Password must be 12-15 characters and include a lowercase letter, an uppercase letter, a number, and a special character",
        })
        return
    }

    // Nickname is optional (the field can be left blank), but if one is
    // given it must be well-formed and not already claimed by another user -
    // checked here too, mirroring the frontend's live availability check,
    // since a direct API call could otherwise skip both.
    if req.Nickname != nil && *req.Nickname != "" {
        if !isValidNicknameFormat(*req.Nickname) {
            w.WriteHeader(http.StatusBadRequest)
            json.NewEncoder(w).Encode(map[string]string{
                "error": "Nickname must be 3-14 characters using only letters and numbers",
            })
            return
        }

        taken, err := ah.nicknameTaken(*req.Nickname)
        if err != nil {
            w.WriteHeader(http.StatusInternalServerError)
            json.NewEncoder(w).Encode(map[string]string{
                "error": "Failed to validate nickname",
            })
            return
        }
        if taken {
            w.WriteHeader(http.StatusConflict)
            json.NewEncoder(w).Encode(map[string]string{
                "error": "Nickname is already taken",
            })
            return
        }
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
        // The pre-checks above cover the common case; this only fires on a
        // race (two requests for the same email/nickname landing at once).
        // Map that to the same clean messages rather than leaking the raw
        // driver error to the client.
        msg := err.Error()
        switch {
        case strings.Contains(msg, "users.email"):
            w.WriteHeader(http.StatusConflict)
            json.NewEncoder(w).Encode(map[string]string{
                "error": "Email already in use",
            })
        case strings.Contains(msg, "users.nickname"):
            w.WriteHeader(http.StatusConflict)
            json.NewEncoder(w).Encode(map[string]string{
                "error": "Nickname is already taken",
            })
        default:
            w.WriteHeader(http.StatusInternalServerError)
            json.NewEncoder(w).Encode(map[string]string{
                "error": "Failed to create user",
            })
        }
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
    req.Email = normalizeEmail(req.Email)

    if accountLockout.Blocked(req.Email) {
        w.WriteHeader(http.StatusTooManyRequests)
        json.NewEncoder(w).Encode(map[string]string{
            "error": "Too many failed attempts for this account, please try again later",
        })
        return
    }

    // COLLATE NOCASE also matches any account stored before emails were
    // normalized to lowercase.
    var userID int64
    var passwordHash string
    err := ah.DB.QueryRow(
        "SELECT id, password_hash FROM users WHERE email = ? COLLATE NOCASE",
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
        // Sustained guessing against a real account, even spread across
        // many IPs (each individually under the per-IP rate limit), gets a
        // response: kill any session that account currently has, forcing
        // re-login everywhere, in case one of them was obtained some other
        // way rather than guessed here.
        if userFound && failedLoginTracker.RecordAndCheck(req.Email) {
            ah.DB.Exec("DELETE FROM sessions WHERE user_id = ?", userID)
        }
        accountLockout.Record(req.Email)

        w.WriteHeader(http.StatusUnauthorized)
        json.NewEncoder(w).Encode(map[string]string{
            "error": "Invalid credentials",
        })
        return
    }

    if err := startSession(ah.DB, w, r, userID); err != nil {
        w.WriteHeader(http.StatusInternalServerError)
        json.NewEncoder(w).Encode(map[string]string{
            "error": "Failed to create session",
        })
        return
    }
    accountLockout.Reset(req.Email)

    json.NewEncoder(w).Encode(map[string]interface{}{
        "message": "Login successful",
        "user_id": userID,
    })
}

// startSession creates a session for userID and sets its cookie on w.
// Shared by password login and external (OAuth) login, so both produce
// exactly the same kind of session.
func startSession(db *sql.DB, w http.ResponseWriter, r *http.Request, userID int64) error {
    sessionToken := uuid.New().String()
    expiresAt := time.Now().Add(sessionDuration)

    if _, err := db.Exec(
        "INSERT INTO sessions (user_id, session_token, expires_at, ip_address, fingerprint) VALUES (?, ?, ?, ?, ?)",
        userID, sessionToken, expiresAt, middleware.ClientIP(r), r.Header.Get(middleware.DeviceFingerprintHeader),
    ); err != nil {
        return err
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
    return nil
}

func (ah *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")

    cookie, err := r.Cookie("session_token")
    if err == nil {
        ah.DB.Exec("DELETE FROM sessions WHERE session_token = ?", cookie.Value)
        // A live connection is only authenticated when it opens - close
        // every one this session has (other tabs included), so they stop
        // receiving this user's messages and notifications.
        ah.Hub.DisconnectSession(cookie.Value)
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
    var storedIP, storedFingerprint sql.NullString
    err = ah.DB.QueryRow(
        "SELECT user_id, expires_at, ip_address, fingerprint FROM sessions WHERE session_token = ?",
        cookie.Value,
    ).Scan(&userID, &expiresAt, &storedIP, &storedFingerprint)

    if err != nil {
        json.NewEncoder(w).Encode(map[string]interface{}{"authenticated": false})
        return
    }

    if time.Now().After(expiresAt) {
        ah.DB.Exec("DELETE FROM sessions WHERE session_token = ?", cookie.Value)
        json.NewEncoder(w).Encode(map[string]interface{}{"authenticated": false})
        return
    }

    // A fingerprint mismatch is a strong signal this cookie is being used
    // from a different device than the one that logged in - invalidate
    // immediately. Only enforced when the caller actually sends the header;
    // requests that don't participate in fingerprinting aren't penalized.
    if fp := r.Header.Get(middleware.DeviceFingerprintHeader); fp != "" &&
        storedFingerprint.Valid && storedFingerprint.String != "" &&
        fp != storedFingerprint.String {
        ah.DB.Exec("DELETE FROM sessions WHERE session_token = ?", cookie.Value)
        json.NewEncoder(w).Encode(map[string]interface{}{"authenticated": false})
        return
    }

    // IP changes are common and often legitimate (mobile networks, Wi-Fi
    // switching) - logged for visibility rather than invalidated.
    if ip := middleware.ClientIP(r); storedIP.Valid && storedIP.String != "" && ip != storedIP.String {
        log.Printf("[security] session for user %d: IP changed from %s to %s", userID, storedIP.String, ip)
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

    if !isValidPassword(req.NewPassword) {
        w.WriteHeader(http.StatusBadRequest)
        json.NewEncoder(w).Encode(map[string]string{"error": "New password must be 12-15 characters and include a lowercase letter, an uppercase letter, a number, and a special character"})
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
    // Every session was just signed out - close their live connections too.
    ah.Hub.DisconnectUser(userID)

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