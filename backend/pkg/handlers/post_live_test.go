package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"social-network/pkg/websocket"
)

// attachFakeClient registers a connection for userID straight into hub's
// maps (no real socket, no hub.Run loop) and returns its Send channel, so
// a test can read exactly which live events that user was pushed.
func attachFakeClient(hub *websocket.Hub, userID int64) chan []byte {
	room := "user_" + strconv.FormatInt(userID, 10)
	c := &websocket.Client{Hub: hub, Send: make(chan []byte, 16), UserID: userID, RoomID: room}
	hub.Clients[c] = true
	if hub.Rooms[room] == nil {
		hub.Rooms[room] = map[*websocket.Client]bool{}
	}
	hub.Rooms[room][c] = true
	return c.Send
}

// drainEventTypes returns the "type" of every event queued on ch.
func drainEventTypes(t *testing.T, ch chan []byte) []string {
	t.Helper()
	var types []string
	for {
		select {
		case msg := <-ch:
			var env struct {
				Type string `json:"type"`
			}
			if err := json.Unmarshal(msg, &env); err != nil {
				t.Fatalf("bad event JSON: %v", err)
			}
			types = append(types, env.Type)
		default:
			return types
		}
	}
}

func hasType(types []string, want string) bool {
	for _, ty := range types {
		if ty == want {
			return true
		}
	}
	return false
}

// A new post, comment, like and delete must reach every online user who
// can see the post - live, with no refresh - and never anyone who can't.
func TestPostEventsBroadcastOnlyToViewers(t *testing.T) {
	db := newTestDB(t)
	hub := websocket.NewHub()
	ph := &PostHandler{DB: db, Hub: hub}

	author := registerTestUser(t, db, "live-author@example.com")
	other := registerTestUser(t, db, "live-other@example.com")
	authorCh := attachFakeClient(hub, author)
	otherCh := attachFakeClient(hub, other)

	createPost := func(privacy string) int64 {
		w := httptest.NewRecorder()
		ph.CreatePost(w, withUserID(newMultipartPostRequest(t, map[string]string{
			"content": "live " + privacy, "privacy_level": privacy,
		}), author))
		if w.Code != http.StatusCreated {
			t.Fatalf("CreatePost: status = %d, body = %s", w.Code, w.Body.String())
		}
		var resp struct {
			PostID int64 `json:"post_id"`
		}
		json.Unmarshal(w.Body.Bytes(), &resp)
		return resp.PostID
	}

	// Public post: both see post_created, then comment/like/delete.
	pub := createPost("public")
	if got := drainEventTypes(t, otherCh); !hasType(got, "post_created") {
		t.Errorf("other user events after public post = %v, want post_created", got)
	}
	drainEventTypes(t, authorCh)

	req := newMultipartPostRequest(t, map[string]string{"content": "nice", "post_id": strconv.FormatInt(pub, 10)})
	ph.CreateComment(httptest.NewRecorder(), withUserID(req, other))
	ph.LikePost(httptest.NewRecorder(), withUserID(httptest.NewRequest(http.MethodPost, "/api/post/like?id="+strconv.FormatInt(pub, 10), nil), other))
	ph.DeletePost(httptest.NewRecorder(), withUserID(httptest.NewRequest(http.MethodDelete, "/api/post/delete?id="+strconv.FormatInt(pub, 10), nil), author))

	for name, ch := range map[string]chan []byte{"author": authorCh, "other": otherCh} {
		got := drainEventTypes(t, ch)
		for _, want := range []string{"comment_created", "post_liked", "post_deleted"} {
			if !hasType(got, want) {
				t.Errorf("%s events = %v, missing %s", name, got, want)
			}
		}
	}

	// Almost-private post, other doesn't follow the author: must not leak.
	createPost("almost_private")
	if got := drainEventTypes(t, otherCh); len(got) != 0 {
		t.Errorf("non-follower got events for an almost_private post: %v", got)
	}
	if got := drainEventTypes(t, authorCh); !hasType(got, "post_created") {
		t.Errorf("author's own tabs events = %v, want post_created", got)
	}
}
