package handlers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

// jsonRequest builds a request with a JSON body but no cookies/context -
// callers that need an authenticated userID wrap the result with
// withUserID (see post_integration_test.go), since the handlers under
// test here read the caller from request context, not a session cookie.
func jsonRequest(t *testing.T, method, target string, body interface{}) *http.Request {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("failed to marshal request body: %v", err)
	}
	r := httptest.NewRequest(method, target, bytes.NewReader(b))
	r.Header.Set("Content-Type", "application/json")
	return r
}

// queryNotificationType returns the type of the first notification found
// for userID matching wantType, or "" if none exists.
func queryNotificationType(t *testing.T, db *sql.DB, userID int64, wantType string) string {
	t.Helper()
	var got string
	err := db.QueryRow(
		"SELECT type FROM notifications WHERE user_id = ? AND type = ? LIMIT 1",
		userID, wantType,
	).Scan(&got)
	if err == sql.ErrNoRows {
		return ""
	}
	if err != nil {
		t.Fatalf("failed to query notifications: %v", err)
	}
	return got
}

// TestGoingPublicAutoAcceptsPendingFollows guards against a real bug: a
// follow request sent while the target account was private stayed at
// status = 'pending' forever, even after the target switched their
// profile to public - at which point canViewAccount already granted the
// requester full view access (public accounts are visible to everyone),
// so the requester silently had a follower's worth of access without
// ever being formally accepted, without appearing in followers_count or
// GetFollowers, and without either side getting a notification.
//
// The fix: switching private -> public bulk-accepts any dangling pending
// follows against that account (matching how a fresh follow against an
// already-public account is auto-accepted - see FollowUser), and notifies
// both the newly-accepted follower and the account owner.
func TestGoingPublicAutoAcceptsPendingFollows(t *testing.T) {
	db := newTestDB(t)
	user := &UserHandler{DB: db} // Hub is nil - fine in tests, see notifyUser's doc comment

	ownerID := registerTestUser(t, db, "owner@example.com")
	followerID := registerTestUser(t, db, "follower@example.com")

	updateOwnerVisibility := func(isPublic bool) {
		r := withUserID(jsonRequest(t, http.MethodPut, "/api/user/update", map[string]interface{}{
			"first_name": "Owner",
			"last_name":  "User",
			"about_me":   "",
			"is_public":  isPublic,
		}), ownerID)
		w := httptest.NewRecorder()
		user.UpdateProfile(w, r)
		if w.Code != http.StatusOK {
			t.Fatalf("UpdateProfile(is_public=%v): status = %d, body = %s", isPublic, w.Code, w.Body.String())
		}
	}

	// Owner goes private.
	updateOwnerVisibility(false)

	// Follower requests to follow the now-private owner - must land as
	// "pending", not auto-accepted.
	followReq := withUserID(jsonRequest(t, http.MethodPost, "/api/user/follow", map[string]interface{}{
		"user_id": ownerID,
	}), followerID)
	followW := httptest.NewRecorder()
	user.FollowUser(followW, followReq)
	var followResp struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(followW.Body.Bytes(), &followResp); err != nil {
		t.Fatalf("FollowUser: failed to parse response: %v", err)
	}
	if followResp.Status != "pending" {
		t.Fatalf("FollowUser against a private account: status = %q, want %q", followResp.Status, "pending")
	}

	var statusBefore string
	if err := db.QueryRow(
		"SELECT status FROM follows WHERE follower_id = ? AND following_id = ?",
		followerID, ownerID,
	).Scan(&statusBefore); err != nil {
		t.Fatalf("failed to read follows row: %v", err)
	}
	if statusBefore != "pending" {
		t.Fatalf("follows.status before going public = %q, want %q", statusBefore, "pending")
	}

	// Owner switches to public - this must NOT require the owner to
	// manually accept the request.
	updateOwnerVisibility(true)

	var statusAfter string
	if err := db.QueryRow(
		"SELECT status FROM follows WHERE follower_id = ? AND following_id = ?",
		followerID, ownerID,
	).Scan(&statusAfter); err != nil {
		t.Fatalf("failed to read follows row after going public: %v", err)
	}
	if statusAfter != "accepted" {
		t.Errorf("follows.status after going public = %q, want %q (pending request must be auto-accepted)", statusAfter, "accepted")
	}

	// The follower must be notified their request was accepted.
	if queryNotificationType(t, db, followerID, "follow_accepted") == "" {
		t.Error("follower did not receive a follow_accepted notification after the owner went public")
	}

	// The owner must be notified their profile switched to public.
	if queryNotificationType(t, db, ownerID, "profile_public") == "" {
		t.Error("owner did not receive a profile_public notification after switching to public")
	}

	// GetFollowers must now include the previously-pending follower.
	followersReq := withUserID(
		httptest.NewRequest(http.MethodGet, "/api/user/followers?id="+strconv.FormatInt(ownerID, 10), nil),
		ownerID,
	)
	followersW := httptest.NewRecorder()
	user.GetFollowers(followersW, followersReq)
	var followers []map[string]interface{}
	if err := json.Unmarshal(followersW.Body.Bytes(), &followers); err != nil {
		t.Fatalf("GetFollowers: failed to parse response: %v", err)
	}
	if len(followers) != 1 {
		t.Errorf("GetFollowers after going public: got %d followers, want 1", len(followers))
	}
}

// TestGoingPublicWithNoPendingFollowsStillNotifiesOwner covers the case
// where there's nothing to auto-accept - the owner should still be told
// their profile is now public, just without the "N requests approved"
// wording.
func TestGoingPublicWithNoPendingFollowsStillNotifiesOwner(t *testing.T) {
	db := newTestDB(t)
	user := &UserHandler{DB: db}
	ownerID := registerTestUser(t, db, "solo-owner@example.com")

	for _, isPublic := range []bool{false, true} {
		r := withUserID(jsonRequest(t, http.MethodPut, "/api/user/update", map[string]interface{}{
			"first_name": "Solo",
			"last_name":  "Owner",
			"about_me":   "",
			"is_public":  isPublic,
		}), ownerID)
		w := httptest.NewRecorder()
		user.UpdateProfile(w, r)
		if w.Code != http.StatusOK {
			t.Fatalf("UpdateProfile(is_public=%v): status = %d", isPublic, w.Code)
		}
	}

	if queryNotificationType(t, db, ownerID, "profile_public") == "" {
		t.Error("owner did not receive a profile_public notification after switching to public with no pending follows")
	}
}

// TestGoingPublicDoesNotDuplicateNotificationOnNoOpUpdate ensures editing
// other profile fields while already public doesn't re-fire the
// auto-accept/notify logic - only an actual private -> public transition
// should trigger it.
func TestGoingPublicDoesNotDuplicateNotificationOnNoOpUpdate(t *testing.T) {
	db := newTestDB(t)
	user := &UserHandler{DB: db}
	ownerID := registerTestUser(t, db, "idempotent-owner@example.com")

	update := func(name string, isPublic bool) {
		r := withUserID(jsonRequest(t, http.MethodPut, "/api/user/update", map[string]interface{}{
			"first_name": name,
			"last_name":  "Owner",
			"about_me":   "",
			"is_public":  isPublic,
		}), ownerID)
		w := httptest.NewRecorder()
		user.UpdateProfile(w, r)
		if w.Code != http.StatusOK {
			t.Fatalf("UpdateProfile: status = %d", w.Code)
		}
	}

	update("Before", false)
	update("First", true)  // private -> public: should notify once
	update("Second", true) // already public: must not notify again

	var count int
	if err := db.QueryRow(
		"SELECT COUNT(*) FROM notifications WHERE user_id = ? AND type = 'profile_public'",
		ownerID,
	).Scan(&count); err != nil {
		t.Fatalf("failed to count notifications: %v", err)
	}
	if count != 1 {
		t.Errorf("profile_public notification count = %d, want 1 (no-op update must not duplicate it)", count)
	}
}
