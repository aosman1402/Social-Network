package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// eventFixture is a group owned by ownerID with one more active member, one
// invited user and one pending join request.
type eventFixture struct {
	gh                                      *GroupHandler
	groupID                                 int64
	ownerID, memberID, invitedID, pendingID int64
}

func newEventFixture(t *testing.T) *eventFixture {
	t.Helper()
	db := newTestDB(t)
	f := &eventFixture{gh: &GroupHandler{DB: db}}
	f.ownerID = registerTestUser(t, db, "owner@example.com")
	f.memberID = registerTestUser(t, db, "member@example.com")
	f.invitedID = registerTestUser(t, db, "invited@example.com")
	f.pendingID = registerTestUser(t, db, "pending@example.com")
	f.groupID = createTestGroup(t, f.gh, f.ownerID)

	for userID, status := range map[int64]string{
		f.memberID:  "active",
		f.invitedID: "invited",
		f.pendingID: "pending",
	} {
		if _, err := db.Exec(
			`INSERT INTO group_members (group_id, user_id, status) VALUES (?, ?, ?)`,
			f.groupID, userID, status,
		); err != nil {
			t.Fatalf("add %s member: %v", status, err)
		}
	}
	return f
}

func (f *eventFixture) createEvent(t *testing.T, userID int64, eventTime time.Time) *httptest.ResponseRecorder {
	t.Helper()
	return callAs(t, f.gh.CreateEvent, userID, "/api/group/event/create", map[string]interface{}{
		"group_id":    f.groupID,
		"title":       "Picnic",
		"description": "Bring snacks",
		"event_time":  eventTime.Format(time.RFC3339),
	})
}

type listedEvent struct {
	ID           int64  `json:"id"`
	Title        string `json:"title"`
	UserResponse string `json:"user_response"`
	Responses    []struct {
		Response string `json:"response"`
		User     struct {
			ID int64 `json:"id"`
		} `json:"user"`
	} `json:"responses"`
}

func (f *eventFixture) listEvents(t *testing.T, userID int64) []listedEvent {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/group/events?group_id="+itoa(f.groupID), nil)
	f.gh.GetEvents(w, withUserID(req, userID))
	if w.Code != http.StatusOK {
		t.Fatalf("GetEvents: status = %d, body = %s", w.Code, w.Body.String())
	}
	var events []listedEvent
	if err := json.Unmarshal(w.Body.Bytes(), &events); err != nil {
		t.Fatalf("GetEvents: bad body %s", w.Body.String())
	}
	return events
}

// TestEventCreatedNotifiesOnlyOtherActiveMembers - the audit check: the
// other member is notified; the creator, invited users and pending
// requesters are not.
func TestEventCreatedNotifiesOnlyOtherActiveMembers(t *testing.T) {
	f := newEventFixture(t)

	if w := f.createEvent(t, f.ownerID, time.Now().Add(24*time.Hour)); w.Code != http.StatusOK {
		t.Fatalf("CreateEvent: status = %d, body = %s", w.Code, w.Body.String())
	}

	if n := countNotifications(t, f.gh.DB, f.memberID, "event_created", true); n != 1 {
		t.Errorf("member got %d event_created notifications, want 1", n)
	}
	var relatedID int64
	f.gh.DB.QueryRow(
		"SELECT related_id FROM notifications WHERE user_id = ? AND type = 'event_created'",
		f.memberID,
	).Scan(&relatedID)
	if relatedID != f.groupID {
		t.Errorf("notification related_id = %d, want group id %d", relatedID, f.groupID)
	}

	for name, id := range map[string]int64{"creator": f.ownerID, "invited": f.invitedID, "pending": f.pendingID} {
		if n := countNotifications(t, f.gh.DB, id, "event_created", false); n != 0 {
			t.Errorf("%s got %d event_created notifications, want 0", name, n)
		}
	}
}

func TestCreateEventValidation(t *testing.T) {
	f := newEventFixture(t)

	if w := f.createEvent(t, f.invitedID, time.Now().Add(time.Hour)); w.Code != http.StatusForbidden {
		t.Errorf("invited (non-active) user: status = %d, want 403", w.Code)
	}
	if w := f.createEvent(t, f.ownerID, time.Now().Add(-time.Hour)); w.Code != http.StatusBadRequest {
		t.Errorf("past event: status = %d, want 400", w.Code)
	}
	w := callAs(t, f.gh.CreateEvent, f.ownerID, "/api/group/event/create", map[string]interface{}{
		"group_id":   f.groupID,
		"title":      "   ",
		"event_time": time.Now().Add(time.Hour).Format(time.RFC3339),
	})
	if w.Code != http.StatusBadRequest {
		t.Errorf("blank title: status = %d, want 400", w.Code)
	}
}

// TestRespondToEvent - needs the event_responses table (migration 000020);
// before it existed every response failed with a 500.
func TestRespondToEvent(t *testing.T) {
	f := newEventFixture(t)
	f.createEvent(t, f.ownerID, time.Now().Add(24*time.Hour))
	events := f.listEvents(t, f.memberID)
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
	eventID := events[0].ID

	respond := func(userID int64, response string) int {
		return callAs(t, f.gh.RespondToEvent, userID, "/api/group/event/respond", map[string]interface{}{
			"event_id": eventID,
			"response": response,
		}).Code
	}

	if code := respond(f.memberID, "going"); code != http.StatusOK {
		t.Fatalf("respond going: status = %d", code)
	}
	if code := respond(f.memberID, "not_going"); code != http.StatusOK {
		t.Fatalf("change to not_going: status = %d", code)
	}
	if code := respond(f.memberID, "maybe"); code != http.StatusBadRequest {
		t.Errorf("invalid response: status = %d, want 400", code)
	}
	if code := respond(f.pendingID, "going"); code != http.StatusForbidden {
		t.Errorf("non-member response: status = %d, want 403", code)
	}

	got := f.listEvents(t, f.memberID)[0]
	if got.UserResponse != "not_going" {
		t.Errorf("user_response = %q, want not_going", got.UserResponse)
	}
	if len(got.Responses) != 1 || got.Responses[0].Response != "not_going" || got.Responses[0].User.ID != f.memberID {
		t.Errorf("responses = %+v, want one not_going from the member", got.Responses)
	}
	if owner := f.listEvents(t, f.ownerID)[0]; owner.UserResponse != "" {
		t.Errorf("owner's user_response = %q, want empty", owner.UserResponse)
	}
}

// TestEventsSortedByRealTimeAcrossTimezones - event_time was compared as
// text, so 10:00+05:00 (05:00 UTC) sorted after 08:00+00:00.
func TestEventsSortedByRealTimeAcrossTimezones(t *testing.T) {
	f := newEventFixture(t)
	day := time.Now().AddDate(0, 0, 5).Format("2006-01-02")

	for _, ev := range []struct{ title, when string }{
		{"later (08:00 UTC)", day + "T08:00:00+00:00"},
		{"earlier (05:00 UTC)", day + "T10:00:00+05:00"},
	} {
		w := callAs(t, f.gh.CreateEvent, f.ownerID, "/api/group/event/create", map[string]interface{}{
			"group_id": f.groupID, "title": ev.title, "event_time": ev.when,
		})
		if w.Code != http.StatusOK {
			t.Fatalf("CreateEvent %s: status = %d, body = %s", ev.title, w.Code, w.Body.String())
		}
	}

	events := f.listEvents(t, f.memberID)
	if len(events) != 2 || events[0].Title != "earlier (05:00 UTC)" {
		t.Fatalf("order = %+v, want the 05:00 UTC event first", events)
	}
}

// TestDeleteEventOnlyByCreator - the event's creator can cancel it (its
// votes go with it); other members, even the group creator, can't.
func TestDeleteEventOnlyByCreator(t *testing.T) {
	f := newEventFixture(t)
	// The member creates the event, so the group creator is "someone else".
	if w := f.createEvent(t, f.memberID, time.Now().Add(24*time.Hour)); w.Code != http.StatusOK {
		t.Fatalf("CreateEvent: status = %d, body = %s", w.Code, w.Body.String())
	}
	eventID := f.listEvents(t, f.memberID)[0].ID
	callAs(t, f.gh.RespondToEvent, f.ownerID, "/api/group/event/respond", map[string]interface{}{
		"event_id": eventID, "response": "going",
	})

	del := func(userID int64, id string) int {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodDelete, "/api/group/event/delete?id="+id, nil)
		f.gh.DeleteEvent(w, withUserID(req, userID))
		return w.Code
	}

	if code := del(f.ownerID, itoa(eventID)); code != http.StatusForbidden {
		t.Errorf("group creator (not the event's creator): status = %d, want 403", code)
	}
	if code := del(f.memberID, "999999"); code != http.StatusNotFound {
		t.Errorf("unknown event: status = %d, want 404", code)
	}
	if code := del(f.memberID, "abc"); code != http.StatusBadRequest {
		t.Errorf("bad id: status = %d, want 400", code)
	}
	if code := del(f.memberID, itoa(eventID)); code != http.StatusOK {
		t.Fatalf("event creator: status = %d, want 200", code)
	}

	if events := f.listEvents(t, f.memberID); len(events) != 0 {
		t.Errorf("event still listed after cancelling: %+v", events)
	}
	var votes int
	f.gh.DB.QueryRow("SELECT COUNT(*) FROM event_responses WHERE event_id = ?", eventID).Scan(&votes)
	if votes != 0 {
		t.Errorf("%d votes left behind for the cancelled event", votes)
	}
}
