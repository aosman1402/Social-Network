package handlers

import (
    "database/sql"
    "encoding/json"
    "net/http"
    "strconv"
    "time"

    "social-network3/pkg/middleware"
    "social-network3/pkg/models"
)

type GroupHandler struct {
    DB *sql.DB
}

type CreateGroupRequest struct {
    Title       string  `json:"title"`
    Description *string `json:"description"`
    IsPrivate   bool    `json:"is_private"`
}

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

// CreateGroup - Create a new group
func (gh *GroupHandler) CreateGroup(w http.ResponseWriter, r *http.Request) {
    userID, _ := middleware.GetUserID(r)

    var req CreateGroupRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }

    if req.Title == "" {
        http.Error(w, "Title is required", http.StatusBadRequest)
        return
    }

    tx, err := gh.DB.Begin()
    if err != nil {
        http.Error(w, "Failed to start transaction", http.StatusInternalServerError)
        return
    }
    defer tx.Rollback()

    result, err := tx.Exec(
        `INSERT INTO groups (creator_id, title, description, created_at, updated_at)
         VALUES (?, ?, ?, ?, ?)`,
        userID, req.Title, req.Description, time.Now(), time.Now(),
    )
    if err != nil {
        http.Error(w, "Failed to create group: "+err.Error(), http.StatusInternalServerError)
        return
    }

    groupID, _ := result.LastInsertId()

    _, err = tx.Exec(
        `INSERT INTO group_members (group_id, user_id, status, created_at)
         VALUES (?, ?, 'active', ?)`,
        groupID, userID, time.Now(),
    )
    if err != nil {
        http.Error(w, "Failed to add creator to group", http.StatusInternalServerError)
        return
    }

    if err := tx.Commit(); err != nil {
        http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
        return
    }

    w.WriteHeader(http.StatusCreated)
    json.NewEncoder(w).Encode(map[string]interface{}{
        "message":  "Group created successfully",
        "group_id": groupID,
        "group": map[string]interface{}{
            "id":           groupID,
            "title":        req.Title,
            "description":  req.Description,
            "creator_id":   userID,
            "created_at":   time.Now(),
            "member_count": 1,
            "is_member":    true,
        },
    })
}

// GetGroup - Get group details
func (gh *GroupHandler) GetGroup(w http.ResponseWriter, r *http.Request) {
    userID, _ := middleware.GetUserID(r)

    groupIDStr := r.URL.Query().Get("id")
    groupID, err := strconv.ParseInt(groupIDStr, 10, 64)
    if err != nil {
        http.Error(w, "Invalid group ID", http.StatusBadRequest)
        return
    }

    var group struct {
        ID          int64     `json:"id"`
        CreatorID   int64     `json:"creator_id"`
        Title       string    `json:"title"`
        Description *string   `json:"description"`
        CreatedAt   time.Time `json:"created_at"`
        UpdatedAt   time.Time `json:"updated_at"`
    }

    err = gh.DB.QueryRow(
        `SELECT id, creator_id, title, description, created_at, updated_at
         FROM groups WHERE id = ?`,
        groupID,
    ).Scan(&group.ID, &group.CreatorID, &group.Title, &group.Description, &group.CreatedAt, &group.UpdatedAt)
    if err != nil {
        http.Error(w, "Group not found", http.StatusNotFound)
        return
    }

    var isMember bool
    var count int
    gh.DB.QueryRow(
        "SELECT COUNT(*) FROM group_members WHERE group_id = ? AND user_id = ? AND status = 'active'",
        groupID, userID,
    ).Scan(&count)
    isMember = count > 0

    var creator models.User
    gh.DB.QueryRow(
        `SELECT id, email, first_name, last_name, nickname, avatar_path, is_public
         FROM users WHERE id = ?`,
        group.CreatorID,
    ).Scan(&creator.ID, &creator.Email, &creator.FirstName, &creator.LastName, &creator.Nickname, &creator.AvatarPath, &creator.IsPublic)

    var memberCount int
    gh.DB.QueryRow(
        "SELECT COUNT(*) FROM group_members WHERE group_id = ? AND status = 'active'",
        groupID,
    ).Scan(&memberCount)

    var members []map[string]interface{}
    if isMember {
        rows, err := gh.DB.Query(
            `SELECT u.id, u.email, u.first_name, u.last_name, u.nickname, u.avatar_path, u.is_public,
                    gm.status, gm.created_at
             FROM group_members gm
             JOIN users u ON gm.user_id = u.id
             WHERE gm.group_id = ? AND gm.status = 'active'
             ORDER BY gm.created_at DESC`,
            groupID,
        )
        if err == nil {
            defer rows.Close()
            for rows.Next() {
                var user models.User
                var status string
                var joinedAt time.Time
                err := rows.Scan(
                    &user.ID, &user.Email, &user.FirstName, &user.LastName, &user.Nickname,
                    &user.AvatarPath, &user.IsPublic, &status, &joinedAt,
                )
                if err == nil {
                    members = append(members, map[string]interface{}{
                        "user":      user,
                        "status":    status,
                        "joined_at": joinedAt,
                    })
                }
            }
            if err = rows.Err(); err != nil {
                http.Error(w, "Error iterating members", http.StatusInternalServerError)
                return
            }
        }
    }

    var pendingRequests []map[string]interface{}
    if userID == group.CreatorID {
        rows, err := gh.DB.Query(
            `SELECT gm.id, gm.user_id, gm.created_at,
                    u.id, u.email, u.first_name, u.last_name, u.nickname, u.avatar_path, u.is_public
             FROM group_members gm
             JOIN users u ON gm.user_id = u.id
             WHERE gm.group_id = ? AND gm.status = 'pending'
             ORDER BY gm.created_at DESC`,
            groupID,
        )
        if err == nil {
            defer rows.Close()
            for rows.Next() {
                var gm struct{ ID, UserID int64 }
                var createdAt time.Time
                var user models.User
                err := rows.Scan(
                    &gm.ID, &gm.UserID, &createdAt,
                    &user.ID, &user.Email, &user.FirstName, &user.LastName, &user.Nickname,
                    &user.AvatarPath, &user.IsPublic,
                )
                if err == nil {
                    pendingRequests = append(pendingRequests, map[string]interface{}{
                        "id":         gm.ID,
                        "user":       user,
                        "created_at": createdAt,
                    })
                }
            }
            if err = rows.Err(); err != nil {
                http.Error(w, "Error iterating requests", http.StatusInternalServerError)
                return
            }
        }
    }

    var isInvited bool
    gh.DB.QueryRow(
        "SELECT COUNT(*) FROM group_members WHERE group_id = ? AND user_id = ? AND status = 'invited'",
        groupID, userID,
    ).Scan(&count)
    isInvited = count > 0

    var hasPendingRequest bool
    gh.DB.QueryRow(
        "SELECT COUNT(*) FROM group_members WHERE group_id = ? AND user_id = ? AND status = 'pending'",
        groupID, userID,
    ).Scan(&count)
    hasPendingRequest = count > 0

    json.NewEncoder(w).Encode(map[string]interface{}{
        "id":                  group.ID,
        "creator_id":          group.CreatorID,
        "title":               group.Title,
        "description":         group.Description,
        "created_at":          group.CreatedAt,
        "creator":             creator,
        "member_count":        memberCount,
        "is_member":           isMember,
        "is_invited":          isInvited,
        "has_pending_request": hasPendingRequest,
        "members":             members,
        "pending_requests":    pendingRequests,
    })
}

// GetAllGroups - Get all groups
func (gh *GroupHandler) GetAllGroups(w http.ResponseWriter, r *http.Request) {
    userID, _ := middleware.GetUserID(r)

    rows, err := gh.DB.Query(
        `SELECT g.id, g.creator_id, g.title, g.description, g.created_at,
                COUNT(DISTINCT gm.user_id) as member_count
         FROM groups g
         LEFT JOIN group_members gm ON g.id = gm.group_id AND gm.status = 'active'
         GROUP BY g.id
         ORDER BY g.created_at DESC`,
    )
    if err != nil {
        http.Error(w, "Failed to get groups", http.StatusInternalServerError)
        return
    }
    defer rows.Close()

    var groups []map[string]interface{}
    for rows.Next() {
        var group struct {
            ID        int64     `json:"id"`
            CreatorID int64     `json:"creator_id"`
            Title     string    `json:"title"`
            Desc      *string   `json:"description"`
            CreatedAt time.Time `json:"created_at"`
        }
        var memberCount int

        err := rows.Scan(&group.ID, &group.CreatorID, &group.Title, &group.Desc, &group.CreatedAt, &memberCount)
        if err != nil {
            continue
        }

        var isMember bool
        var count int
        gh.DB.QueryRow(
            "SELECT COUNT(*) FROM group_members WHERE group_id = ? AND user_id = ? AND status = 'active'",
            group.ID, userID,
        ).Scan(&count)
        isMember = count > 0

        var hasPendingRequest bool
        gh.DB.QueryRow(
            "SELECT COUNT(*) FROM group_members WHERE group_id = ? AND user_id = ? AND status = 'pending'",
            group.ID, userID,
        ).Scan(&count)
        hasPendingRequest = count > 0

        var isInvited bool
        gh.DB.QueryRow(
            "SELECT COUNT(*) FROM group_members WHERE group_id = ? AND user_id = ? AND status = 'invited'",
            group.ID, userID,
        ).Scan(&count)
        isInvited = count > 0

        groups = append(groups, map[string]interface{}{
            "id":                   group.ID,
            "creator_id":           group.CreatorID,
            "title":                group.Title,
            "description":          group.Desc,
            "created_at":           group.CreatedAt,
            "member_count":         memberCount,
            "is_member":            isMember,
            "has_pending_request":  hasPendingRequest,
            "is_invited":           isInvited,
        })
    }

    if err = rows.Err(); err != nil {
        http.Error(w, "Error iterating groups", http.StatusInternalServerError)
        return
    }

    json.NewEncoder(w).Encode(groups)
}

// GetMyGroups - Get groups the user is a member of
func (gh *GroupHandler) GetMyGroups(w http.ResponseWriter, r *http.Request) {
    userID, _ := middleware.GetUserID(r)

    rows, err := gh.DB.Query(
        `SELECT g.id, g.creator_id, g.title, g.description, g.created_at,
                COUNT(DISTINCT gm2.user_id) as member_count
         FROM groups g
         JOIN group_members gm ON g.id = gm.group_id
         LEFT JOIN group_members gm2 ON g.id = gm2.group_id AND gm2.status = 'active'
         WHERE gm.user_id = ? AND gm.status = 'active'
         GROUP BY g.id
         ORDER BY g.created_at DESC`,
        userID,
    )
    if err != nil {
        http.Error(w, "Failed to get groups", http.StatusInternalServerError)
        return
    }
    defer rows.Close()

    var groups []map[string]interface{}
    for rows.Next() {
        var group struct {
            ID        int64     `json:"id"`
            CreatorID int64     `json:"creator_id"`
            Title     string    `json:"title"`
            Desc      *string   `json:"description"`
            CreatedAt time.Time `json:"created_at"`
        }
        var memberCount int

        err := rows.Scan(&group.ID, &group.CreatorID, &group.Title, &group.Desc, &group.CreatedAt, &memberCount)
        if err != nil {
            continue
        }

        groups = append(groups, map[string]interface{}{
            "id":           group.ID,
            "creator_id":   group.CreatorID,
            "title":        group.Title,
            "description":  group.Desc,
            "created_at":   group.CreatedAt,
            "member_count": memberCount,
        })
    }

    if err = rows.Err(); err != nil {
        http.Error(w, "Error iterating groups", http.StatusInternalServerError)
        return
    }

    json.NewEncoder(w).Encode(groups)
}

// InviteUserToGroup - Invite a user to join a group
func (gh *GroupHandler) InviteUserToGroup(w http.ResponseWriter, r *http.Request) {
    userID, _ := middleware.GetUserID(r)

    var req GroupInviteRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }

    var count int
    err := gh.DB.QueryRow(
        "SELECT COUNT(*) FROM group_members WHERE group_id = ? AND user_id = ? AND status = 'active'",
        req.GroupID, userID,
    ).Scan(&count)
    if err != nil || count == 0 {
        http.Error(w, "You are not a member of this group", http.StatusForbidden)
        return
    }

    err = gh.DB.QueryRow(
        "SELECT COUNT(*) FROM group_members WHERE group_id = ? AND user_id = ?",
        req.GroupID, req.UserID,
    ).Scan(&count)
    if err == nil && count > 0 {
        http.Error(w, "User is already a member or has a pending request", http.StatusBadRequest)
        return
    }

    _, err = gh.DB.Exec(
        `INSERT INTO group_members (group_id, user_id, status, created_at, updated_at)
         VALUES (?, ?, 'invited', ?, ?)`,
        req.GroupID, req.UserID, time.Now(), time.Now(),
    )
    if err != nil {
        http.Error(w, "Failed to invite user: "+err.Error(), http.StatusInternalServerError)
        return
    }

    var groupTitle string
    gh.DB.QueryRow("SELECT title FROM groups WHERE id = ?", req.GroupID).Scan(&groupTitle)

    var inviterName string
    gh.DB.QueryRow(
        "SELECT first_name || ' ' || last_name FROM users WHERE id = ?",
        userID,
    ).Scan(&inviterName)

    _, err = gh.DB.Exec(
        `INSERT INTO notifications (user_id, type, content, related_id, created_at)
         VALUES (?, 'group_invitation', ?, ?, ?)`,
        req.UserID, inviterName+" invited you to join: "+groupTitle, req.GroupID, time.Now(),
    )
    if err != nil {
        // Log error but don't fail
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
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }

    var count int
    err := gh.DB.QueryRow(
        "SELECT COUNT(*) FROM group_members WHERE group_id = ? AND user_id = ?",
        req.GroupID, userID,
    ).Scan(&count)
    if err == nil && count > 0 {
        http.Error(w, "You are already a member or have a pending request", http.StatusBadRequest)
        return
    }

    _, err = gh.DB.Exec(
        `INSERT INTO group_members (group_id, user_id, status, created_at, updated_at)
         VALUES (?, ?, 'pending', ?, ?)`,
        req.GroupID, userID, time.Now(), time.Now(),
    )
    if err != nil {
        http.Error(w, "Failed to request to join: "+err.Error(), http.StatusInternalServerError)
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
            _, err = gh.DB.Exec(
                `INSERT INTO notifications (user_id, type, content, related_id, created_at)
                 VALUES (?, 'group_request', ?, ?, ?)`,
                creatorID, userName+" requested to join: "+groupTitle, req.GroupID, time.Now(),
            )
            if err != nil {
                // Log error but don't fail
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
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }

    var groupID, requestingUserID int64
    err := gh.DB.QueryRow(
        "SELECT group_id, user_id FROM group_members WHERE id = ? AND status = 'pending'",
        req.RequestID,
    ).Scan(&groupID, &requestingUserID)
    if err != nil {
        http.Error(w, "Request not found", http.StatusNotFound)
        return
    }

    var creatorID int64
    err = gh.DB.QueryRow(
        "SELECT creator_id FROM groups WHERE id = ?",
        groupID,
    ).Scan(&creatorID)
    if err != nil || creatorID != userID {
        http.Error(w, "Only group creator can handle requests", http.StatusForbidden)
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
        http.Error(w, "Failed to update request", http.StatusInternalServerError)
        return
    }

    if req.Action == "accept" {
        var groupTitle string
        gh.DB.QueryRow("SELECT title FROM groups WHERE id = ?", groupID).Scan(&groupTitle)

        _, err = gh.DB.Exec(
            `INSERT INTO notifications (user_id, type, content, related_id, created_at)
             VALUES (?, 'group_request_accepted', ?, ?, ?)`,
            requestingUserID, "Your request to join: "+groupTitle+" has been accepted", groupID, time.Now(),
        )
        if err != nil {
            // Log error but don't fail
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
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }

    var groupID int64
    err := gh.DB.QueryRow(
        "SELECT group_id FROM group_members WHERE id = ? AND user_id = ? AND status = 'invited'",
        req.InvitationID, userID,
    ).Scan(&groupID)
    if err != nil {
        http.Error(w, "Invitation not found", http.StatusNotFound)
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
        http.Error(w, "Failed to update invitation", http.StatusInternalServerError)
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
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }

    var creatorID int64
    err := gh.DB.QueryRow(
        "SELECT creator_id FROM groups WHERE id = ?",
        req.GroupID,
    ).Scan(&creatorID)
    if err != nil {
        http.Error(w, "Group not found", http.StatusNotFound)
        return
    }

    if creatorID == userID {
        http.Error(w, "Group creator cannot leave the group", http.StatusBadRequest)
        return
    }

    _, err = gh.DB.Exec(
        "DELETE FROM group_members WHERE group_id = ? AND user_id = ?",
        req.GroupID, userID,
    )
    if err != nil {
        http.Error(w, "Failed to leave group", http.StatusInternalServerError)
        return
    }

    json.NewEncoder(w).Encode(map[string]string{
        "message": "Left group successfully",
    })
}

// CreateEvent - Create a group event
func (gh *GroupHandler) CreateEvent(w http.ResponseWriter, r *http.Request) {
    userID, _ := middleware.GetUserID(r)

    var req struct {
        GroupID     int64  `json:"group_id"`
        Title       string `json:"title"`
        Description string `json:"description"`
        EventTime   string `json:"event_time"`
    }

    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }

    if req.Title == "" || req.EventTime == "" {
        http.Error(w, "Title and time are required", http.StatusBadRequest)
        return
    }

    eventTime, err := time.Parse(time.RFC3339, req.EventTime)
    if err != nil {
        http.Error(w, "Invalid event time format", http.StatusBadRequest)
        return
    }

    var count int
    err = gh.DB.QueryRow(
        "SELECT COUNT(*) FROM group_members WHERE group_id = ? AND user_id = ? AND status = 'active'",
        req.GroupID, userID,
    ).Scan(&count)
    if err != nil || count == 0 {
        http.Error(w, "You are not a member of this group", http.StatusForbidden)
        return
    }

    result, err := gh.DB.Exec(
        `INSERT INTO events (group_id, creator_id, title, description, event_time, created_at, updated_at)
         VALUES (?, ?, ?, ?, ?, ?, ?)`,
        req.GroupID, userID, req.Title, req.Description, eventTime, time.Now(), time.Now(),
    )
    if err != nil {
        http.Error(w, "Failed to create event: "+err.Error(), http.StatusInternalServerError)
        return
    }

    eventID, _ := result.LastInsertId()

    var creator models.User
    gh.DB.QueryRow(
        "SELECT first_name, last_name FROM users WHERE id = ?",
        userID,
    ).Scan(&creator.FirstName, &creator.LastName)

    var groupTitle string
    gh.DB.QueryRow("SELECT title FROM groups WHERE id = ?", req.GroupID).Scan(&groupTitle)

    rows, err := gh.DB.Query(
        "SELECT user_id FROM group_members WHERE group_id = ? AND status = 'active' AND user_id != ?",
        req.GroupID, userID,
    )
    if err == nil {
        defer rows.Close()
        for rows.Next() {
            var memberID int64
            rows.Scan(&memberID)
            gh.DB.Exec(
                `INSERT INTO notifications (user_id, type, content, related_id, created_at)
                 VALUES (?, 'event_created', ?, ?, ?)`,
                memberID, "New event in "+groupTitle+": "+req.Title, eventID, time.Now(),
            )
        }
        if err = rows.Err(); err != nil {
            http.Error(w, "Error iterating members", http.StatusInternalServerError)
            return
        }
    }

    json.NewEncoder(w).Encode(map[string]interface{}{
        "message":  "Event created successfully",
        "event_id": eventID,
        "event": map[string]interface{}{
            "id":          eventID,
            "group_id":    req.GroupID,
            "title":       req.Title,
            "description": req.Description,
            "event_time":  eventTime,
            "created_at":  time.Now(),
            "creator":     creator,
        },
    })
}

// GetEvents - Get events for a group
func (gh *GroupHandler) GetEvents(w http.ResponseWriter, r *http.Request) {
    userID, _ := middleware.GetUserID(r)

    groupIDStr := r.URL.Query().Get("group_id")
    groupID, err := strconv.ParseInt(groupIDStr, 10, 64)
    if err != nil {
        http.Error(w, "Invalid group ID", http.StatusBadRequest)
        return
    }

    var count int
    err = gh.DB.QueryRow(
        "SELECT COUNT(*) FROM group_members WHERE group_id = ? AND user_id = ? AND status = 'active'",
        groupID, userID,
    ).Scan(&count)
    if err != nil || count == 0 {
        http.Error(w, "You are not a member of this group", http.StatusForbidden)
        return
    }

    rows, err := gh.DB.Query(
        `SELECT e.id, e.group_id, e.creator_id, e.title, e.description, e.event_time, e.created_at,
                u.id, u.email, u.first_name, u.last_name, u.nickname, u.avatar_path, u.is_public
         FROM events e
         JOIN users u ON e.creator_id = u.id
         WHERE e.group_id = ?
         ORDER BY e.event_time ASC`,
        groupID,
    )
    if err != nil {
        http.Error(w, "Failed to get events", http.StatusInternalServerError)
        return
    }
    defer rows.Close()

    var events []map[string]interface{}
    for rows.Next() {
        var event struct {
            ID          int64      `json:"id"`
            GroupID     int64      `json:"group_id"`
            CreatorID   int64      `json:"creator_id"`
            Title       string     `json:"title"`
            Description *string    `json:"description"`
            EventTime   time.Time  `json:"event_time"`
            CreatedAt   time.Time  `json:"created_at"`
        }
        var creator models.User

        err := rows.Scan(
            &event.ID, &event.GroupID, &event.CreatorID, &event.Title, &event.Description,
            &event.EventTime, &event.CreatedAt,
            &creator.ID, &creator.Email, &creator.FirstName, &creator.LastName,
            &creator.Nickname, &creator.AvatarPath, &creator.IsPublic,
        )
        if err != nil {
            continue
        }

        var userResponse string
        gh.DB.QueryRow(
            "SELECT response FROM event_responses WHERE event_id = ? AND user_id = ?",
            event.ID, userID,
        ).Scan(&userResponse)

        var responses []map[string]interface{}
        rows2, err := gh.DB.Query(
            `SELECT er.user_id, er.response, er.created_at,
                    u.id, u.email, u.first_name, u.last_name, u.nickname, u.avatar_path, u.is_public
             FROM event_responses er
             JOIN users u ON er.user_id = u.id
             WHERE er.event_id = ?`,
            event.ID,
        )
        if err == nil {
            defer rows2.Close()
            for rows2.Next() {
                var user models.User
                var response string
                var createdAt time.Time
                err := rows2.Scan(
                    &user.ID, &response, &createdAt,
                    &user.ID, &user.Email, &user.FirstName, &user.LastName,
                    &user.Nickname, &user.AvatarPath, &user.IsPublic,
                )
                if err == nil {
                    responses = append(responses, map[string]interface{}{
                        "user":      user,
                        "response":  response,
                        "created_at": createdAt,
                    })
                }
            }
            if err = rows2.Err(); err != nil {
                http.Error(w, "Error iterating responses", http.StatusInternalServerError)
                return
            }
        }

        events = append(events, map[string]interface{}{
            "id":            event.ID,
            "group_id":      event.GroupID,
            "creator":       creator,
            "title":         event.Title,
            "description":   event.Description,
            "event_time":    event.EventTime,
            "created_at":    event.CreatedAt,
            "user_response": userResponse,
            "responses":     responses,
        })
    }

    if err = rows.Err(); err != nil {
        http.Error(w, "Error iterating events", http.StatusInternalServerError)
        return
    }

    json.NewEncoder(w).Encode(events)
}

// RespondToEvent - Respond to an event (going/not going)
func (gh *GroupHandler) RespondToEvent(w http.ResponseWriter, r *http.Request) {
    userID, _ := middleware.GetUserID(r)

    var req struct {
        EventID  int64  `json:"event_id"`
        Response string `json:"response"`
    }

    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }

    if req.Response != "going" && req.Response != "not_going" {
        http.Error(w, "Invalid response. Must be 'going' or 'not_going'", http.StatusBadRequest)
        return
    }

    var groupID int64
    err := gh.DB.QueryRow(
        "SELECT group_id FROM events WHERE id = ?",
        req.EventID,
    ).Scan(&groupID)
    if err != nil {
        http.Error(w, "Event not found", http.StatusNotFound)
        return
    }

    var count int
    err = gh.DB.QueryRow(
        "SELECT COUNT(*) FROM group_members WHERE group_id = ? AND user_id = ? AND status = 'active'",
        groupID, userID,
    ).Scan(&count)
    if err != nil || count == 0 {
        http.Error(w, "You are not a member of this group", http.StatusForbidden)
        return
    }

    _, err = gh.DB.Exec(
        `INSERT INTO event_responses (event_id, user_id, response, created_at, updated_at)
         VALUES (?, ?, ?, ?, ?)
         ON CONFLICT(event_id, user_id) DO UPDATE SET response = ?, updated_at = ?`,
        req.EventID, userID, req.Response, time.Now(), time.Now(), req.Response, time.Now(),
    )
    if err != nil {
        http.Error(w, "Failed to respond to event", http.StatusInternalServerError)
        return
    }

    json.NewEncoder(w).Encode(map[string]string{
        "message": "Response recorded successfully",
    })
}

// GetGroupPosts - Get posts from a group
func (gh *GroupHandler) GetGroupPosts(w http.ResponseWriter, r *http.Request) {
    userID, _ := middleware.GetUserID(r)

    groupIDStr := r.URL.Query().Get("group_id")
    groupID, err := strconv.ParseInt(groupIDStr, 10, 64)
    if err != nil {
        http.Error(w, "Invalid group ID", http.StatusBadRequest)
        return
    }

    var count int
    err = gh.DB.QueryRow(
        "SELECT COUNT(*) FROM group_members WHERE group_id = ? AND user_id = ? AND status = 'active'",
        groupID, userID,
    ).Scan(&count)
    if err != nil || count == 0 {
        http.Error(w, "You are not a member of this group", http.StatusForbidden)
        return
    }

    rows, err := gh.DB.Query(
        `SELECT p.id, p.user_id, p.content, p.image_path, p.privacy_level, p.group_id, p.created_at, p.updated_at,
                u.id, u.email, u.first_name, u.last_name, u.nickname, u.avatar_path, u.is_public,
                COUNT(DISTINCT c.id) as comment_count
         FROM posts p
         JOIN users u ON p.user_id = u.id
         LEFT JOIN comments c ON p.id = c.post_id
         WHERE p.group_id = ?
         GROUP BY p.id
         ORDER BY p.created_at DESC
         LIMIT 50`,
        groupID,
    )
  
    if err != nil {
        http.Error(w, "Failed to get posts: "+err.Error(), http.StatusInternalServerError)
        return
    }
    defer rows.Close()

    var posts []map[string]interface{}
    for rows.Next() {
        var post models.Post
        var user models.User
        var commentCount int
        err := rows.Scan(
            &post.ID, &post.UserID, &post.Content, &post.ImagePath, &post.PrivacyLevel,
            &post.GroupID, &post.CreatedAt, &post.UpdatedAt,
            &user.ID, &user.Email, &user.FirstName, &user.LastName, &user.Nickname,
            &user.AvatarPath, &user.IsPublic,
            &commentCount,
        )
        if err != nil {
            continue
        }
        post.User = &user
        posts = append(posts, map[string]interface{}{
            "id":            post.ID,
            "user_id":       post.UserID,
            "content":       post.Content,
            "image_path":    post.ImagePath,
            "privacy_level": post.PrivacyLevel,
            "group_id":      post.GroupID,
            "created_at":    post.CreatedAt,
            "updated_at":    post.UpdatedAt,
            "user":          post.User,
            "comment_count": commentCount,
        })
    }
}