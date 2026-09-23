package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func checkNicknameRequest(nickname string) *http.Request {
	return httptest.NewRequest(http.MethodGet, "/api/check-nickname?nickname="+url.QueryEscape(nickname), nil)
}

func decodeCheckNickname(t *testing.T, w *httptest.ResponseRecorder) map[string]bool {
	t.Helper()
	var resp map[string]bool
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("CheckNickname: failed to parse response JSON: %v (body: %s)", err, w.Body.String())
	}
	return resp
}

// TestCheckNicknameInvalidFormat covers nicknames that fail the format rule
// (3-14 chars, letters/digits only) - these must be reported invalid
// without ever touching the database, so no format ever "coincidentally"
// looks available.
func TestCheckNicknameInvalidFormat(t *testing.T) {
	db := newTestDB(t)
	auth := &AuthHandler{DB: db}

	cases := []string{
		"",                // empty
		"ab",              // too short
		"a2345678901234b", // too long (15 chars)
		"has space",
		"has-dash",
		"has_underscore",
		"emoji😀name",
	}

	for _, nickname := range cases {
		w := httptest.NewRecorder()
		auth.CheckNickname(w, checkNicknameRequest(nickname))

		resp := decodeCheckNickname(t, w)
		if resp["valid"] != false {
			t.Errorf("nickname %q: valid = %v, want false", nickname, resp["valid"])
		}
		if resp["available"] != false {
			t.Errorf("nickname %q: available = %v, want false", nickname, resp["available"])
		}
	}
}

// TestCheckNicknameAvailable covers a well-formed nickname nobody has
// registered yet.
func TestCheckNicknameAvailable(t *testing.T) {
	db := newTestDB(t)
	auth := &AuthHandler{DB: db}

	w := httptest.NewRecorder()
	auth.CheckNickname(w, checkNicknameRequest("FreshNick1"))

	resp := decodeCheckNickname(t, w)
	if resp["valid"] != true {
		t.Errorf("valid = %v, want true", resp["valid"])
	}
	if resp["available"] != true {
		t.Errorf("available = %v, want true", resp["available"])
	}
}

// TestCheckNicknameTaken covers a well-formed nickname that's already
// registered to another user - valid format, but not available.
func TestCheckNicknameTaken(t *testing.T) {
	db := newTestDB(t)
	auth := &AuthHandler{DB: db}

	nickname := "TakenNick1"
	w := doJSON(t, auth.Register, http.MethodPost, "/api/register", map[string]interface{}{
		"email":         "nickholder@example.com",
		"password":      "Sup3rSec9!zx",
		"first_name":    "Test",
		"last_name":     "User",
		"date_of_birth": "1990-01-01",
		"nickname":      nickname,
	}, nil)
	if w.Code != http.StatusCreated {
		t.Fatalf("Register: status = %d, body = %s", w.Code, w.Body.String())
	}

	checkW := httptest.NewRecorder()
	auth.CheckNickname(checkW, checkNicknameRequest(nickname))

	resp := decodeCheckNickname(t, checkW)
	if resp["valid"] != true {
		t.Errorf("valid = %v, want true", resp["valid"])
	}
	if resp["available"] != false {
		t.Errorf("available = %v, want false - nickname is already registered", resp["available"])
	}
}
