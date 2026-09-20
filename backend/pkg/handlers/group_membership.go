package handlers

import (
    "encoding/json"
    "log"
    "net/http"
    "time"

    "social-network/pkg/middleware"
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

    err = gh.DB.QueryRow(
        "SELECT COUNT(*) FROM group_members WHERE group_id = ? AND user_id = ?",
        req.GroupID, req.UserID,
    ).Scan(&count)
    if err == nil && count > 0 {
        writeJSONError(w, http.StatusBadRequest, "User is already a member or has a pending request")
        return
    }

    _, err = gh.DB.Exec(
        `INSERT INTO group_members (group_id, user_id, status, created_at, updated_at)
         VALUES (?, ?, 'invited', ?, ?)`,
        req.GroupID, req.UserID, time.Now(), time.Now(),
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

        var notifCount int
        gh.DB.QueryRow(
            "SELECT COUNT(*) FROM notifications WHERE user_id = ? AND type = 'group_request' AND related_id = ?",
            creatorID, req.GroupID,
        ).Scan(&notifCount)

        if notifCount == 0 {
            content := userName + " requested to join: " + groupTitle
            createdAt := time.Now()
            result, err := gh.DB.Exec(
                `INSERT INTO notifications (user_id, type, content, related_id, created_at)
                 VALUES (?, 'group_request', ?, ?, ?)`,
                creatorID, content, req.GroupID, createdAt,
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

    status := "declined"
    if req.Action == "accept" {
        status = "active"
    }

    _, err = gh.DB.Exec(
        "UPDATE group_members SET status = ?, updated_at = ? WHERE id = ?",
        status, time.Now(), req.RequestID,
    )
    if err != nil {
        writeJSONError(w, http.StatusInternalServerError, "Failed to update request")
        return
    }

    if req.Action == "accept" {
        var groupTitle string
        gh.DB.QueryRow("SELECT title FROM groups WHERE id = ?", groupID).Scan(&groupTitle)

        content := "Your request to join: " + groupTitle + " has been accepted"
        createdAt := time.Now()
        result, err := gh.DB.Exec(
            `INSERT INTO notifications (user_id, type, content, related_id, created_at)
             VALUES (?, 'group_request_accepted', ?, ?, ?)`,
            requestingUserID, content, groupID, createdAt,
        )
        if err == nil {
            notificationID, _ := result.LastInsertId()
            gh.Hub.NotifyUser(requestingUserID, map[string]interface{}{
                "id":         notificationID,
                "user_id":    requestingUserID,
                "type":       "group_request_accepted",
                "content":    content,
                "is_read":    false,
                "related_id": groupID,
                "created_at": createdAt,
            })
        }
    }

    json.NewEncoder(w).Encode(map[string]string{
        "message": "Request " + req.Action + "ed successfully",
    })
}

// HandleGroupInvitation - Handle a group invitation (accept/decline)
func (gh *GroupHandler) HandleGroupInvitation(w http.ResponseWriter, r *http.Request) {
    userID, _ := middleware.GetUserID(r)

    var req struct {
        InvitationID int64  `json:"invitation_id"`
        Action       string `json:"action"`
    }

    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeJSONError(w, http.StatusBadRequest, "Invalid request body")
        return
    }

    var groupID int64
    err := gh.DB.QueryRow(
        "SELECT group_id FROM group_members WHERE id = ? AND user_id = ? AND status = 'invited'",
        req.InvitationID, userID,
    ).Scan(&groupID)
    if err != nil {
        writeJSONError(w, http.StatusNotFound, "Invitation not found")
        return
    }

    status := "declined"
    if req.Action == "accept" {
        status = "active"
    }

    _, err = gh.DB.Exec(
        "UPDATE group_members SET status = ?, updated_at = ? WHERE id = ?",
        status, time.Now(), req.InvitationID,
    )
    if err != nil {
        writeJSONError(w, http.StatusInternalServerError, "Failed to update invitation")
        return
    }

    json.NewEncoder(w).Encode(map[string]string{
        "message": "Invitation " + req.Action + "ed successfully",
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
