package handlers

import (
    "net/http"
    "net/http/httptest"
    "testing"

    "social-network/pkg/websocket"
)

// insertTestNotification adds an unread notification for userID and
// returns its id.
func insertTestNotification(t *testing.T, wsh *WebSocketHandler, userID int64, notifType string, relatedID int64) int64 {
    t.Helper()
    result, err := wsh.DB.Exec(
        `INSERT INTO notifications (user_id, type, content, related_id) VALUES (?, ?, 'test', ?)`,
        userID, notifType, relatedID,
    )
    if err != nil {
        t.Fatalf("insert notification: %v", err)
    }
    id, _ := result.LastInsertId()
    return id
}

// TestMarkReadPushesNotificationsRead - marking one or all notifications
// read pushes "notifications_read" to the owner, so the navbar bell and the
// notifications page (and other tabs) stay in sync with each other.
// Re-marking something already read stays silent.
func TestMarkReadPushesNotificationsRead(t *testing.T) {
    db := newTestDB(t)
    hub := websocket.NewHub()
    wsh := &WebSocketHandler{DB: db, Hub: hub}

    userID := registerTestUser(t, db, "reader@example.com")
    events := attachFakeClient(hub, userID)
    first := insertTestNotification(t, wsh, userID, "new_follower", 1)
    insertTestNotification(t, wsh, userID, "new_follower", 2)

    markOne := func() {
        w := httptest.NewRecorder()
        wsh.MarkNotificationAsRead(w, withUserID(jsonRequest(t, http.MethodPut, "/api/notifications/read", map[string]interface{}{
            "notification_id": first,
        }), userID))
        if w.Code != http.StatusOK {
            t.Fatalf("MarkNotificationAsRead: status = %d", w.Code)
        }
    }

    markOne()
    if !hasType(drainEventTypes(t, events), "notifications_read") {
        t.Error("marking one read did not push notifications_read")
    }

    markOne()
    if hasType(drainEventTypes(t, events), "notifications_read") {
        t.Error("re-marking an already-read notification pushed an event")
    }

    w := httptest.NewRecorder()
    wsh.MarkAllNotificationsAsRead(w, withUserID(httptest.NewRequest(http.MethodPut, "/api/notifications/read-all", nil), userID))
    if w.Code != http.StatusOK {
        t.Fatalf("MarkAllNotificationsAsRead: status = %d", w.Code)
    }
    if !hasType(drainEventTypes(t, events), "notifications_read") {
        t.Error("mark-all did not push notifications_read")
    }
}

// TestAnsweringInvitationPushesNotificationsRead - the server marks the
// invitation notification read itself when it's answered; the invitee's
// bell has to hear about it too.
func TestAnsweringInvitationPushesNotificationsRead(t *testing.T) {
    db := newTestDB(t)
    hub := websocket.NewHub()
    gh := &GroupHandler{DB: db, Hub: hub}

    inviterID := registerTestUser(t, db, "inviter@example.com")
    inviteeID := registerTestUser(t, db, "invitee@example.com")
    insertAcceptedFollow(t, db, inviteeID, inviterID)
    groupID := createTestGroup(t, gh, inviterID)

    events := attachFakeClient(hub, inviteeID)
    callAs(t, gh.InviteUserToGroup, inviterID, "/api/group/invite", map[string]interface{}{
        "user_id": inviteeID, "group_id": groupID,
    })
    drainEventTypes(t, events) // the invitation itself

    callAs(t, gh.HandleGroupInvitation, inviteeID, "/api/group/invitation", map[string]interface{}{
        "group_id": groupID, "action": "accept",
    })
    if !hasType(drainEventTypes(t, events), "notifications_read") {
        t.Error("answering an invitation did not push notifications_read")
    }
}
