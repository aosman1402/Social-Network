package handlers

import (
    "database/sql"
    "encoding/json"
    "log"
    "net/http"
    "time"

    "social-network/pkg/middleware"
    "social-network/pkg/websocket"
)

type GroupInviteRequest struct {
    UserID  int64 `json:"user_id"`
    GroupID int64 `json:"group_id"`
}

type GroupJoinRequest struct {
    GroupID int64 `json:"group_id"`
}

type GroupRequestAction struct {
    RequestID int64  `json:"request_id"`
    Action    string `json:"action"`
}

// InviteUserToGroup - Invite a user to join a group
func (gh *GroupHandler) InviteUserToGroup(w http.ResponseWriter, r *http.Request) {
    userID, _ := middleware.GetUserID(r)

    var req GroupInviteRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeJSONError(w, http.StatusBadRequest, "Invalid request body")
        return
    }

    var count int
    err := gh.DB.QueryRow(
        "SELECT COUNT(*) FROM group_members WHERE group_id = ? AND user_id = ? AND status = 'active'",
        req.GroupID, userID,
    ).Scan(&count)
    if err != nil || count == 0 {
        writeJSONError(w, http.StatusForbidden, "You are not a member of this group")
        return
    }

    // Invitations are limited to people the inviter has a follow
    // relationship with (either direction) - the same rule as direct
    // messages. The invite UI only ever lists followers, but the API
    // previously accepted any user_id.
    connected, err := websocket.CanDirectMessage(gh.DB, userID, req.UserID)
    if err != nil {
        writeJSONError(w, http.StatusInternalServerError, "Database error")
        return
    }
    if !connected {
        writeJSONError(w, http.StatusForbidden, "You can only invite users who follow you or that you follow")
        return
    }

    err = gh.DB.QueryRow(
        "SELECT COUNT(*) FROM group_members WHERE group_id = ? AND user_id = ?",
        req.GroupID, req.UserID,
    ).Scan(&count)
    if err == nil && count > 0 {
        writeJSONError(w, http.StatusBadRequest, "User is already a member or has a pending request")
        return
    }

    _, err = gh.DB.Exec(
        `INSERT INTO group_members (group_id, user_id, status, invited_by, created_at, updated_at)
         VALUES (?, ?, 'invited', ?, ?, ?)`,
        req.GroupID, req.UserID, userID, time.Now(), time.Now(),
    )
    if err != nil {
        log.Printf("InviteUserToGroup: failed to insert invite: %v", err)
        writeJSONError(w, http.StatusInternalServerError, "Failed to invite user")
        return
    }

    var groupTitle string
    gh.DB.QueryRow("SELECT title FROM groups WHERE id = ?", req.GroupID).Scan(&groupTitle)

    var inviterName string
    gh.DB.QueryRow(
        "SELECT first_name || ' ' || last_name FROM users WHERE id = ?",
        userID,
    ).Scan(&inviterName)

    inviteContent := inviterName + " invited you to join: " + groupTitle
    inviteCreatedAt := time.Now()
    result, err := gh.DB.Exec(
        `INSERT INTO notifications (user_id, type, content, related_id, created_at)
         VALUES (?, 'group_invitation', ?, ?, ?)`,
        req.UserID, inviteContent, req.GroupID, inviteCreatedAt,
    )
    if err == nil {
        notificationID, _ := result.LastInsertId()
        gh.Hub.NotifyUser(req.UserID, map[string]interface{}{
            "id":         notificationID,
            "user_id":    req.UserID,
            "type":       "group_invitation",
            "content":    inviteContent,
            "is_read":    false,
            "related_id": req.GroupID,
            "created_at": inviteCreatedAt,
        })
    }

    json.NewEncoder(w).Encode(map[string]string{
        "message": "User invited successfully",
    })
}

// RequestToJoinGroup - Request to join a group
func (gh *GroupHandler) RequestToJoinGroup(w http.ResponseWriter, r *http.Request) {
    userID, _ := middleware.GetUserID(r)

    var req GroupJoinRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeJSONError(w, http.StatusBadRequest, "Invalid request body")
        return
    }

    var count int
    err := gh.DB.QueryRow(
        "SELECT COUNT(*) FROM group_members WHERE group_id = ? AND user_id = ?",
        req.GroupID, userID,
    ).Scan(&count)
    if err == nil && count > 0 {
        writeJSONError(w, http.StatusBadRequest, "You are already a member or have a pending request")
        return
    }

    _, err = gh.DB.Exec(
        `INSERT INTO group_members (group_id, user_id, status, created_at, updated_at)
         VALUES (?, ?, 'pending', ?, ?)`,
        req.GroupID, userID, time.Now(), time.Now(),
    )
    if err != nil {
        log.Printf("RequestToJoinGroup: failed to insert request: %v", err)
        writeJSONError(w, http.StatusInternalServerError, "Failed to request to join")
        return
    }

    var creatorID int64
    var groupTitle string
    err = gh.DB.QueryRow(
        "SELECT creator_id, title FROM groups WHERE id = ?",
        req.GroupID,
    ).Scan(&creatorID, &groupTitle)

    if err == nil {
        var userName string
        gh.DB.QueryRow(
            "SELECT first_name || ' ' || last_name FROM users WHERE id = ?",
            userID,
        ).Scan(&userName)

        // Skip only a duplicate of this same requester's still-unread
        // request. This used to check for *any* group_request on the
        // group, so after the first person asked to join, every later
        // request reached the creator with no notification at all.
        var notifCount int
        gh.DB.QueryRow(
            `SELECT COUNT(*) FROM notifications
             WHERE user_id = ? AND type = 'group_request' AND related_id = ? AND actor_id = ? AND is_read = 0`,
            creatorID, req.GroupID, userID,
        ).Scan(&notifCount)

        if notifCount == 0 {
            content := userName + " requested to join: " + groupTitle
            createdAt := time.Now()
            result, err := gh.DB.Exec(
                `INSERT INTO notifications (user_id, type, content, related_id, actor_id, created_at)
                 VALUES (?, 'group_request', ?, ?, ?, ?)`,
                creatorID, content, req.GroupID, userID, createdAt,
            )
            if err == nil {
                notificationID, _ := result.LastInsertId()
                gh.Hub.NotifyUser(creatorID, map[string]interface{}{
                    "id":         notificationID,
                    "user_id":    creatorID,
                    "type":       "group_request",
                    "content":    content,
                    "is_read":    false,
                    "related_id": req.GroupID,
                    "created_at": createdAt,
                })
            }
        }
    }

    json.NewEncoder(w).Encode(map[string]string{
        "message": "Request to join sent successfully",
    })
}

// HandleGroupRequest - Handle a group join request (accept/decline)
func (gh *GroupHandler) HandleGroupRequest(w http.ResponseWriter, r *http.Request) {
    userID, _ := middleware.GetUserID(r)

    var req GroupRequestAction
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeJSONError(w, http.StatusBadRequest, "Invalid request body")
        return
    }

    if req.Action != "accept" && req.Action != "decline" {
        writeJSONError(w, http.StatusBadRequest, "Invalid action. Must be 'accept' or 'decline'")
        return
    }

    var groupID, requestingUserID int64
    err := gh.DB.QueryRow(
        "SELECT group_id, user_id FROM group_members WHERE id = ? AND status = 'pending'",
        req.RequestID,
    ).Scan(&groupID, &requestingUserID)
    if err != nil {
        writeJSONError(w, http.StatusNotFound, "Request not found")
        return
    }

    var creatorID int64
    err = gh.DB.QueryRow(
        "SELECT creator_id FROM groups WHERE id = ?",
        groupID,
    ).Scan(&creatorID)
    if err != nil || creatorID != userID {
        writeJSONError(w, http.StatusForbidden, "Only group creator can handle requests")
        return
    }

    if err := resolveMembership(gh.DB, req.RequestID, req.Action); err != nil {
        writeJSONError(w, http.StatusInternalServerError, "Failed to update request")
        return
    }

    // The request has been answered - clear its notification from the
    // creator's unread count, live in every open tab.
    markNotificationsRead(gh.DB, gh.Hub, userID,
        "type = 'group_request' AND related_id = ? AND actor_id = ?", groupID, requestingUserID)

    // Tell the requester either way - a declined request used to produce
    // no notification at all, so they never found out.
    var groupTitle string
    gh.DB.QueryRow("SELECT title FROM groups WHERE id = ?", groupID).Scan(&groupTitle)

    notifType := "group_request_accepted"
    content := "Your request to join: " + groupTitle + " has been accepted"
    if req.Action == "decline" {
        notifType = "group_request_declined"
        content = "Your request to join: " + groupTitle + " has been declined"
    }

    createdAt := time.Now()
    result, err := gh.DB.Exec(
        `INSERT INTO notifications (user_id, type, content, related_id, created_at)
         VALUES (?, ?, ?, ?, ?)`,
        requestingUserID, notifType, content, groupID, createdAt,
    )
    if err == nil {
        notificationID, _ := result.LastInsertId()
        gh.Hub.NotifyUser(requestingUserID, map[string]interface{}{
            "id":         notificationID,
            "user_id":    requestingUserID,
            "type":       notifType,
            "content":    content,
            "is_read":    false,
            "related_id": groupID,
            "created_at": createdAt,
        })
    }

    json.NewEncoder(w).Encode(map[string]string{
        "message": "Request " + pastTense(req.Action) + " successfully",
    })
}

// resolveMembership applies an accept/decline to a pending join request or
// invitation row. Accepting makes the row an active membership; declining
// deletes it outright rather than leaving a 'declined' row behind - every
// invite/join check treats any existing row as "already a member or has a
// pending request", so a leftover row would lock that user out of the
// group for good (no re-invite, no join request).
func resolveMembership(db *sql.DB, membershipID int64, action string) error {
    if action == "accept" {
        _, err := db.Exec(
            "UPDATE group_members SET status = 'active', updated_at = ? WHERE id = ?",
            time.Now(), membershipID,
        )
        return err
    }
    _, err := db.Exec("DELETE FROM group_members WHERE id = ?", membershipID)
    return err
}

// pastTense turns a validated "accept"/"decline" action into the word used
// in response messages.
func pastTense(action string) string {
    if action == "accept" {
        return "accepted"
    }
    return "declined"
}

// HandleGroupInvitation - Handle a group invitation (accept/decline). The
// invitee identifies the invitation by group_id, since they can have at
// most one membership row per group (UNIQUE(group_id, user_id)).
func (gh *GroupHandler) HandleGroupInvitation(w http.ResponseWriter, r *http.Request) {
    userID, _ := middleware.GetUserID(r)

    var req struct {
        GroupID int64  `json:"group_id"`
        Action  string `json:"action"`
    }

    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeJSONError(w, http.StatusBadRequest, "Invalid request body")
        return
    }

    if req.Action != "accept" && req.Action != "decline" {
        writeJSONError(w, http.StatusBadRequest, "Invalid action. Must be 'accept' or 'decline'")
        return
    }

    var membershipID int64
    var invitedBy sql.NullInt64
    err := gh.DB.QueryRow(
        "SELECT id, invited_by FROM group_members WHERE group_id = ? AND user_id = ? AND status = 'invited'",
        req.GroupID, userID,
    ).Scan(&membershipID, &invitedBy)
    if err != nil {
        writeJSONError(w, http.StatusNotFound, "Invitation not found")
        return
    }

    if err := resolveMembership(gh.DB, membershipID, req.Action); err != nil {
        writeJSONError(w, http.StatusInternalServerError, "Failed to update invitation")
        return
    }

    // The invitation has been answered - clear it from the invitee's
    // unread count, and tell their open tabs so the bell updates live.
    markNotificationsRead(gh.DB, gh.Hub, userID,
        "type = 'group_invitation' AND related_id = ?", req.GroupID)

    var groupTitle string
    var creatorID int64
    gh.DB.QueryRow("SELECT title, creator_id FROM groups WHERE id = ?", req.GroupID).Scan(&groupTitle, &creatorID)

    // Tell whoever sent the invitation how it was answered. Invitations
    // created before invited_by existed fall back to the group creator.
    inviterID := creatorID
    if invitedBy.Valid {
        inviterID = invitedBy.Int64
    }

    if inviterID != 0 {
        var inviteeName string
        gh.DB.QueryRow(
            "SELECT first_name || ' ' || last_name FROM users WHERE id = ?",
            userID,
        ).Scan(&inviteeName)

        notifType := "group_invitation_accepted"
        content := inviteeName + " accepted your invitation to join: " + groupTitle
        if req.Action == "decline" {
            notifType = "group_invitation_declined"
            content = inviteeName + " declined your invitation to join: " + groupTitle
        }

        createdAt := time.Now()
        result, err := gh.DB.Exec(
            `INSERT INTO notifications (user_id, type, content, related_id, created_at)
             VALUES (?, ?, ?, ?, ?)`,
            inviterID, notifType, content, req.GroupID, createdAt,
        )
        if err == nil {
            notificationID, _ := result.LastInsertId()
            gh.Hub.NotifyUser(inviterID, map[string]interface{}{
                "id":         notificationID,
                "user_id":    inviterID,
                "type":       notifType,
                "content":    content,
                "is_read":    false,
                "related_id": req.GroupID,
                "created_at": createdAt,
            })
        }
    }

    json.NewEncoder(w).Encode(map[string]string{
        "message": "Invitation " + pastTense(req.Action) + " successfully",
    })
}

// LeaveGroup - Leave a group
func (gh *GroupHandler) LeaveGroup(w http.ResponseWriter, r *http.Request) {
    userID, _ := middleware.GetUserID(r)

    var req struct {
        GroupID int64 `json:"group_id"`
    }

    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeJSONError(w, http.StatusBadRequest, "Invalid request body")
        return
    }

    var creatorID int64
    err := gh.DB.QueryRow(
        "SELECT creator_id FROM groups WHERE id = ?",
        req.GroupID,
    ).Scan(&creatorID)
    if err != nil {
        writeJSONError(w, http.StatusNotFound, "Group not found")
        return
    }

    if creatorID == userID {
        writeJSONError(w, http.StatusBadRequest, "Group creator cannot leave the group")
        return
    }

    _, err = gh.DB.Exec(
        "DELETE FROM group_members WHERE group_id = ? AND user_id = ?",
        req.GroupID, userID,
    )
    if err != nil {
        writeJSONError(w, http.StatusInternalServerError, "Failed to leave group")
        return
    }

    json.NewEncoder(w).Encode(map[string]string{
        "message": "Left group successfully",
    })
}
