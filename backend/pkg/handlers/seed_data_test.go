package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Migration 000022 fills a fresh database with demo data. These tests check
// it's usable through the real handlers (not just present), that its down
// migration removes it completely, and that it's safe on a database that
// already has real users.

const demoPassword = "Demo-Pass-2026"

var demoEmails = []string{"alice@demo.test", "bob@demo.test", "carol@demo.test", "dave@demo.test", "eve@demo.test"}

func demoID(t *testing.T, db *sql.DB, email string) int64 {
	t.Helper()
	var id int64
	if err := db.QueryRow("SELECT id FROM users WHERE email = ?", email).Scan(&id); err != nil {
		t.Fatalf("demo user %s missing: %v", email, err)
	}
	return id
}

// getAs runs a GET handler as userID and decodes the JSON response.
func getAs(t *testing.T, h http.HandlerFunc, userID int64, target string, out interface{}) int {
	t.Helper()
	w := httptest.NewRecorder()
	h(w, withUserID(httptest.NewRequest(http.MethodGet, target, nil), userID))
	if out != nil && w.Code == http.StatusOK {
		if err := json.Unmarshal(w.Body.Bytes(), out); err != nil {
			t.Fatalf("GET %s: bad JSON %s", target, w.Body.String())
		}
	}
	return w.Code
}

func TestSeedDemoAccountsCanLogIn(t *testing.T) {
	db := newSeededTestDB(t)
	ah := &AuthHandler{DB: db}
	for _, email := range demoEmails {
		w := doJSON(t, ah.Login, http.MethodPost, "/api/login", map[string]string{"email": email, "password": demoPassword}, nil)
		if w.Code != http.StatusOK {
			t.Errorf("login as %s: status = %d, body = %s", email, w.Code, w.Body.String())
		}
	}
	if !isValidPassword(demoPassword) {
		t.Error("the documented demo password doesn't meet the site's own password policy")
	}
	var public bool
	db.QueryRow("SELECT is_public FROM users WHERE email = 'carol@demo.test'").Scan(&public)
	if public {
		t.Error("Carol should be the private demo profile")
	}
}

func TestSeedPostPrivacyWorks(t *testing.T) {
	db := newSeededTestDB(t)
	ph := &PostHandler{DB: db}
	bob, dave := demoID(t, db, "bob@demo.test"), demoID(t, db, "dave@demo.test")
	carol := demoID(t, db, "carol@demo.test")

	feed := func(userID int64) string {
		var posts []struct {
			Content string `json:"content"`
		}
		if code := getAs(t, ph.GetPosts, userID, "/api/posts", &posts); code != http.StatusOK {
			t.Fatalf("GetPosts: status %d", code)
		}
		var all []string
		for _, p := range posts {
			all = append(all, p.Content)
		}
		return strings.Join(all, "\n")
	}

	bobFeed, daveFeed, carolFeed := feed(bob), feed(dave), feed(carol)
	if !strings.Contains(bobFeed, "Just for you Bob") {
		t.Error("Bob doesn't see the private post Alice shared with him")
	}
	if strings.Contains(daveFeed, "Just for you Bob") {
		t.Error("Dave sees a private post that wasn't shared with him")
	}
	if !strings.Contains(daveFeed, "Followers only: planning a surprise") {
		t.Error("Dave (a follower) doesn't see Alice's almost-private post")
	}
	if strings.Contains(carolFeed, "Followers only: planning a surprise") {
		t.Error("Carol (not a follower) sees Alice's almost-private post")
	}
	if !strings.Contains(carolFeed, "Sunrise from the ridge") {
		t.Error("public posts missing from the feed")
	}
}

func TestSeedGroupsEventsChatsAndNotifications(t *testing.T) {
	db := newSeededTestDB(t)
	gh := &GroupHandler{DB: db}
	wsh := &WebSocketHandler{DB: db}
	uh := &UserHandler{DB: db}
	alice, bob := demoID(t, db, "alice@demo.test"), demoID(t, db, "bob@demo.test")
	carol, eve := demoID(t, db, "carol@demo.test"), demoID(t, db, "eve@demo.test")

	var groupID int64
	db.QueryRow("SELECT id FROM groups WHERE title = 'Weekend Hikers'").Scan(&groupID)

	var detail struct {
		IsMember        bool `json:"is_member"`
		IsInvited       bool `json:"is_invited"`
		PendingRequests []struct {
			User struct {
				ID int64 `json:"id"`
			} `json:"user"`
		} `json:"pending_requests"`
	}
	getAs(t, gh.GetGroup, alice, "/api/group/detail?id="+itoa(groupID), &detail)
	if !detail.IsMember || len(detail.PendingRequests) != 1 || detail.PendingRequests[0].User.ID != eve {
		t.Errorf("Alice's group view: member=%v pending=%+v, want Eve's join request", detail.IsMember, detail.PendingRequests)
	}
	detail.IsInvited = false
	getAs(t, gh.GetGroup, carol, "/api/group/detail?id="+itoa(groupID), &detail)
	if !detail.IsInvited {
		t.Error("Carol should have a pending invitation to Weekend Hikers")
	}

	var events []listedEvent
	getAs(t, gh.GetEvents, bob, "/api/group/events?group_id="+itoa(groupID), &events)
	if len(events) != 1 || events[0].UserResponse != "going" || len(events[0].Responses) != 3 {
		t.Errorf("events for Bob = %+v, want the hike with his 'going' and 3 votes", events)
	}

	var groupChat []ChatMessage
	getAs(t, wsh.GetGroupChatHistory, bob, "/api/messages/group-history?group_id="+itoa(groupID), &groupChat)
	var dms []ChatMessage
	getAs(t, wsh.GetChatHistory, alice, "/api/messages/history?user_id="+itoa(bob), &dms)
	if len(groupChat) != 3 || len(dms) != 3 {
		t.Errorf("got %d group messages and %d DMs, want 3 and 3", len(groupChat), len(dms))
	}

	var requests []struct {
		Follower struct {
			ID int64 `json:"id"`
		} `json:"follower"`
	}
	getAs(t, uh.GetFollowRequests, carol, "/api/user/follow-requests", &requests)
	if len(requests) != 1 || requests[0].Follower.ID != eve {
		t.Errorf("Carol's follow requests = %+v, want Eve's", requests)
	}

	var notifications []struct {
		Type string `json:"type"`
	}
	getAs(t, wsh.GetNotifications, carol, "/api/notifications", &notifications)
	types := map[string]bool{}
	for _, n := range notifications {
		types[n.Type] = true
	}
	if !types["follow_request"] || !types["group_invitation"] {
		t.Errorf("Carol's notifications = %v, want follow_request and group_invitation", types)
	}
}

// TestSeedDownRemovesOnlyDemoData - the down migration deletes every demo
// row (via ON DELETE CASCADE) and nothing belonging to real users.
func TestSeedDownRemovesOnlyDemoData(t *testing.T) {
	db := newSeededTestDB(t)
	realID := registerTestUser(t, db, "real@example.com")
	alice := demoID(t, db, "alice@demo.test")
	insertAcceptedFollow(t, db, realID, alice)
	db.Exec("INSERT INTO posts (user_id, content, privacy_level) VALUES (?, 'my own post', 'public')", realID)

	down, err := os.ReadFile(filepath.Join("..", "db", "migrations", "sqlite", "000022_seed_demo_data.down.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(string(down)); err != nil {
		t.Fatalf("down migration: %v", err)
	}

	for _, q := range []string{
		"SELECT COUNT(*) FROM users WHERE email LIKE '%@demo.test'",
		"SELECT COUNT(*) FROM posts WHERE user_id <> " + itoa(realID),
		"SELECT COUNT(*) FROM comments", "SELECT COUNT(*) FROM post_likes", "SELECT COUNT(*) FROM post_allowed_users",
		"SELECT COUNT(*) FROM follows", "SELECT COUNT(*) FROM groups", "SELECT COUNT(*) FROM group_members",
		"SELECT COUNT(*) FROM events", "SELECT COUNT(*) FROM event_responses", "SELECT COUNT(*) FROM messages",
		"SELECT COUNT(*) FROM notifications",
	} {
		var n int
		db.QueryRow(q).Scan(&n)
		if n != 0 {
			t.Errorf("%s = %d after the down migration, want 0", q, n)
		}
	}
	var realLeft, realPosts int
	db.QueryRow("SELECT COUNT(*) FROM users WHERE id = ?", realID).Scan(&realLeft)
	db.QueryRow("SELECT COUNT(*) FROM posts WHERE user_id = ?", realID).Scan(&realPosts)
	if realLeft != 1 || realPosts != 1 {
		t.Errorf("the real user or their post was removed (user %d, posts %d)", realLeft, realPosts)
	}
}

// TestSeedSafeWithExistingUsers - applying the seed to a database whose
// real users already hold a demo nickname must not fail (a failed
// migration would stop the server starting); that demo user just gets no
// nickname, and everything else is still linked up.
func TestSeedSafeWithExistingUsers(t *testing.T) {
	db := newTestDB(t) // demo data removed
	if _, err := db.Exec(`INSERT INTO users (email, password_hash, first_name, last_name, date_of_birth, nickname)
                          VALUES ('someone@example.com', 'x', 'Real', 'Alice', '1990-01-01', 'AliceW')`); err != nil {
		t.Fatal(err)
	}

	up, err := os.ReadFile(filepath.Join("..", "db", "migrations", "sqlite", "000022_seed_demo_data.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(string(up)); err != nil {
		t.Fatalf("seed failed on a database with an existing 'AliceW': %v", err)
	}

	var nickname sql.NullString
	db.QueryRow("SELECT nickname FROM users WHERE email = 'alice@demo.test'").Scan(&nickname)
	if nickname.Valid {
		t.Errorf("demo Alice got nickname %q, want none (it was taken)", nickname.String)
	}
	var groups, events int
	db.QueryRow("SELECT COUNT(*) FROM groups").Scan(&groups)
	db.QueryRow("SELECT COUNT(*) FROM events").Scan(&events)
	if groups != 2 || events != 1 {
		t.Errorf("groups = %d, events = %d, want 2 and 1", groups, events)
	}
}
