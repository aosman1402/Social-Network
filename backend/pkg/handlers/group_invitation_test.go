package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// insertAcceptedFollow makes followerID an accepted follower of followingID.
func insertAcceptedFollow(t *testing.T, db *sql.DB, followerID, followingID int64) {
	t.Helper()
	now := time.Now()
	if _, err := db.Exec(
		`INSERT INTO follows (follower_id, following_id, status, created_at, updated_at) VALUES (?, ?, 'accepted', ?, ?)`,
		followerID, followingID, now, now,
	); err != nil {
		t.Fatalf("insert follow: %v", err)
	}
}

// callAs runs handler as userID with a JSON body and returns the recorder.
func callAs(t *testing.T, handler http.HandlerFunc, userID int64, target string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	handler(w, withUserID(jsonRequest(t, http.MethodPost, target, body), userID))
	return w
}

// createTestGroup creates a group owned by creatorID and returns its id.
func createTestGroup(t *testing.T, gh *GroupHandler, creatorID int64) int64 {
	t.Helper()
	w := callAs(t, gh.CreateGroup, creatorID, "/api/group/create", map[string]interface{}{
		"title":       "Test Group",
		"description": "test",
	})
	if w.Code != http.StatusOK && w.Code != http.StatusCreated {
		t.Fatalf("CreateGroup: status = %d, body = %s", w.Code, w.Body.String())
	}
	var resp struct {
		GroupID int64 `json:"group_id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil || resp.GroupID == 0 {
		t.Fatalf("CreateGroup: bad response %s", w.Body.String())
	}
	return resp.GroupID
}

// membershipStatus returns the invitee's group_members status, or "" if
// they have no row for the group.
func membershipStatus(t *testing.T, db *sql.DB, groupID, userID int64) string {
	t.Helper()
	var status string
	err := db.QueryRow(
		"SELECT status FROM group_members WHERE group_id = ? AND user_id = ?",
		groupID, userID,
	).Scan(&status)
	if err == sql.ErrNoRows {
		return ""
	}
	if err != nil {
		t.Fatalf("failed to query membership: %v", err)
	}
	return status
}

// TestGroupInvitationAccept - the invitee accepts by group_id, becomes an
// active member, and the inviter is told.
func TestGroupInvitationAccept(t *testing.T) {
	db := newTestDB(t)
	gh := &GroupHandler{DB: db}

	inviterID := registerTestUser(t, db, "inviter@example.com")
	inviteeID := registerTestUser(t, db, "invitee@example.com")
	insertAcceptedFollow(t, db, inviteeID, inviterID)
	groupID := createTestGroup(t, gh, inviterID)

	if w := callAs(t, gh.InviteUserToGroup, inviterID, "/api/group/invite", map[string]interface{}{
		"user_id": inviteeID, "group_id": groupID,
	}); w.Code != http.StatusOK {
		t.Fatalf("InviteUserToGroup: status = %d, body = %s", w.Code, w.Body.String())
	}

	w := callAs(t, gh.HandleGroupInvitation, inviteeID, "/api/group/invitation", map[string]interface{}{
		"group_id": groupID, "action": "accept",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("accept: status = %d, body = %s", w.Code, w.Body.String())
	}
	if got := membershipStatus(t, db, groupID, inviteeID); got != "active" {
		t.Errorf("status after accept = %q, want active", got)
	}
	if got := queryNotificationType(t, db, inviterID, "group_invitation_accepted"); got == "" {
		t.Error("inviter did not get a group_invitation_accepted notification")
	}

	var unread int
	db.QueryRow(
		"SELECT COUNT(*) FROM notifications WHERE user_id = ? AND type = 'group_invitation' AND is_read = 0",
		inviteeID,
	).Scan(&unread)
	if unread != 0 {
		t.Errorf("invitation notification still unread after accepting")
	}
}

// TestGroupInvitationDeclineDoesNotLockOut guards against a real bug: a
// declined invitation left a 'declined' row behind, and since the invite
// and join checks reject anyone with an existing row, that user could
// never be re-invited or request to join again.
func TestGroupInvitationDeclineDoesNotLockOut(t *testing.T) {
	db := newTestDB(t)
	gh := &GroupHandler{DB: db}

	inviterID := registerTestUser(t, db, "inviter@example.com")
	inviteeID := registerTestUser(t, db, "invitee@example.com")
	insertAcceptedFollow(t, db, inviteeID, inviterID)
	groupID := createTestGroup(t, gh, inviterID)

	invite := func() *httptest.ResponseRecorder {
		return callAs(t, gh.InviteUserToGroup, inviterID, "/api/group/invite", map[string]interface{}{
			"user_id": inviteeID, "group_id": groupID,
		})
	}

	if w := invite(); w.Code != http.StatusOK {
		t.Fatalf("InviteUserToGroup: status = %d, body = %s", w.Code, w.Body.String())
	}

	w := callAs(t, gh.HandleGroupInvitation, inviteeID, "/api/group/invitation", map[string]interface{}{
		"group_id": groupID, "action": "decline",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("decline: status = %d, body = %s", w.Code, w.Body.String())
	}
	if got := membershipStatus(t, db, groupID, inviteeID); got != "" {
		t.Errorf("status after decline = %q, want no row", got)
	}
	if got := queryNotificationType(t, db, inviterID, "group_invitation_declined"); got == "" {
		t.Error("inviter did not get a group_invitation_declined notification")
	}

	if w := invite(); w.Code != http.StatusOK {
		t.Errorf("re-invite after decline: status = %d, body = %s", w.Code, w.Body.String())
	}
}

// TestGroupInvitationRejectsUnknownAction - anything other than
// accept/decline is a bad request, not a silent decline.
func TestGroupInvitationRejectsUnknownAction(t *testing.T) {
	db := newTestDB(t)
	gh := &GroupHandler{DB: db}

	inviterID := registerTestUser(t, db, "inviter@example.com")
	inviteeID := registerTestUser(t, db, "invitee@example.com")
	insertAcceptedFollow(t, db, inviteeID, inviterID)
	groupID := createTestGroup(t, gh, inviterID)

	callAs(t, gh.InviteUserToGroup, inviterID, "/api/group/invite", map[string]interface{}{
		"user_id": inviteeID, "group_id": groupID,
	})

	w := callAs(t, gh.HandleGroupInvitation, inviteeID, "/api/group/invitation", map[string]interface{}{
		"group_id": groupID, "action": "banana",
	})
	if w.Code != http.StatusBadRequest {
		t.Errorf("unknown action: status = %d, want 400", w.Code)
	}
	if got := membershipStatus(t, db, groupID, inviteeID); got != "invited" {
		t.Errorf("status after bad action = %q, want invited (unchanged)", got)
	}
}

// TestGroupJoinRequestDeclineDoesNotLockOut - same lock-out bug on the join
// request side: a declined request must not stop the user asking again.
func TestGroupJoinRequestDeclineDoesNotLockOut(t *testing.T) {
	db := newTestDB(t)
	gh := &GroupHandler{DB: db}

	creatorID := registerTestUser(t, db, "creator@example.com")
	requesterID := registerTestUser(t, db, "requester@example.com")
	groupID := createTestGroup(t, gh, creatorID)

	join := func() *httptest.ResponseRecorder {
		return callAs(t, gh.RequestToJoinGroup, requesterID, "/api/group/join", map[string]interface{}{
			"group_id": groupID,
		})
	}

	if w := join(); w.Code != http.StatusOK {
		t.Fatalf("RequestToJoinGroup: status = %d, body = %s", w.Code, w.Body.String())
	}

	var requestID int64
	db.QueryRow(
		"SELECT id FROM group_members WHERE group_id = ? AND user_id = ?",
		groupID, requesterID,
	).Scan(&requestID)

	w := callAs(t, gh.HandleGroupRequest, creatorID, "/api/group/request", map[string]interface{}{
		"request_id": requestID, "action": "decline",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("decline request: status = %d, body = %s", w.Code, w.Body.String())
	}

	if w := join(); w.Code != http.StatusOK {
		t.Errorf("join again after decline: status = %d, body = %s", w.Code, w.Body.String())
	}
}

// TestGroupInviteRequiresFollowRelationship - the invite UI only lists
// followers, but the API used to accept any user_id, letting two users with
// no connection at all end up in a group together.
func TestGroupInviteRequiresFollowRelationship(t *testing.T) {
	db := newTestDB(t)
	gh := &GroupHandler{DB: db}

	inviterID := registerTestUser(t, db, "inviter@example.com")
	strangerID := registerTestUser(t, db, "stranger@example.com")
	groupID := createTestGroup(t, gh, inviterID)

	w := callAs(t, gh.InviteUserToGroup, inviterID, "/api/group/invite", map[string]interface{}{
		"user_id": strangerID, "group_id": groupID,
	})
	if w.Code != http.StatusForbidden {
		t.Errorf("invite stranger: status = %d, want 403", w.Code)
	}
	if got := membershipStatus(t, db, groupID, strangerID); got != "" {
		t.Errorf("stranger membership = %q, want no row", got)
	}

	// Either direction of follow counts: the inviter following them is enough.
	insertAcceptedFollow(t, db, inviterID, strangerID)
	w = callAs(t, gh.InviteUserToGroup, inviterID, "/api/group/invite", map[string]interface{}{
		"user_id": strangerID, "group_id": groupID,
	})
	if w.Code != http.StatusOK {
		t.Errorf("invite after follow: status = %d, body = %s", w.Code, w.Body.String())
	}
}

// countNotifications counts userID's notifications of notifType, optionally
// only unread ones.
func countNotifications(t *testing.T, db *sql.DB, userID int64, notifType string, unreadOnly bool) int {
	t.Helper()
	query := "SELECT COUNT(*) FROM notifications WHERE user_id = ? AND type = ?"
	if unreadOnly {
		query += " AND is_read = 0"
	}
	var n int
	if err := db.QueryRow(query, userID, notifType).Scan(&n); err != nil {
		t.Fatalf("count notifications: %v", err)
	}
	return n
}

// TestEveryJoinRequestNotifiesCreator guards against a real bug: only the
// first join request a group ever received notified the creator - the
// duplicate check matched any group_request for the group, not just the
// same requester's.
func TestEveryJoinRequestNotifiesCreator(t *testing.T) {
	db := newTestDB(t)
	gh := &GroupHandler{DB: db}

	creatorID := registerTestUser(t, db, "creator@example.com")
	firstID := registerTestUser(t, db, "first@example.com")
	secondID := registerTestUser(t, db, "second@example.com")
	groupID := createTestGroup(t, gh, creatorID)

	for _, requester := range []int64{firstID, secondID} {
		if w := callAs(t, gh.RequestToJoinGroup, requester, "/api/group/join", map[string]interface{}{
			"group_id": groupID,
		}); w.Code != http.StatusOK {
			t.Fatalf("RequestToJoinGroup: status = %d, body = %s", w.Code, w.Body.String())
		}
	}

	if got := countNotifications(t, db, creatorID, "group_request", false); got != 2 {
		t.Errorf("creator got %d group_request notifications, want 2", got)
	}
}

// TestAnsweringJoinRequestNotifiesAndClears - the requester hears back on
// accept *and* decline, and the creator's request notification is marked
// read once answered (only that requester's, not the others').
func TestAnsweringJoinRequestNotifiesAndClears(t *testing.T) {
	db := newTestDB(t)
	gh := &GroupHandler{DB: db}

	creatorID := registerTestUser(t, db, "creator@example.com")
	acceptedID := registerTestUser(t, db, "accepted@example.com")
	declinedID := registerTestUser(t, db, "declined@example.com")
	waitingID := registerTestUser(t, db, "waiting@example.com")
	groupID := createTestGroup(t, gh, creatorID)

	requestIDs := map[int64]int64{}
	for _, requester := range []int64{acceptedID, declinedID, waitingID} {
		callAs(t, gh.RequestToJoinGroup, requester, "/api/group/join", map[string]interface{}{"group_id": groupID})
		var id int64
		db.QueryRow("SELECT id FROM group_members WHERE group_id = ? AND user_id = ?", groupID, requester).Scan(&id)
		requestIDs[requester] = id
	}

	answer := func(requester int64, action string) {
		w := callAs(t, gh.HandleGroupRequest, creatorID, "/api/group/request", map[string]interface{}{
			"request_id": requestIDs[requester], "action": action,
		})
		if w.Code != http.StatusOK {
			t.Fatalf("%s: status = %d, body = %s", action, w.Code, w.Body.String())
		}
	}
	answer(acceptedID, "accept")
	answer(declinedID, "decline")

	if queryNotificationType(t, db, acceptedID, "group_request_accepted") == "" {
		t.Error("accepted requester was not notified")
	}
	if queryNotificationType(t, db, declinedID, "group_request_declined") == "" {
		t.Error("declined requester was not notified")
	}
	if got := countNotifications(t, db, creatorID, "group_request", true); got != 1 {
		t.Errorf("creator has %d unread group_request notifications, want 1 (only the unanswered one)", got)
	}
}

// TestMemberWhoIsNotCreatorCanInvite - the audit check "Can a user make
// group invitations, after being part of the group (being the user
// different from the creator of the group)?". The invite goes out, the
// invitee is notified, and accepting credits the member who invited them,
// not the creator. Someone who isn't an active member can't invite.
func TestMemberWhoIsNotCreatorCanInvite(t *testing.T) {
	db := newTestDB(t)
	gh := &GroupHandler{DB: db}

	creatorID := registerTestUser(t, db, "creator@example.com")
	memberID := registerTestUser(t, db, "member@example.com")
	friendID := registerTestUser(t, db, "friend@example.com")
	outsiderID := registerTestUser(t, db, "outsider@example.com")
	groupID := createTestGroup(t, gh, creatorID)
	if _, err := db.Exec(
		`INSERT INTO group_members (group_id, user_id, status) VALUES (?, ?, 'active')`,
		groupID, memberID,
	); err != nil {
		t.Fatalf("add member: %v", err)
	}
	// The member follows the friend - the creator has no link to them.
	insertAcceptedFollow(t, db, memberID, friendID)
	insertAcceptedFollow(t, db, outsiderID, friendID)

	if w := callAs(t, gh.InviteUserToGroup, outsiderID, "/api/group/invite", map[string]interface{}{
		"user_id": friendID, "group_id": groupID,
	}); w.Code != http.StatusForbidden {
		t.Errorf("non-member invite: status = %d, want 403", w.Code)
	}

	if w := callAs(t, gh.InviteUserToGroup, memberID, "/api/group/invite", map[string]interface{}{
		"user_id": friendID, "group_id": groupID,
	}); w.Code != http.StatusOK {
		t.Fatalf("member invite: status = %d, body = %s", w.Code, w.Body.String())
	}
	if got := membershipStatus(t, db, groupID, friendID); got != "invited" {
		t.Errorf("friend's status = %q, want invited", got)
	}
	if n := countNotifications(t, db, friendID, "group_invitation", true); n != 1 {
		t.Errorf("friend got %d group_invitation notifications, want 1", n)
	}

	if w := callAs(t, gh.HandleGroupInvitation, friendID, "/api/group/invitation", map[string]interface{}{
		"group_id": groupID, "action": "accept",
	}); w.Code != http.StatusOK {
		t.Fatalf("accept: status = %d, body = %s", w.Code, w.Body.String())
	}
	if got := membershipStatus(t, db, groupID, friendID); got != "active" {
		t.Errorf("friend's status after accept = %q, want active", got)
	}
	if got := queryNotificationType(t, db, memberID, "group_invitation_accepted"); got == "" {
		t.Error("the inviting member was not told the invitation was accepted")
	}
	if got := queryNotificationType(t, db, creatorID, "group_invitation_accepted"); got != "" {
		t.Error("the creator was notified about an invitation they didn't send")
	}
}

// TestGroupDetailListsInvitedMembers - members see who's already invited
// (so the invite list can leave them out), and the member count still only
// counts active members.
func TestGroupDetailListsInvitedMembers(t *testing.T) {
	db := newTestDB(t)
	gh := &GroupHandler{DB: db}
	creatorID := registerTestUser(t, db, "creator@example.com")
	inviteeID := registerTestUser(t, db, "invitee@example.com")
	insertAcceptedFollow(t, db, inviteeID, creatorID)
	groupID := createTestGroup(t, gh, creatorID)
	callAs(t, gh.InviteUserToGroup, creatorID, "/api/group/invite", map[string]interface{}{
		"user_id": inviteeID, "group_id": groupID,
	})

	w := httptest.NewRecorder()
	gh.GetGroup(w, withUserID(httptest.NewRequest(http.MethodGet, "/api/group/detail?id="+itoa(groupID), nil), creatorID))
	var detail struct {
		MemberCount int `json:"member_count"`
		Members     []struct {
			Status string `json:"status"`
			User   struct {
				ID int64 `json:"id"`
			} `json:"user"`
		} `json:"members"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &detail); err != nil {
		t.Fatalf("bad detail body %s", w.Body.String())
	}
	if detail.MemberCount != 1 {
		t.Errorf("member_count = %d, want 1 (the invitee isn't a member yet)", detail.MemberCount)
	}
	if len(detail.Members) != 2 || detail.Members[0].User.ID != creatorID ||
		detail.Members[1].User.ID != inviteeID || detail.Members[1].Status != "invited" {
		t.Errorf("members = %+v, want the creator (active) then the invitee (invited)", detail.Members)
	}
}
