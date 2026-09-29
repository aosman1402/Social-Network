package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	gorilla "github.com/gorilla/websocket"

	"social-network/pkg/websocket"
)

// TestHandleWebSocket_OPTIONS covers the manual CORS preflight handling at
// the top of HandleWebSocket (a real WebSocket handshake isn't a normal
// CORS request, so it isn't covered by server.go's regular cors() wrapper -
// the handler does its own Origin check).
func TestHandleWebSocket_OPTIONS(t *testing.T) {
	tests := []struct {
		name       string
		origin     string
		wantOrigin string // expected Access-Control-Allow-Origin header, "" if it must be absent
	}{
		{"allowed origin", "http://localhost:3000", "http://localhost:3000"},
		{"disallowed origin", "http://evil.example.com", ""},
		{"no origin header", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := newTestDB(t)
			wsh := &WebSocketHandler{DB: db}

			r := httptest.NewRequest(http.MethodOptions, "/api/ws", nil)
			if tt.origin != "" {
				r.Header.Set("Origin", tt.origin)
			}
			w := httptest.NewRecorder()
			wsh.HandleWebSocket(w, r)

			if w.Code != http.StatusOK {
				t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
			}
			got := w.Header().Get("Access-Control-Allow-Origin")
			if got != tt.wantOrigin {
				t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, tt.wantOrigin)
			}
		})
	}
}

// TestHandleWebSocket_NoCookie covers a connection attempt with no session
// cookie at all.
func TestHandleWebSocket_NoCookie(t *testing.T) {
	db := newTestDB(t)
	wsh := &WebSocketHandler{DB: db}

	r := httptest.NewRequest(http.MethodGet, "/api/ws", nil)
	w := httptest.NewRecorder()
	wsh.HandleWebSocket(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
	assertJSONError(t, w)
}

// TestHandleWebSocket_EmptyCookie covers a session_token cookie that's
// present but empty - handled as a distinct case from "missing" in the
// handler, so it gets its own test.
func TestHandleWebSocket_EmptyCookie(t *testing.T) {
	db := newTestDB(t)
	wsh := &WebSocketHandler{DB: db}

	r := httptest.NewRequest(http.MethodGet, "/api/ws", nil)
	r.AddCookie(&http.Cookie{Name: "session_token", Value: ""})
	w := httptest.NewRecorder()
	wsh.HandleWebSocket(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
	assertJSONError(t, w)
}

// TestHandleWebSocket_InvalidToken covers a cookie value that matches no
// session row.
func TestHandleWebSocket_InvalidToken(t *testing.T) {
	db := newTestDB(t)
	wsh := &WebSocketHandler{DB: db}

	r := httptest.NewRequest(http.MethodGet, "/api/ws", nil)
	r.AddCookie(&http.Cookie{Name: "session_token", Value: "does-not-exist"})
	w := httptest.NewRecorder()
	wsh.HandleWebSocket(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
	assertJSONError(t, w)
}

// TestHandleWebSocket_ExpiredToken covers a session row that exists but has
// already expired.
func TestHandleWebSocket_ExpiredToken(t *testing.T) {
	db := newTestDB(t)
	wsh := &WebSocketHandler{DB: db}
	userID := registerTestUser(t, db, "ws-expired@example.com")

	token := "ws-expired-token"
	insertTestSession(t, db, userID, token, time.Now().Add(-time.Hour), "1.2.3.4", "")

	r := httptest.NewRequest(http.MethodGet, "/api/ws", nil)
	r.AddCookie(&http.Cookie{Name: "session_token", Value: token})
	w := httptest.NewRecorder()
	wsh.HandleWebSocket(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
	assertJSONError(t, w)
}

// TestHandleWebSocket_SuccessfulUpgrade covers the full happy path: a valid
// session upgrades the connection and the resulting client is registered
// with the hub, keyed by the user's own room. Needs a real listener (not
// httptest.ResponseRecorder, which doesn't implement http.Hijacker) since
// gorilla/websocket's Upgrade hijacks the underlying connection.
func TestHandleWebSocket_SuccessfulUpgrade(t *testing.T) {
	db := newTestDB(t)
	hub := websocket.NewHub()
	go hub.Run()
	wsh := &WebSocketHandler{DB: db, Hub: hub}

	userID := registerTestUser(t, db, "ws-success@example.com")
	token := "ws-success-token"
	insertTestSession(t, db, userID, token, time.Now().Add(time.Hour), "1.2.3.4", "")

	server := httptest.NewServer(http.HandlerFunc(wsh.HandleWebSocket))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	header := http.Header{}
	header.Set("Cookie", "session_token="+token)
	header.Set("Origin", "http://localhost:3000")

	conn, resp, err := gorilla.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		status := "<no response>"
		if resp != nil {
			status = resp.Status
		}
		t.Fatalf("Dial failed: %v (response: %s)", err, status)
	}
	defer conn.Close()

	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Errorf("handshake status = %d, want %d", resp.StatusCode, http.StatusSwitchingProtocols)
	}

	deadline := time.Now().Add(2 * time.Second)
	for !hub.IsUserOnline(userID) {
		if time.Now().After(deadline) {
			t.Fatal("user never showed up as online in the hub after connecting")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestHandleWebSocket_RejectsMissingOrDisallowedOrigin covers the upgrader's
// Origin check: even with a valid session cookie, a handshake with no
// Origin header, or one from a site that isn't our frontend, must be
// refused rather than upgraded.
func TestHandleWebSocket_RejectsMissingOrDisallowedOrigin(t *testing.T) {
	for name, origin := range map[string]string{
		"no origin header":  "",
		"disallowed origin": "http://evil.example.com",
	} {
		t.Run(name, func(t *testing.T) {
			db := newTestDB(t)
			hub := websocket.NewHub()
			go hub.Run()
			wsh := &WebSocketHandler{DB: db, Hub: hub}

			userID := registerTestUser(t, db, "ws-origin@example.com")
			token := "ws-origin-token"
			insertTestSession(t, db, userID, token, time.Now().Add(time.Hour), "1.2.3.4", "")

			server := httptest.NewServer(http.HandlerFunc(wsh.HandleWebSocket))
			defer server.Close()

			header := http.Header{}
			header.Set("Cookie", "session_token="+token)
			if origin != "" {
				header.Set("Origin", origin)
			}
			conn, resp, err := gorilla.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), header)
			if err == nil {
				conn.Close()
				t.Fatal("Dial succeeded, want the handshake to be refused")
			}
			if resp == nil || resp.StatusCode != http.StatusForbidden {
				t.Errorf("handshake response = %v, want 403 Forbidden", resp)
			}
		})
	}
}
