package handlers

import (
    "database/sql"
    "encoding/json"
    "log"
    "net/http"
    "strconv"
    "time"

    "social-network/pkg/middleware"
    "social-network/pkg/models"
    "social-network/pkg/websocket"
)

type GroupHandler struct {
    DB  *sql.DB
    Hub *websocket.Hub
}

type CreateGroupRequest struct {
    Title       string  `json:"title"`
    Description *string `json:"description"`
    IsPrivate   bool    `json:"is_private"`
}

// CreateGroup - Create a new group
func (gh *GroupHandler) CreateGroup(w http.ResponseWriter, r *http.Request) {
    userID, _ := middleware.GetUserID(r)

    var req CreateGroupRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeJSONError(w, http.StatusBadRequest, "Invalid request body")
        return
    }

    if req.Title == "" {
        writeJSONError(w, http.StatusBadRequest, "Title is required")
        return
    }

    tx, err := gh.DB.Begin()
    if err != nil {
        writeJSONError(w, http.StatusInternalServerError, "Failed to start transaction")
        return
    }
    defer tx.Rollback()

    result, err := tx.Exec(
        `INSERT INTO groups (creator_id, title, description, created_at, updated_at)
         VALUES (?, ?, ?, ?, ?)`,
        userID, req.Title, req.Description, time.Now(), time.Now(),
    )
    if err != nil {
        log.Printf("CreateGroup: failed to insert group: %v", err)
        writeJSONError(w, http.StatusInternalServerError, "Failed to create group")
        return
    }

    groupID, _ := result.LastInsertId()

    _, err = tx.Exec(
        `INSERT INTO group_members (group_id, user_id, status, created_at)
         VALUES (?, ?, 'active', ?)`,
        groupID, userID, time.Now(),
    )
    if err != nil {
        writeJSONError(w, http.StatusInternalServerError, "Failed to add creator to group")
        return
    }

    if err := tx.Commit(); err != nil {
        writeJSONError(w, http.StatusInternalServerError, "Failed to commit transaction")
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
        writeJSONError(w, http.StatusBadRequest, "Invalid group ID")
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
        writeJSONError(w, http.StatusNotFound, "Group not found")
        return
    }

    // One lookup for the caller's own membership row, rather than three
    // separate COUNT(*) queries (is_member/is_invited/has_pending_request)
    // against the same (group_id, user_id) pair.
    var myStatus sql.NullString
    gh.DB.QueryRow(
        "SELECT status FROM group_members WHERE group_id = ? AND user_id = ?",
        groupID, userID,
    ).Scan(&myStatus)
    isMember := myStatus.String == "active"
    isInvited := myStatus.String == "invited"
    hasPendingRequest := myStatus.String == "pending"

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

    members := []map[string]interface{}{}
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
                writeJSONError(w, http.StatusInternalServerError, "Error iterating members")
                return
            }
        }
    }

    pendingRequests := []map[string]interface{}{}
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
                writeJSONError(w, http.StatusInternalServerError, "Error iterating requests")
                return
            }
        }
    }

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
    limit, offset := parsePagination(r, 50, 100)

    // The caller's own membership status (if any) per group is fetched via
    // a second LEFT JOIN keyed to userID, rather than three separate
    // per-row COUNT(*) queries (is_member/has_pending_request/is_invited) -
    // previously a group list of N groups issued 1 + 3N queries.
    rows, err := gh.DB.Query(
        `SELECT g.id, g.creator_id, g.title, g.description, g.created_at,
                COUNT(DISTINCT gm.user_id) as member_count,
                my_membership.status as my_status
         FROM groups g
         LEFT JOIN group_members gm ON g.id = gm.group_id AND gm.status = 'active'
         LEFT JOIN group_members my_membership ON g.id = my_membership.group_id AND my_membership.user_id = ?
         GROUP BY g.id
         ORDER BY g.created_at DESC
         LIMIT ? OFFSET ?`,
        userID, limit, offset,
    )
    if err != nil {
        writeJSONError(w, http.StatusInternalServerError, "Failed to get groups")
        return
    }
    defer rows.Close()

    groups := []map[string]interface{}{}
    for rows.Next() {
        var group struct {
            ID        int64     `json:"id"`
            CreatorID int64     `json:"creator_id"`
            Title     string    `json:"title"`
            Desc      *string   `json:"description"`
            CreatedAt time.Time `json:"created_at"`
        }
        var memberCount int
        var myStatus sql.NullString

        err := rows.Scan(&group.ID, &group.CreatorID, &group.Title, &group.Desc, &group.CreatedAt, &memberCount, &myStatus)
        if err != nil {
            continue
        }

        groups = append(groups, map[string]interface{}{
            "id":                  group.ID,
            "creator_id":          group.CreatorID,
            "title":               group.Title,
            "description":         group.Desc,
            "created_at":          group.CreatedAt,
            "member_count":        memberCount,
            "is_member":           myStatus.String == "active",
            "has_pending_request": myStatus.String == "pending",
            "is_invited":          myStatus.String == "invited",
        })
    }

    if err = rows.Err(); err != nil {
        writeJSONError(w, http.StatusInternalServerError, "Error iterating groups")
        return
    }

    json.NewEncoder(w).Encode(groups)
}

// GetMyGroups - Get groups the user is a member of
func (gh *GroupHandler) GetMyGroups(w http.ResponseWriter, r *http.Request) {
    userID, _ := middleware.GetUserID(r)
    limit, offset := parsePagination(r, 50, 100)

    rows, err := gh.DB.Query(
        `SELECT g.id, g.creator_id, g.title, g.description, g.created_at,
                COUNT(DISTINCT gm2.user_id) as member_count
         FROM groups g
         JOIN group_members gm ON g.id = gm.group_id
         LEFT JOIN group_members gm2 ON g.id = gm2.group_id AND gm2.status = 'active'
         WHERE gm.user_id = ? AND gm.status = 'active'
         GROUP BY g.id
         ORDER BY g.created_at DESC
         LIMIT ? OFFSET ?`,
        userID, limit, offset,
    )
    if err != nil {
        writeJSONError(w, http.StatusInternalServerError, "Failed to get groups")
        return
    }
    defer rows.Close()

    groups := []map[string]interface{}{}
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
        writeJSONError(w, http.StatusInternalServerError, "Error iterating groups")
        return
    }

    json.NewEncoder(w).Encode(groups)
}

// GetGroupPosts - Get posts from a group
func (gh *GroupHandler) GetGroupPosts(w http.ResponseWriter, r *http.Request) {
    userID, _ := middleware.GetUserID(r)

    groupIDStr := r.URL.Query().Get("group_id")
    groupID, err := strconv.ParseInt(groupIDStr, 10, 64)
    if err != nil {
        writeJSONError(w, http.StatusBadRequest, "Invalid group ID")
        return
    }

    var count int
    err = gh.DB.QueryRow(
        "SELECT COUNT(*) FROM group_members WHERE group_id = ? AND user_id = ? AND status = 'active'",
        groupID, userID,
    ).Scan(&count)
    if err != nil || count == 0 {
        writeJSONError(w, http.StatusForbidden, "You are not a member of this group")
        return
    }

    limit, offset := parsePagination(r, 50, 100)
    rows, err := gh.DB.Query(
        `SELECT `+postRowColumns+`
         FROM posts p
         JOIN users u ON p.user_id = u.id
         WHERE p.group_id = ?
         ORDER BY p.created_at DESC
         LIMIT ? OFFSET ?`,
        userID, userID, groupID, limit, offset,
    )

    if err != nil {
        log.Printf("GetGroupPosts: failed to query posts: %v", err)
        writeJSONError(w, http.StatusInternalServerError, "Failed to get posts")
        return
    }
    defer rows.Close()

    posts := []map[string]interface{}{}
    for rows.Next() {
        post, err := scanPostRow(rows)
        if err != nil {
            continue
        }
        posts = append(posts, post)
    }

    if err = rows.Err(); err != nil {
        writeJSONError(w, http.StatusInternalServerError, "Error iterating posts")
        return
    }

    json.NewEncoder(w).Encode(posts)
}