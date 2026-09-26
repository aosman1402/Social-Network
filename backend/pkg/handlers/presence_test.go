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

type presenceEvent struct {
	Type string `json:"type"`
	Data struct {
		UserID int64 `json:"user_id"`
		Online bool  `json:"online"`
	} `json:"data"`
}

// presenceEvents returns the presence events among conn's frames within wait.
func presenceEvents(conn *gorilla.Conn, wait time.Duration) []presenceEvent {
	var out []presenceEvent
	for _, f := range receivedFrames(conn, wait) {
		var e presenceEvent
		if json.Unmarshal([]byte(f), &e) == nil && e.Type == "presence" {
			out = append(out, e)
		}
	}
	return out
}

// TestPresenceGoesToContactsOnly - Niko's contact sees him go offline when
// his last tab closes and online when he's back; a stranger sees nothing.
func TestPresenceGoesToContactsOnly(t *testing.T) {
	env := newWSTestEnvWith(t, EnablePresence)
	nikoID, nikoConn := env.connect("niko@example.com")
	femailID, femailConn := env.connect("femail2@example.com")
	_, strangerConn := env.connect("stranger@example.com")
	insertAcceptedFollow(t, env.wsh.DB, nikoID, femailID)

	nikoConn.Close() // Niko's only tab closes
	got := presenceEvents(femailConn, 500*time.Millisecond)
	if len(got) != 1 || got[0].Data.UserID != nikoID || got[0].Data.Online {
		t.Fatalf("contact got %+v, want one 'Niko offline' event", got)
	}
	if env.wsh.Hub.IsUserOnline(nikoID) {
		t.Error("hub still reports Niko online")
	}
	if s := presenceEvents(strangerConn, 200*time.Millisecond); len(s) != 0 {
		t.Errorf("stranger got presence events: %+v", s)
	}
}

func TestPresenceOnlineWhenBack(t *testing.T) {
	env := newWSTestEnvWith(t, EnablePresence)
	nikoID, nikoConn := env.connect("niko@example.com")
	femailID, femailConn := env.connect("femail2@example.com")
	insertAcceptedFollow(t, env.wsh.DB, femailID, nikoID)

	nikoConn.Close()
	deadline := time.Now().Add(2 * time.Second)
	for env.wsh.Hub.IsUserOnline(nikoID) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	second := dialTestWS(t, env.server.URL, "tok-niko@example.com") // back online
	defer second.Close()

	got := presenceEvents(femailConn, 600*time.Millisecond)
	if len(got) != 2 || got[0].Data.Online || !got[1].Data.Online {
		t.Fatalf("contact got %+v, want offline then online", got)
	}
}

func TestGetPresence(t *testing.T) {
	env := newWSTestEnvWith(t, EnablePresence)
	nikoID, _ := env.connect("niko@example.com")
	femailID := registerTestUser(t, env.wsh.DB, "femail2@example.com") // not connected
	strangerID := registerTestUser(t, env.wsh.DB, "stranger@example.com")
	insertAcceptedFollow(t, env.wsh.DB, femailID, nikoID)

	ask := func(viewer, target int64) (int, bool) {
		w := httptest.NewRecorder()
		env.wsh.GetPresence(w, withUserID(httptest.NewRequest(http.MethodGet, "/api/user/presence?id="+itoa(target), nil), viewer))
		var body struct {
			Online bool `json:"online"`
		}
		json.Unmarshal(w.Body.Bytes(), &body)
		return w.Code, body.Online
	}
	if code, online := ask(femailID, nikoID); code != http.StatusOK || !online {
		t.Errorf("contact asking about connected Niko: %d online=%v, want 200 true", code, online)
	}
	if code, online := ask(nikoID, femailID); code != http.StatusOK || online {
		t.Errorf("Niko asking about offline contact: %d online=%v, want 200 false", code, online)
	}
	if code, _ := ask(strangerID, nikoID); code != http.StatusForbidden {
		t.Errorf("stranger asking: status %d, want 403", code)
	}
}

// TestLogoutClosesThatSessionsConnections - after logout, no tab of that
// session keeps a live connection (it would keep receiving the user's
// messages), and contacts see the user go offline.
func TestLogoutClosesThatSessionsConnections(t *testing.T) {
	env := newWSTestEnvWith(t, EnablePresence)
	nikoID, tab1 := env.connect("niko@example.com")
	tab2 := dialTestWS(t, env.server.URL, "tok-niko@example.com") // second tab, same session
	defer tab2.Close()
	femailID, femailConn := env.connect("femail2@example.com")
	insertAcceptedFollow(t, env.wsh.DB, nikoID, femailID)

	req := httptest.NewRequest(http.MethodPost, "/api/logout", nil)
	req.AddCookie(&http.Cookie{Name: "session_token", Value: "tok-niko@example.com"})
	w := httptest.NewRecorder()
	(&AuthHandler{DB: env.wsh.DB, Hub: env.wsh.Hub}).Logout(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("logout: status %d", w.Code)
	}

	for name, conn := range map[string]*gorilla.Conn{"tab 1": tab1, "tab 2": tab2} {
		conn.SetReadDeadline(time.Now().Add(time.Second))
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				if strings.Contains(err.Error(), "timeout") {
					t.Errorf("%s is still connected after logout", name)
				}
				break
			}
		}
	}
	if env.wsh.Hub.IsUserOnline(nikoID) {
		t.Error("hub still reports Niko online after logout")
	}
	got := presenceEvents(femailConn, 500*time.Millisecond)
	if len(got) != 1 || got[0].Data.Online {
		t.Errorf("contact got %+v, want one 'Niko offline' event", got)
	}
}

// TestChatContactsAndOnlineInChatList - the chat page's "start a chat"
// list is exactly the people with a follow link (with live status), and
// the conversation list reports each person's status too.
func TestChatContactsAndOnlineInChatList(t *testing.T) {
	env := newWSTestEnvWith(t, EnablePresence)
	meID, _ := env.connect("me@example.com")
	onlineID, _ := env.connect("online@example.com")
	offlineID := registerTestUser(t, env.wsh.DB, "offline@example.com")
	strangerID, _ := env.connect("stranger@example.com")
	insertAcceptedFollow(t, env.wsh.DB, meID, onlineID)
	insertAcceptedFollow(t, env.wsh.DB, offlineID, meID)

	w := httptest.NewRecorder()
	env.wsh.GetChatContacts(w, withUserID(httptest.NewRequest(http.MethodGet, "/api/chat/contacts", nil), meID))
	var contacts []struct {
		User struct {
			ID int64 `json:"id"`
		} `json:"user"`
		Online bool `json:"online"`
	}
	json.Unmarshal(w.Body.Bytes(), &contacts)
	got := map[int64]bool{}
	for _, c := range contacts {
		got[c.User.ID] = c.Online
	}
	if len(got) != 2 || got[onlineID] != true || got[offlineID] != false {
		t.Errorf("contacts = %v, want online=true and offline=false only", got)
	}
	if _, ok := got[strangerID]; ok {
		t.Error("a stranger was listed as a contact")
	}

	if _, err := env.wsh.DB.Exec(
		"INSERT INTO messages (sender_id, recipient_id, content) VALUES (?, ?, 'hi')", onlineID, meID,
	); err != nil {
		t.Fatal(err)
	}
	w = httptest.NewRecorder()
	env.wsh.GetChats(w, withUserID(httptest.NewRequest(http.MethodGet, "/api/chats", nil), meID))
	var chats []struct {
		OtherUserID int64 `json:"other_user_id"`
		Online      bool  `json:"online"`
	}
	json.Unmarshal(w.Body.Bytes(), &chats)
	if len(chats) != 1 || chats[0].OtherUserID != onlineID || !chats[0].Online {
		t.Errorf("chats = %+v, want the online contact marked online", chats)
	}
}
