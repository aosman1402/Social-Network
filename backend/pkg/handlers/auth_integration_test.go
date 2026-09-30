package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func doJSON(t *testing.T, handler http.HandlerFunc, method, target string, body interface{}, cookies []*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()

	var bodyReader *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("failed to marshal request body: %v", err)
		}
		bodyReader = bytes.NewReader(b)
	} else {
		bodyReader = bytes.NewReader(nil)
	}

	r := httptest.NewRequest(method, target, bodyReader)
	r.Header.Set("Content-Type", "application/json")
	for _, c := range cookies {
		r.AddCookie(c)
	}

	w := httptest.NewRecorder()
	handler(w, r)
	return w
}

func TestRegisterAndLogin(t *testing.T) {
	db := newTestDB(t)
	auth := &AuthHandler{DB: db}

	registerBody := map[string]interface{}{
		"email":         "integration@example.com",
		"password":      "Sup3rSec9!zx",
		"first_name":    "Test",
		"last_name":     "User",
		"date_of_birth": "1990-01-01",
	}

	w := doJSON(t, auth.Register, http.MethodPost, "/api/register", registerBody, nil)
	if w.Code != http.StatusCreated {
		t.Fatalf("Register: status = %d, body = %s", w.Code, w.Body.String())
	}

	var registerResp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &registerResp); err != nil {
		t.Fatalf("Register: failed to parse response JSON: %v (body: %s)", err, w.Body.String())
	}
	if _, ok := registerResp["user_id"]; !ok {
		t.Errorf("Register: response missing user_id: %v", registerResp)
	}

	// Registering the same email again must be rejected, not silently
	// accepted or crash.
	w = doJSON(t, auth.Register, http.MethodPost, "/api/register", registerBody, nil)
	if w.Code != http.StatusConflict {
		t.Errorf("Register (duplicate email): status = %d, want %d", w.Code, http.StatusConflict)
	}
	assertJSONError(t, w)

	// Login with the wrong password must fail.
	w = doJSON(t, auth.Login, http.MethodPost, "/api/login", map[string]string{
		"email": "integration@example.com", "password": "WrongPassword9!zx",
	}, nil)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("Login (wrong password): status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
	assertJSONError(t, w)

	// Login with the correct password must succeed and set a session
	// cookie - never leak the token in the response body.
	w = doJSON(t, auth.Login, http.MethodPost, "/api/login", map[string]string{
		"email": "integration@example.com", "password": "Sup3rSec9!zx",
	}, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("Login: status = %d, body = %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "session_token") {
		t.Errorf("Login response body leaks the session token: %s", w.Body.String())
	}

	result := w.Result()
	cookies := result.Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "session_token" {
			sessionCookie = c
		}
	}
	if sessionCookie == nil {
		t.Fatalf("Login: no session_token cookie set; cookies = %v", cookies)
	}
	if !sessionCookie.HttpOnly {
		t.Error("session_token cookie must be HttpOnly")
	}

	// CheckAuth with that cookie must now report authenticated.
	w = doJSON(t, auth.CheckAuth, http.MethodGet, "/api/check-auth", nil, []*http.Cookie{sessionCookie})
	var checkResp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &checkResp); err != nil {
		t.Fatalf("CheckAuth: failed to parse response JSON: %v (body: %s)", err, w.Body.String())
	}
	if checkResp["authenticated"] != true {
		t.Errorf("CheckAuth: authenticated = %v, want true", checkResp["authenticated"])
	}
	if checkResp["email"] != "integration@example.com" {
		t.Errorf("CheckAuth: email = %v, want integration@example.com", checkResp["email"])
	}
}

func TestRegisterRejectsWeakPassword(t *testing.T) {
	db := newTestDB(t)
	auth := &AuthHandler{DB: db}

	w := doJSON(t, auth.Register, http.MethodPost, "/api/register", map[string]interface{}{
		"email":         "weakpass@example.com",
		"password":      "short1!",
		"first_name":    "Test",
		"last_name":     "User",
		"date_of_birth": "1990-01-01",
	}, nil)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
	assertJSONError(t, w)
}

// assertJSONError fails the test if w's body isn't valid JSON with a
// non-empty "error" field - every error response in this codebase must be
// JSON (see writeJSONError's doc comment for why: the frontend always
// calls response.json() and a plain-text body breaks that silently).
func assertJSONError(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("error response is not valid JSON: %v (body: %s)", err, w.Body.String())
	}
	if msg, _ := body["error"].(string); msg == "" {
		t.Errorf("error response missing non-empty \"error\" field: %v", body)
	}
}
