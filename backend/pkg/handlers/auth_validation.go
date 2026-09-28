package handlers

import (
    "encoding/json"
    "net/http"
    "strings"
)

// isValidEmail is a minimal, dependency-free format check: does the address
// contain "@", and is there a "." somewhere after it. It's not a full RFC
// 5322 validator - it's just enough to catch an obviously malformed address
// (no "@" at all, or no domain dot), matching what the frontend flags to the
// user, but enforced here too so a direct API call can't skip it.
func isValidEmail(email string) bool {
    at := strings.Index(email, "@")
    if at == -1 || at == len(email)-1 {
        return false
    }
    return strings.Contains(email[at+1:], ".")
}

const (
    minPasswordLength = 12
    maxPasswordLength = 15
    // passwordSpecialChars: the space character plus every standard
    // printable-ASCII punctuation mark.
    passwordSpecialChars = " !\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~"
)

// isValidPassword enforces the site's password policy: 12-15 characters,
// made up entirely of lowercase letters, uppercase letters, digits, and/or
// passwordSpecialChars, with at least one of each of those four categories
// present. Mirrors the frontend's live requirements checklist, enforced here
// too so a direct API call can't skip it.
func isValidPassword(password string) bool {
    length := 0
    var hasLower, hasUpper, hasDigit, hasSpecial bool

    for _, r := range password {
        length++
        switch {
        case r >= 'a' && r <= 'z':
            hasLower = true
        case r >= 'A' && r <= 'Z':
            hasUpper = true
        case r >= '0' && r <= '9':
            hasDigit = true
        case strings.ContainsRune(passwordSpecialChars, r):
            hasSpecial = true
        default:
            return false // character outside every allowed category
        }
    }

    return length >= minPasswordLength && length <= maxPasswordLength &&
        hasLower && hasUpper && hasDigit && hasSpecial
}

const (
    minNicknameLength = 3
    maxNicknameLength = 14
)

// isValidNicknameFormat reports whether nickname is 3-14 characters made up
// entirely of ASCII letters and digits. It doesn't check uniqueness - that's
// a separate DB lookup, done by callers (Register, CheckNickname).
func isValidNicknameFormat(nickname string) bool {
    length := 0
    for _, r := range nickname {
        length++
        isLower := r >= 'a' && r <= 'z'
        isUpper := r >= 'A' && r <= 'Z'
        isDigit := r >= '0' && r <= '9'
        if !isLower && !isUpper && !isDigit {
            return false
        }
    }
    return length >= minNicknameLength && length <= maxNicknameLength
}

// nicknameTaken reports whether nickname is already registered to a user.
func (ah *AuthHandler) nicknameTaken(nickname string) (bool, error) {
    var count int
    err := ah.DB.QueryRow("SELECT COUNT(*) FROM users WHERE nickname = ?", nickname).Scan(&count)
    return count > 0, err
}

// emailTaken reports whether email is already registered to a user.
// normalizeEmail trims surrounding spaces and lowercases an email, so
// "Alice@T.test " and "alice@t.test" are the same account everywhere
// (registration, login, and the per-account login lockout).
func normalizeEmail(email string) string {
    return strings.ToLower(strings.TrimSpace(email))
}

// emailTaken ignores case, so a differently-capitalised copy of an existing
// email (including one stored before emails were normalized) counts as taken.
func (ah *AuthHandler) emailTaken(email string) (bool, error) {
    var count int
    err := ah.DB.QueryRow("SELECT COUNT(*) FROM users WHERE email = ? COLLATE NOCASE", email).Scan(&count)
    return count > 0, err
}

// CheckNickname reports whether a nickname is well-formed and not already
// taken. Public (no auth required) - it's used while filling out the
// registration form, before an account (and therefore a session) exists.
func (ah *AuthHandler) CheckNickname(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")

    nickname := r.URL.Query().Get("nickname")
    if !isValidNicknameFormat(nickname) {
        json.NewEncoder(w).Encode(map[string]bool{
            "valid":     false,
            "available": false,
        })
        return
    }

    taken, err := ah.nicknameTaken(nickname)
    if err != nil {
        w.WriteHeader(http.StatusInternalServerError)
        json.NewEncoder(w).Encode(map[string]string{"error": "Failed to check nickname"})
        return
    }

    json.NewEncoder(w).Encode(map[string]bool{
        "valid":     true,
        "available": !taken,
    })
}
