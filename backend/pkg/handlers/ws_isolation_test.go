package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	gorilla "github.com/gorilla/websocket"

	"social-network/pkg/websocket"
)

// wsTestEnv is a live WebSocket server plus helpers for connecting users.
type wsTestEnv struct {
	t      *testing.T
	wsh    *WebSocketHandler
	server *httptest.Server
}

func newWSTestEnv(t *testing.T) *wsTestEnv {
	t.Helper()
	db := newTestDB(t)
	hub := websocket.NewHub()
	go hub.Run()
	wsh := &WebSocketHandler{DB: db, Hub: hub}
	server := httptest.NewServer(http.HandlerFunc(wsh.HandleWebSocket))
	t.Cleanup(server.Close)
	return &wsTestEnv{t: t, wsh: wsh, server: server}
}

// connect registers a user, opens a WebSocket for them and waits until the
// hub has them online.
func (e *wsTestEnv) connect(email string) (int64, *gorilla.Conn) {
	e.t.Helper()
	userID := registerTestUser(e.t, e.wsh.DB, email)
	token := "tok-" + email
	insertTestSession(e.t, e.wsh.DB, userID, token, time.Now().Add(time.Hour), "1.2.3.4", "")
	conn := dialTestWS(e.t, e.server.URL, token)
	e.t.Cleanup(func() { conn.Close() })

	deadline := time.Now().Add(2 * time.Second)
	for !e.wsh.Hub.IsUserOnline(userID) {
		if time.Now().After(deadline) {
			e.t.Fatalf("%s never came online", email)
		}
		time.Sleep(10 * time.Millisecond)
	}
	return userID, conn
}

func writeFrame(t *testing.T, conn *gorilla.Conn, frame map[string]interface{}) {
	t.Helper()
	b, _ := json.Marshal(frame)
	if err := conn.WriteMessage(gorilla.TextMessage, b); err != nil {
		t.Fatalf("write failed: %v", err)
	}
}

// receivedFrames collects everything conn receives within wait.
func receivedFrames(conn *gorilla.Conn, wait time.Duration) []string {
	var frames []string
	conn.SetReadDeadline(time.Now().Add(wait))
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return frames
		}
		frames = append(frames, string(data))
	}
}

// TestJoinRoomCannotEavesdrop guards against a real hole: a client could
// send {"type":"join_room","room_id":"user_<id>"} for any id and then
// receive that user's private messages and notifications live.
func TestJoinRoomCannotEavesdrop(t *testing.T) {
	env := newWSTestEnv(t)
	aliceID, _ := env.connect("alice@example.com")
	bobID, _ := env.connect("bob@example.com")
	_, eveConn := env.connect("eve@example.com")
	insertAcceptedFollow(t, env.wsh.DB, bobID, aliceID)

	writeFrame(t, eveConn, map[string]interface{}{"type": "join_room", "room_id": "user_" + itoa(aliceID)})
	time.Sleep(100 * time.Millisecond)

	if _, err := websocket.SendPrivateMessage(env.wsh.DB, env.wsh.Hub, bobID, aliceID, "secret for alice", false); err != nil {
		t.Fatalf("SendPrivateMessage: %v", err)
	}

	for _, f := range receivedFrames(eveConn, 300*time.Millisecond) {
		if strings.Contains(f, "secret for alice") {
			t.Fatalf("eavesdropper received someone else's DM: %s", f)
		}
	}
}

// TestTypingNotRelayedToNonFollowers - typing events used to be forwarded
// raw to any user id, letting a stranger push arbitrary data to anyone.
func TestTypingNotRelayedToNonFollowers(t *testing.T) {
	env := newWSTestEnv(t)
	_, strangerConn := env.connect("stranger@example.com")
	targetID, targetConn := env.connect("target@example.com")

	writeFrame(t, strangerConn, map[string]interface{}{
		"type": "typing",
		"data": map[string]interface{}{"recipient_id": targetID, "is_typing": true, "content": "smuggled"},
	})

	if frames := receivedFrames(targetConn, 300*time.Millisecond); len(frames) != 0 {
		t.Fatalf("non-follower's typing event was delivered: %v", frames)
	}
}

// TestTypingRelayedWithoutExtraFields - between users who may chat, the
// typing event arrives, but only with the server's fixed fields.
func TestTypingRelayedWithoutExtraFields(t *testing.T) {
	env := newWSTestEnv(t)
	senderID, senderConn := env.connect("sender@example.com")
	targetID, targetConn := env.connect("target@example.com")
	insertAcceptedFollow(t, env.wsh.DB, senderID, targetID)

	writeFrame(t, senderConn, map[string]interface{}{
		"type": "typing",
		"data": map[string]interface{}{"recipient_id": targetID, "is_typing": true, "content": "smuggled"},
	})

	frames := receivedFrames(targetConn, 300*time.Millisecond)
	if len(frames) != 1 {
		t.Fatalf("got %d frames, want 1 typing event: %v", len(frames), frames)
	}
	if strings.Contains(frames[0], "smuggled") {
		t.Errorf("typing event carried client-supplied fields: %s", frames[0])
	}
	if !strings.Contains(frames[0], `"type":"typing"`) || !strings.Contains(frames[0], `"sender_id":`+itoa(senderID)) {
		t.Errorf("unexpected typing event: %s", frames[0])
	}
}

// TestGroupMessageOnlyReachesMembers - group messages go to each active
// member's own channel; a non-member can't receive them (previously by
// joining the "group_<id>" room).
func TestGroupMessageOnlyReachesMembers(t *testing.T) {
	env := newWSTestEnv(t)
	gh := &GroupHandler{DB: env.wsh.DB}

	ownerID, ownerConn := env.connect("owner@example.com")
	memberID, memberConn := env.connect("member@example.com")
	_, outsiderConn := env.connect("outsider@example.com")

	groupID := createTestGroup(t, gh, ownerID)
	if _, err := env.wsh.DB.Exec(
		`INSERT INTO group_members (group_id, user_id, status) VALUES (?, ?, 'active')`,
		groupID, memberID,
	); err != nil {
		t.Fatalf("add member: %v", err)
	}

	writeFrame(t, outsiderConn, map[string]interface{}{"type": "join_room", "room_id": "group_" + itoa(groupID)})
	time.Sleep(100 * time.Millisecond)

	writeFrame(t, ownerConn, map[string]interface{}{
		"type": "chat",
		"data": map[string]interface{}{"group_id": groupID, "content": "members only"},
	})

	gotMember := false
	for _, f := range receivedFrames(memberConn, 400*time.Millisecond) {
		if strings.Contains(f, "members only") {
			gotMember = true
		}
	}
	if !gotMember {
		t.Error("group member did not receive the group message")
	}
	for _, f := range receivedFrames(outsiderConn, 200*time.Millisecond) {
		if strings.Contains(f, "members only") {
			t.Fatalf("non-member received a group message: %s", f)
		}
	}
}

func itoa(n int64) string {
	b, _ := json.Marshal(n)
	return string(b)
}
