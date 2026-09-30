package handlers

import (
	"net/http"
	"testing"
)

func registerForTest(t *testing.T, auth *AuthHandler, email string) {
	t.Helper()
	w := doJSON(t, auth.Register, http.MethodPost, "/api/register", map[string]interface{}{
		"email":         email,
		"password":      "Sup3rSec9!zx",
		"first_name":    "Test",
		"last_name":     "User",
		"date_of_birth": "1990-01-01",
	}, nil)
	if w.Code != http.StatusCreated {
		t.Fatalf("Register %q: status = %d, body = %s", email, w.Code, w.Body.String())
	}
}

func loginForTest(t *testing.T, auth *AuthHandler, email, password string) int {
	t.Helper()
	w := doJSON(t, auth.Login, http.MethodPost, "/api/login", map[string]string{
		"email": email, "password": password,
	}, nil)
	return w.Code
}

// TestEmailIsNormalized covers emails differing only in case or surrounding
// spaces: they are the same account - stored lowercase, usable to log in
// with any capitalisation, and rejected as a duplicate at registration.
func TestEmailIsNormalized(t *testing.T) {
	db := newTestDB(t)
	auth := &AuthHandler{DB: db}

	registerForTest(t, auth, "  Mixed.Case@Example.COM ")

	var stored string
	if err := db.QueryRow("SELECT email FROM users WHERE email LIKE '%example.com'").Scan(&stored); err != nil {
		t.Fatalf("failed to read stored email: %v", err)
	}
	if stored != "mixed.case@example.com" {
		t.Errorf("stored email = %q, want %q", stored, "mixed.case@example.com")
	}

	w := doJSON(t, auth.Register, http.MethodPost, "/api/register", map[string]interface{}{
		"email":         "MIXED.CASE@example.com",
		"password":      "Sup3rSec9!zx",
		"first_name":    "Copy",
		"last_name":     "User",
		"date_of_birth": "1990-01-01",
	}, nil)
	if w.Code != http.StatusConflict {
		t.Errorf("Register (same email, other case): status = %d, want %d", w.Code, http.StatusConflict)
	}

	for _, email := range []string{"mixed.case@example.com", "MIXED.CASE@EXAMPLE.COM", " Mixed.Case@Example.com "} {
		if got := loginForTest(t, auth, email, "Sup3rSec9!zx"); got != http.StatusOK {
			t.Errorf("Login as %q: status = %d, want %d", email, got, http.StatusOK)
		}
	}
}

// TestLoginMatchesLegacyMixedCaseEmail covers an account stored with
// capitals before emails were normalized - it must still be able to log in.
func TestLoginMatchesLegacyMixedCaseEmail(t *testing.T) {
	db := newTestDB(t)
	auth := &AuthHandler{DB: db}

	registerForTest(t, auth, "legacy@example.com")
	if _, err := db.Exec("UPDATE users SET email = 'Legacy@Example.com' WHERE email = 'legacy@example.com'"); err != nil {
		t.Fatalf("failed to simulate legacy email: %v", err)
	}

	if got := loginForTest(t, auth, "legacy@example.com", "Sup3rSec9!zx"); got != http.StatusOK {
		t.Errorf("Login: status = %d, want %d", got, http.StatusOK)
	}
}

// TestLoginLocksAccountAfterFiveFailures covers the per-account lockout:
// after 5 wrong passwords the account is blocked with 429 - even for the
// correct password, and whatever capitalisation of the email is used -
// while other accounts are unaffected.
func TestLoginLocksAccountAfterFiveFailures(t *testing.T) {
	db := newTestDB(t)
	auth := &AuthHandler{DB: db}
	registerForTest(t, auth, "locked@example.com")
	registerForTest(t, auth, "bystander@example.com")

	for i := 0; i < 5; i++ {
		if got := loginForTest(t, auth, "locked@example.com", "Wr0ngPassw0rd!"); got != http.StatusUnauthorized {
			t.Fatalf("wrong password %d: status = %d, want %d", i+1, got, http.StatusUnauthorized)
		}
	}

	if got := loginForTest(t, auth, "locked@example.com", "Sup3rSec9!zx"); got != http.StatusTooManyRequests {
		t.Errorf("correct password while locked: status = %d, want %d", got, http.StatusTooManyRequests)
	}
	if got := loginForTest(t, auth, "LOCKED@example.com", "Sup3rSec9!zx"); got != http.StatusTooManyRequests {
		t.Errorf("other capitalisation while locked: status = %d, want %d", got, http.StatusTooManyRequests)
	}
	if got := loginForTest(t, auth, "bystander@example.com", "Sup3rSec9!zx"); got != http.StatusOK {
		t.Errorf("different account: status = %d, want %d", got, http.StatusOK)
	}
}

// TestLoginSuccessResetsFailures covers a real user's typos: a successful
// login clears earlier failures, so they don't add up to a lockout later.
func TestLoginSuccessResetsFailures(t *testing.T) {
	db := newTestDB(t)
	auth := &AuthHandler{DB: db}
	registerForTest(t, auth, "typos@example.com")

	for round := 0; round < 2; round++ {
		for i := 0; i < 4; i++ {
			loginForTest(t, auth, "typos@example.com", "Wr0ngPassw0rd!")
		}
		if got := loginForTest(t, auth, "typos@example.com", "Sup3rSec9!zx"); got != http.StatusOK {
			t.Fatalf("round %d, correct password after 4 typos: status = %d, want %d", round+1, got, http.StatusOK)
		}
	}
}

// TestLoginLockoutDoesNotRevealUnknownEmails covers an email that isn't
// registered: it gets the same 401s and then the same 429 as a real one,
// so the lockout can't be used to find out which emails exist.
func TestLoginLockoutDoesNotRevealUnknownEmails(t *testing.T) {
	db := newTestDB(t)
	auth := &AuthHandler{DB: db}

	for i := 0; i < 5; i++ {
		if got := loginForTest(t, auth, "nobody@example.com", "Wr0ngPassw0rd!"); got != http.StatusUnauthorized {
			t.Fatalf("attempt %d: status = %d, want %d", i+1, got, http.StatusUnauthorized)
		}
	}
	if got := loginForTest(t, auth, "nobody@example.com", "Wr0ngPassw0rd!"); got != http.StatusTooManyRequests {
		t.Errorf("6th attempt: status = %d, want %d", got, http.StatusTooManyRequests)
	}
}
