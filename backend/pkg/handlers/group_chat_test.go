package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	gorilla "github.com/gorilla/websocket"
)

// groupChatFixture is a group with two active members (owner, member) and
// an outsider, all connected over WebSocket.
type groupChatFixture struct {
	env                           *wsTestEnv
	groupID                       int64
	ownerID, memberID, outsiderID int64
	ownerConn, memberConn         *gorilla.Conn
	outsiderConn                  *gorilla.Conn
}

func newGroupChatFixture(t *testing.T) *groupChatFixture {
	t.Helper()
	env := newWSTestEnv(t)
	f := &groupChatFixture{env: env}
	f.ownerID, f.ownerConn = env.connect("owner@example.com")
	f.memberID, f.memberConn = env.connect("member@example.com")
	f.outsiderID, f.outsiderConn = env.connect("outsider@example.com")

	f.groupID = createTestGroup(t, &GroupHandler{DB: env.wsh.DB}, f.ownerID)
	if _, err := env.wsh.DB.Exec(
		`INSERT INTO group_members (group_id, user_id, status) VALUES (?, ?, 'active')`,
		f.groupID, f.memberID,
	); err != nil {
		t.Fatalf("add member: %v", err)
	}
	return f
}

// groupHistory calls GetGroupChatHistory as userID.
func (f *groupChatFixture) groupHistory(t *testing.T, userID int64) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/messages/group-history?group_id="+itoa(f.groupID), nil)
	f.env.wsh.GetGroupChatHistory(w, withUserID(req, userID))
	return w
}

// chatFramesContaining counts the "chat" events among frames that carry text.
func chatFramesContaining(frames []string, text string) int {
	n := 0
	for _, fr := range frames {
		if strings.Contains(fr, `"type":"chat"`) && strings.Contains(fr, text) {
			n++
		}
	}
	return n
}

// TestGroupChatDeliveredOnceToEachMember - the sender's own copy comes from
// the server (the chat panel shows no optimistic copy), so every active
// member, sender included, must get exactly one "chat" event per message.
func TestGroupChatDeliveredOnceToEachMember(t *testing.T) {
	f := newGroupChatFixture(t)

	writeFrame(t, f.ownerConn, map[string]interface{}{
		"type": "chat",
		"data": map[string]interface{}{"group_id": f.groupID, "content": "hello group 👋"},
	})

	if n := chatFramesContaining(receivedFrames(f.ownerConn, 400*time.Millisecond), "hello group"); n != 1 {
		t.Errorf("sender received %d copies, want 1", n)
	}
	memberFrames := receivedFrames(f.memberConn, 200*time.Millisecond)
	if n := chatFramesContaining(memberFrames, "hello group"); n != 1 {
		t.Errorf("member received %d copies, want 1", n)
	}
	if n := chatFramesContaining(receivedFrames(f.outsiderConn, 200*time.Millisecond), "hello group"); n != 0 {
		t.Errorf("non-member received %d copies, want 0", n)
	}

	// The sender has no nickname or avatar (both NULL) - that used to fail
	// the sender lookup and deliver the message with an empty sender.
	for _, fr := range memberFrames {
		if !strings.Contains(fr, "hello group") {
			continue
		}
		var env struct {
			Data struct {
				Content string `json:"content"`
				Sender  struct {
					ID        int64  `json:"id"`
					FirstName string `json:"first_name"`
				} `json:"sender"`
			} `json:"data"`
		}
		if err := json.Unmarshal([]byte(fr), &env); err != nil {
			t.Fatalf("bad frame %s: %v", fr, err)
		}
		if env.Data.Sender.ID != f.ownerID || env.Data.Sender.FirstName == "" {
			t.Errorf("sender not filled in: %+v", env.Data.Sender)
		}
		if env.Data.Content != "hello group 👋" {
			t.Errorf("content = %q, emoji not preserved", env.Data.Content)
		}
	}
}

// TestGroupChatHistory - members can read the history, a non-member can't.
func TestGroupChatHistory(t *testing.T) {
	f := newGroupChatFixture(t)

	writeFrame(t, f.ownerConn, map[string]interface{}{
		"type": "chat",
		"data": map[string]interface{}{"group_id": f.groupID, "content": "saved message"},
	})
	receivedFrames(f.memberConn, 400*time.Millisecond) // wait for delivery

	if w := f.groupHistory(t, f.outsiderID); w.Code != http.StatusForbidden {
		t.Errorf("non-member history: status = %d, want 403", w.Code)
	}

	w := f.groupHistory(t, f.memberID)
	if w.Code != http.StatusOK {
		t.Fatalf("member history: status = %d, body = %s", w.Code, w.Body.String())
	}
	var msgs []ChatMessage
	if err := json.Unmarshal(w.Body.Bytes(), &msgs); err != nil {
		t.Fatalf("bad history body %s: %v", w.Body.String(), err)
	}
	if len(msgs) != 1 || msgs[0].Content != "saved message" || msgs[0].SenderID != f.ownerID {
		t.Fatalf("history = %+v, want the owner's one message", msgs)
	}
	if msgs[0].Sender == nil || msgs[0].Sender.FirstName == "" {
		t.Errorf("history message has no sender name: %+v", msgs[0].Sender)
	}
}

// TestGroupChatRejectsInvalidContent - blank and over-long messages are
// neither saved nor delivered, and the sender gets a chat_error.
func TestGroupChatRejectsInvalidContent(t *testing.T) {
	f := newGroupChatFixture(t)

	for _, content := range []string{"   ", strings.Repeat("a", 1001)} {
		writeFrame(t, f.ownerConn, map[string]interface{}{
			"type": "chat",
			"data": map[string]interface{}{"group_id": f.groupID, "content": content},
		})
	}
	// One read only - a gorilla conn can't be read again after its read
	// deadline expires.
	frames := receivedFrames(f.ownerConn, 400*time.Millisecond)
	errors := 0
	for _, fr := range frames {
		if strings.Contains(fr, `"type":"chat_error"`) {
			errors++
		}
	}
	if errors != 2 || len(frames) != 2 {
		t.Errorf("got frames %v, want two chat_errors and nothing else", frames)
	}

	var count int
	f.env.wsh.DB.QueryRow("SELECT COUNT(*) FROM messages WHERE group_id = ?", f.groupID).Scan(&count)
	if count != 0 {
		t.Errorf("%d invalid messages were saved", count)
	}
	if frames := receivedFrames(f.memberConn, 100*time.Millisecond); chatFramesContaining(frames, "") != 0 {
		t.Errorf("member received invalid messages: %v", frames)
	}
}

// TestGroupChatBurstArrivesWholeAndInOrder - a quick run of messages must
// reach members as one parseable JSON event per frame, in the order sent.
// Queued events used to be packed into one newline-separated frame, which
// the browser's JSON.parse rejects - so bursts were silently lost - and
// each message was handled in its own goroutine, which shuffled the order.
func TestGroupChatBurstArrivesWholeAndInOrder(t *testing.T) {
	f := newGroupChatFixture(t)
	const n = 100

	for i := 0; i < n; i++ {
		writeFrame(t, f.ownerConn, map[string]interface{}{
			"type": "chat",
			"data": map[string]interface{}{"group_id": f.groupID, "content": "burst " + itoa(int64(i))},
		})
	}

	var got []string
	for _, fr := range receivedFrames(f.memberConn, 2*time.Second) {
		var env struct {
			Type string `json:"type"`
			Data struct {
				Content string `json:"content"`
			} `json:"data"`
		}
		if err := json.Unmarshal([]byte(fr), &env); err != nil {
			t.Fatalf("frame is not a single JSON event (a browser would drop it): %.120q", fr)
		}
		if env.Type == "chat" {
			got = append(got, env.Data.Content)
		}
	}
	if len(got) != n {
		t.Fatalf("member received %d/%d messages", len(got), n)
	}
	for i, content := range got {
		if want := "burst " + itoa(int64(i)); content != want {
			t.Fatalf("message %d = %q, want %q (out of order)", i, content, want)
		}
	}
}
