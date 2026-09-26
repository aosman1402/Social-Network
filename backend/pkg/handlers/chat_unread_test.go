package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	gorilla "github.com/gorilla/websocket"

	"social-network/pkg/websocket"
)

// unreadMessageCount calls GetUnreadMessageCount as userID.
func unreadMessageCount(t *testing.T, wsh *WebSocketHandler, userID int64) int {
	t.Helper()
	w := httptest.NewRecorder()
	wsh.GetUnreadMessageCount(w, withUserID(httptest.NewRequest(http.MethodGet, "/api/messages/unread-count", nil), userID))
	if w.Code != http.StatusOK {
		t.Fatalf("GetUnreadMessageCount: status = %d, body = %s", w.Code, w.Body.String())
	}
	var resp struct {
		UnreadCount int `json:"unread_count"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("bad response: %s", w.Body.String())
	}
	return resp.UnreadCount
}

// readEventOfType reads frames from conn until one of type want arrives.
func readEventOfType(t *testing.T, conn *gorilla.Conn, want string) json.RawMessage {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("never received a %q event: %v", want, err)
		}
		var envelope struct {
			Type string          `json:"type"`
			Data json.RawMessage `json:"data"`
		}
		if json.Unmarshal(data, &envelope) == nil && envelope.Type == want {
			return envelope.Data
		}
	}
}

// TestChatUnreadCountAndReadReceipt covers the navbar's Chat (N) badge: the
// unread count goes up when a message arrives, and reading it (a WebSocket
// read receipt) brings it back down and pushes "messages_read" to the
// reader so every open tab can resync its badge.
func TestChatUnreadCountAndReadReceipt(t *testing.T) {
	db := newTestDB(t)
	hub := websocket.NewHub()
	go hub.Run()
	wsh := &WebSocketHandler{DB: db, Hub: hub}

	userA := registerTestUser(t, db, "unread-a@example.com")
	userB := registerTestUser(t, db, "unread-b@example.com")

	now := time.Now()
	if _, err := db.Exec(`INSERT INTO follows (follower_id, following_id, status, created_at, updated_at) VALUES (?, ?, 'accepted', ?, ?)`, userA, userB, now, now); err != nil {
		t.Fatalf("insert follow: %v", err)
	}

	tokenB := "unread-token-b"
	insertTestSession(t, db, userB, tokenB, time.Now().Add(time.Hour), "1.2.3.4", "")

	server := httptest.NewServer(http.HandlerFunc(wsh.HandleWebSocket))
	defer server.Close()
	connB := dialTestWS(t, server.URL, tokenB)
	defer connB.Close()

	deadline := time.Now().Add(2 * time.Second)
	for !hub.IsUserOnline(userB) {
		if time.Now().After(deadline) {
			t.Fatal("recipient never came online")
		}
		time.Sleep(10 * time.Millisecond)
	}

	if got := unreadMessageCount(t, wsh, userB); got != 0 {
		t.Fatalf("unread before any message = %d, want 0", got)
	}

	var messageIDs []int64
	for _, content := range []string{"one", "two"} {
		sent, err := websocket.SendPrivateMessage(db, hub, userA, userB, content)
		if err != nil {
			t.Fatalf("SendPrivateMessage: %v", err)
		}
		messageIDs = append(messageIDs, sent.MessageID)
		readEventOfType(t, connB, "chat")
	}

	if got := unreadMessageCount(t, wsh, userB); got != 2 {
		t.Errorf("recipient unread = %d, want 2", got)
	}
	if got := unreadMessageCount(t, wsh, userA); got != 0 {
		t.Errorf("sender unread = %d, want 0 (own messages don't count)", got)
	}

	receipt, _ := json.Marshal(map[string]interface{}{
		"type": "read_receipt",
		"data": map[string]interface{}{"message_id": messageIDs[0]},
	})
	if err := connB.WriteMessage(gorilla.TextMessage, receipt); err != nil {
		t.Fatalf("write read receipt: %v", err)
	}
	readEventOfType(t, connB, "messages_read")

	if got := unreadMessageCount(t, wsh, userB); got != 1 {
		t.Errorf("recipient unread after reading one = %d, want 1", got)
	}
}
