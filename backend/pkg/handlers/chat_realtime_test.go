package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	gorilla "github.com/gorilla/websocket"

	"social-network/pkg/websocket"
)

// dialTestWS opens a real WebSocket connection to server, authenticated
// with token, the same way a browser would (session cookie + Origin).
func dialTestWS(t *testing.T, serverURL, token string) *gorilla.Conn {
	t.Helper()
	wsURL := "ws" + strings.TrimPrefix(serverURL, "http")
	header := http.Header{}
	header.Set("Cookie", "session_token="+token)
	header.Set("Origin", "http://localhost:3000")
	conn, resp, err := gorilla.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		status := "<nil>"
		if resp != nil {
			status = resp.Status
		}
		t.Fatalf("dial failed: %v (resp: %s)", err, status)
	}
	return conn
}

func TestRealtimeChatDeliveredToFollowingRecipient(t *testing.T) {
	db := newTestDB(t)
	hub := websocket.NewHub()
	go hub.Run()
	wsh := &WebSocketHandler{DB: db, Hub: hub}

	userA := registerTestUser(t, db, "chat-a@example.com")
	userB := registerTestUser(t, db, "chat-b@example.com")

	now := time.Now()
	if _, err := db.Exec(`INSERT INTO follows (follower_id, following_id, status, created_at, updated_at) VALUES (?, ?, 'accepted', ?, ?)`, userA, userB, now, now); err != nil {
		t.Fatalf("insert follow: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO follows (follower_id, following_id, status, created_at, updated_at) VALUES (?, ?, 'accepted', ?, ?)`, userB, userA, now, now); err != nil {
		t.Fatalf("insert follow: %v", err)
	}

	tokenA, tokenB := "chat-token-a", "chat-token-b"
	insertTestSession(t, db, userA, tokenA, time.Now().Add(time.Hour), "1.2.3.4", "")
	insertTestSession(t, db, userB, tokenB, time.Now().Add(time.Hour), "1.2.3.4", "")

	server := httptest.NewServer(http.HandlerFunc(wsh.HandleWebSocket))
	defer server.Close()

	connA := dialTestWS(t, server.URL, tokenA)
	defer connA.Close()
	connB := dialTestWS(t, server.URL, tokenB)
	defer connB.Close()

	deadline := time.Now().Add(2 * time.Second)
	for !hub.IsUserOnline(userA) || !hub.IsUserOnline(userB) {
		if time.Now().After(deadline) {
			t.Fatal("users never came online")
		}
		time.Sleep(10 * time.Millisecond)
	}

	send := map[string]interface{}{
		"type": "chat",
		"data": map[string]interface{}{"recipient_id": userB, "content": "hello from A"},
	}
	b, _ := json.Marshal(send)
	if err := connA.WriteMessage(gorilla.TextMessage, b); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	connB.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, data, err := connB.ReadMessage()
	if err != nil {
		t.Fatalf("recipient never received the message in real time: %v", err)
	}

	var envelope struct {
		Type string `json:"type"`
		Data struct {
			Content     string `json:"content"`
			SenderID    int64  `json:"sender_id"`
			RecipientID int64  `json:"recipient_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		t.Fatalf("failed to parse envelope: %v", err)
	}
	if envelope.Type != "chat" {
		t.Errorf("type = %q, want %q", envelope.Type, "chat")
	}
	if envelope.Data.Content != "hello from A" {
		t.Errorf("content = %q, want %q", envelope.Data.Content, "hello from A")
	}
	if envelope.Data.SenderID != userA {
		t.Errorf("sender_id = %d, want %d", envelope.Data.SenderID, userA)
	}
}

// TestChatBlockedBetweenNonFollowingUsers is a regression test for the
// exact bug the audit caught: users default to a *public* account
// (is_public = TRUE), and CanDirectMessage previously let anyone message a
// public account regardless of follow status - meaning "not following each
// other at all" almost never actually blocked a chat in practice. Direct
// messages must require an actual follow relationship, full stop.
func TestChatBlockedBetweenNonFollowingUsers(t *testing.T) {
	db := newTestDB(t)
	hub := websocket.NewHub()
	go hub.Run()

	userA := registerTestUser(t, db, "nofollow-a@example.com")
	userB := registerTestUser(t, db, "nofollow-b@example.com")
	// Deliberately left public (the default) and with no follow row at all
	// in either direction.

	_, err := websocket.SendPrivateMessage(db, hub, userA, userB, "hi")
	if err != websocket.ErrNotAllowed {
		t.Fatalf("SendPrivateMessage err = %v, want ErrNotAllowed", err)
	}

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM messages WHERE sender_id = ? AND recipient_id = ?", userA, userB).Scan(&count); err != nil {
		t.Fatalf("query: %v", err)
	}
	if count != 0 {
		t.Errorf("message row count = %d, want 0 - a blocked message must never be persisted", count)
	}
}

// TestChatBlockedSendOverWebSocketNotifiesSender covers the other half of
// the same bug: previously, when a WebSocket "chat" send was blocked, the
// server only logged it - the sender's UI had already optimistically shown
// the message as delivered and never learned it failed. The sender must now
// receive a "chat_error" event, and the recipient must receive nothing.
func TestChatBlockedSendOverWebSocketNotifiesSender(t *testing.T) {
	db := newTestDB(t)
	hub := websocket.NewHub()
	go hub.Run()
	wsh := &WebSocketHandler{DB: db, Hub: hub}

	userA := registerTestUser(t, db, "blocked-a@example.com")
	userB := registerTestUser(t, db, "blocked-b@example.com")

	tokenA, tokenB := "blocked-token-a", "blocked-token-b"
	insertTestSession(t, db, userA, tokenA, time.Now().Add(time.Hour), "1.2.3.4", "")
	insertTestSession(t, db, userB, tokenB, time.Now().Add(time.Hour), "1.2.3.4", "")

	server := httptest.NewServer(http.HandlerFunc(wsh.HandleWebSocket))
	defer server.Close()

	connA := dialTestWS(t, server.URL, tokenA)
	defer connA.Close()
	connB := dialTestWS(t, server.URL, tokenB)
	defer connB.Close()

	deadline := time.Now().Add(2 * time.Second)
	for !hub.IsUserOnline(userA) || !hub.IsUserOnline(userB) {
		if time.Now().After(deadline) {
			t.Fatal("users never came online")
		}
		time.Sleep(10 * time.Millisecond)
	}

	send := map[string]interface{}{
		"type": "chat",
		"data": map[string]interface{}{"recipient_id": userB, "content": "should be blocked"},
	}
	b, _ := json.Marshal(send)
	if err := connA.WriteMessage(gorilla.TextMessage, b); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	// The sender must get a chat_error back.
	connA.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, data, err := connA.ReadMessage()
	if err != nil {
		t.Fatalf("sender never received a chat_error: %v", err)
	}
	var envelope struct {
		Type string `json:"type"`
		Data struct {
			Error       string `json:"error"`
			RecipientID int64  `json:"recipient_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		t.Fatalf("failed to parse envelope: %v", err)
	}
	if envelope.Type != "chat_error" {
		t.Errorf("type = %q, want %q", envelope.Type, "chat_error")
	}
	if envelope.Data.Error == "" {
		t.Error("expected a non-empty error message")
	}
	if envelope.Data.RecipientID != userB {
		t.Errorf("recipient_id = %d, want %d", envelope.Data.RecipientID, userB)
	}

	// The recipient must receive nothing at all.
	connB.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	if _, _, err := connB.ReadMessage(); err == nil {
		t.Error("recipient received a message that should have been blocked")
	}
}

func TestGetChatHistory_BlockedForNonFollowingUsers(t *testing.T) {
	db := newTestDB(t)
	wsh := &WebSocketHandler{DB: db}

	userA := registerTestUser(t, db, "history-blocked-a@example.com")
	userB := registerTestUser(t, db, "history-blocked-b@example.com")

	r := withUserID(httptest.NewRequest(http.MethodGet, "/api/messages/history?user_id="+strconv.FormatInt(userB, 10), nil), userA)
	w := httptest.NewRecorder()
	wsh.GetChatHistory(w, r)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d, body = %s", w.Code, http.StatusForbidden, w.Body.String())
	}
	assertJSONError(t, w)
}

func TestGetChatHistory_AllowedForFollowingUsers(t *testing.T) {
	db := newTestDB(t)
	wsh := &WebSocketHandler{DB: db}

	userA := registerTestUser(t, db, "history-allowed-a@example.com")
	userB := registerTestUser(t, db, "history-allowed-b@example.com")

	now := time.Now()
	if _, err := db.Exec(`INSERT INTO follows (follower_id, following_id, status, created_at, updated_at) VALUES (?, ?, 'accepted', ?, ?)`, userA, userB, now, now); err != nil {
		t.Fatalf("insert follow: %v", err)
	}

	r := withUserID(httptest.NewRequest(http.MethodGet, "/api/messages/history?user_id="+strconv.FormatInt(userB, 10), nil), userA)
	w := httptest.NewRecorder()
	wsh.GetChatHistory(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", w.Code, http.StatusOK, w.Body.String())
	}
}
