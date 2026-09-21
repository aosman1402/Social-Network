package handlers

import (
    "encoding/json"
    "net/http"
    "strconv"
    "time"

    "social-network/pkg/middleware"
    "social-network/pkg/models"
)

// FollowUser - Follow a user
func (uh *UserHandler) FollowUser(w http.ResponseWriter, r *http.Request) {
    followerID, _ := middleware.GetUserID(r)

    var req struct {
        UserID int64 `json:"user_id"`
    }

    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeJSONError(w, http.StatusBadRequest, "Invalid request body")
        return
    }

    if followerID == req.UserID {
        writeJSONError(w, http.StatusBadRequest, "Cannot follow yourself")
        return
    }

    var isPublic bool
    err := uh.DB.QueryRow(
        "SELECT is_public FROM users WHERE id = ?",
        req.UserID,
    ).Scan(&isPublic)

    if err != nil {
        writeJSONError(w, http.StatusNotFound, "User not found")
        return
    }

    var existingStatus string
    err = uh.DB.QueryRow(
        "SELECT status FROM follows WHERE follower_id = ? AND following_id = ?",
        followerID, req.UserID,
    ).Scan(&existingStatus)

    if err == nil {
        switch existingStatus {
        case "accepted":
            json.NewEncoder(w).Encode(map[string]interface{}{
                "message":  "Already following this user",
                "status":   "accepted",
                "followed": true,
            })
            return
        case "pending":
            json.NewEncoder(w).Encode(map[string]interface{}{
                "message":  "Follow request already pending",
                "status":   "pending",
                "followed": false,
            })
            return
        }
    }

    status := "pending"
    if isPublic {
        status = "accepted"
    }

    _, err = uh.DB.Exec(
        `INSERT INTO follows (follower_id, following_id, status, created_at, updated_at)
         VALUES (?, ?, ?, ?, ?)
         ON CONFLICT(follower_id, following_id) DO UPDATE SET status = ?, updated_at = ?`,
        followerID, req.UserID, status, time.Now(), time.Now(), status, time.Now(),
    )

    if err != nil {
        writeJSONError(w, http.StatusInternalServerError, "Failed to follow user")
        return
    }

    // Let the followed user know either way: a request to act on (private
    // account) or simply that they gained a follower (public account) -
    // previously only the private-account case notified anyone.
    var followerName string
    uh.DB.QueryRow(
        "SELECT first_name || ' ' || last_name FROM users WHERE id = ?",
        followerID,
    ).Scan(&followerName)

    notifType := "new_follower"
    content := followerName + " started following you"
    if !isPublic {
        notifType = "follow_request"
        content = followerName + " requested to follow you"
    }

    createdAt := time.Now()
    result, err := uh.DB.Exec(
        `INSERT INTO notifications (user_id, type, content, related_id, created_at)
         VALUES (?, ?, ?, ?, ?)`,
        req.UserID, notifType, content, followerID, createdAt,
    )
    if err == nil {
        notificationID, _ := result.LastInsertId()
        uh.Hub.NotifyUser(req.UserID, map[string]interface{}{
            "id":         notificationID,
            "user_id":    req.UserID,
            "type":       notifType,
            "content":    content,
            "is_read":    false,
            "related_id": followerID,
            "created_at": createdAt,
        })
    }

    var message string
    if isPublic {
        message = "You are now following this user"
    } else {
        message = "Follow request sent"
    }

    json.NewEncoder(w).Encode(map[string]interface{}{
        "message":  message,
        "status":   status,
        "followed": status == "accepted",
    })
}

// UnfollowUser - Unfollow a user
func (uh *UserHandler) UnfollowUser(w http.ResponseWriter, r *http.Request) {
    followerID, _ := middleware.GetUserID(r)

    var req struct {
        UserID int64 `json:"user_id"`
    }

    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeJSONError(w, http.StatusBadRequest, "Invalid request body")
        return
    }

    if followerID == req.UserID {
        writeJSONError(w, http.StatusBadRequest, "Cannot unfollow yourself")
        return
    }

    // Capture the status being removed before deleting it: a cancelled
    // still-pending request never became a real follow relationship, so
    // the target shouldn't be told they were "unfollowed" for it.
    var previousStatus string
    uh.DB.QueryRow(
        "SELECT status FROM follows WHERE follower_id = ? AND following_id = ?",
        followerID, req.UserID,
    ).Scan(&previousStatus)

    result, err := uh.DB.Exec(
        "DELETE FROM follows WHERE follower_id = ? AND following_id = ?",
        followerID, req.UserID,
    )

    if err != nil {
        writeJSONError(w, http.StatusInternalServerError, "Failed to unfollow user")
        return
    }

    rowsAffected, _ := result.RowsAffected()
    if rowsAffected == 0 {
        writeJSONError(w, http.StatusBadRequest, "Not following this user")
        return
    }

    if previousStatus == "accepted" {
        var followerName string
        uh.DB.QueryRow(
            "SELECT first_name || ' ' || last_name FROM users WHERE id = ?",
            followerID,
        ).Scan(&followerName)

        content := followerName + " unfollowed you"
        createdAt := time.Now()
        notifResult, err := uh.DB.Exec(
            `INSERT INTO notifications (user_id, type, content, related_id, created_at)
             VALUES (?, 'unfollow', ?, ?, ?)`,
            req.UserID, content, followerID, createdAt,
        )
        if err == nil {
            notificationID, _ := notifResult.LastInsertId()
            uh.Hub.NotifyUser(req.UserID, map[string]interface{}{
                "id":         notificationID,
                "user_id":    req.UserID,
                "type":       "unfollow",
                "content":    content,
                "is_read":    false,
                "related_id": followerID,
                "created_at": createdAt,
            })
        }
    }

    json.NewEncoder(w).Encode(map[string]string{
        "message": "Unfollowed successfully",
    })
}

// HandleFollowRequest - Accept or decline a follow request
func (uh *UserHandler) HandleFollowRequest(w http.ResponseWriter, r *http.Request) {
    userID, _ := middleware.GetUserID(r)

    var req struct {
        FollowerID int64  `json:"follower_id"`
        Action     string `json:"action"`
    }

    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeJSONError(w, http.StatusBadRequest, "Invalid request body")
        return
    }

    if req.Action != "accept" && req.Action != "decline" {
        writeJSONError(w, http.StatusBadRequest, "Invalid action. Must be 'accept' or 'decline'")
        return
    }

    var count int
    err := uh.DB.QueryRow(
        "SELECT COUNT(*) FROM follows WHERE follower_id = ? AND following_id = ? AND status = 'pending'",
        req.FollowerID, userID,
    ).Scan(&count)

    if err != nil || count == 0 {
        writeJSONError(w, http.StatusNotFound, "No pending follow request found")
        return
    }

    status := "declined"
    if req.Action == "accept" {
        status = "accepted"
    }

    _, err = uh.DB.Exec(
        "UPDATE follows SET status = ?, updated_at = ? WHERE follower_id = ? AND following_id = ?",
        status, time.Now(), req.FollowerID, userID,
    )

    if err != nil {
        writeJSONError(w, http.StatusInternalServerError, "Failed to update follow request")
        return
    }

    if req.Action == "accept" {
        var userName string
        uh.DB.QueryRow(
            "SELECT first_name || ' ' || last_name FROM users WHERE id = ?",
            userID,
        ).Scan(&userName)

        content := userName + " accepted your follow request"
        createdAt := time.Now()
        result, err := uh.DB.Exec(
            `INSERT INTO notifications (user_id, type, content, related_id, created_at)
             VALUES (?, 'follow_accepted', ?, ?, ?)`,
            req.FollowerID, content, userID, createdAt,
        )
        if err == nil {
            notificationID, _ := result.LastInsertId()
            uh.Hub.NotifyUser(req.FollowerID, map[string]interface{}{
                "id":         notificationID,
                "user_id":    req.FollowerID,
                "type":       "follow_accepted",
                "content":    content,
                "is_read":    false,
                "related_id": userID,
                "created_at": createdAt,
            })
        }
    }

    json.NewEncoder(w).Encode(map[string]string{
        "message": "Follow request " + req.Action + "ed",
    })
}

// GetFollowRequests - Get pending follow requests
func (uh *UserHandler) GetFollowRequests(w http.ResponseWriter, r *http.Request) {
    userID, _ := middleware.GetUserID(r)
    limit, offset := parsePagination(r, 200, 500)

    rows, err := uh.DB.Query(
        `SELECT f.id, f.follower_id, f.status, f.created_at,
                u.id, u.email, u.first_name, u.last_name, u.nickname, u.avatar_path, u.is_public
         FROM follows f
         JOIN users u ON f.follower_id = u.id
         WHERE f.following_id = ? AND f.status = 'pending'
         ORDER BY f.created_at DESC
         LIMIT ? OFFSET ?`,
        userID, limit, offset,
    )

    if err != nil {
        writeJSONError(w, http.StatusInternalServerError, "Failed to get follow requests")
        return
    }
    defer rows.Close()

    requests := []map[string]interface{}{}
    for rows.Next() {
        var follow struct{ ID, FollowerID int64 }
        var status string
        var createdAt time.Time
        var user models.User

        err := rows.Scan(
            &follow.ID, &follow.FollowerID, &status, &createdAt,
            &user.ID, &user.Email, &user.FirstName, &user.LastName, &user.Nickname,
            &user.AvatarPath, &user.IsPublic,
        )
        if err != nil {
            continue
        }

        requests = append(requests, map[string]interface{}{
            "id":         follow.ID,
            "follower":   user,
            "status":     status,
            "created_at": createdAt,
        })
    }

    if err = rows.Err(); err != nil {
        writeJSONError(w, http.StatusInternalServerError, "Error iterating requests")
        return
    }

    json.NewEncoder(w).Encode(requests)
}

// GetFollowers - Get list of followers
func (uh *UserHandler) GetFollowers(w http.ResponseWriter, r *http.Request) {
    currentUserID, _ := middleware.GetUserID(r)
    userIDStr := r.URL.Query().Get("id")
    userID, err := strconv.ParseInt(userIDStr, 10, 64)
    if err != nil {
        writeJSONError(w, http.StatusBadRequest, "Invalid user ID")
        return
    }

    canView, err := uh.canViewAccount(currentUserID, userID)
    if err != nil {
        writeJSONError(w, http.StatusInternalServerError, "Failed to verify access")
        return
    }
    if !canView {
        writeJSONError(w, http.StatusForbidden, "This profile is private")
        return
    }

    limit, offset := parsePagination(r, 200, 500)
    rows, err := uh.DB.Query(
        `SELECT u.id, u.email, u.first_name, u.last_name, u.nickname, u.avatar_path, u.is_public,
                f.status, f.created_at
         FROM follows f
         JOIN users u ON f.follower_id = u.id
         WHERE f.following_id = ? AND f.status = 'accepted'
         ORDER BY f.created_at DESC
         LIMIT ? OFFSET ?`,
        userID, limit, offset,
    )

    if err != nil {
        writeJSONError(w, http.StatusInternalServerError, "Failed to get followers")
        return
    }
    defer rows.Close()

    followers := []map[string]interface{}{}
    for rows.Next() {
        var user models.User
        var status string
        var createdAt time.Time
        err := rows.Scan(
            &user.ID, &user.Email, &user.FirstName, &user.LastName, &user.Nickname,
            &user.AvatarPath, &user.IsPublic,
            &status, &createdAt,
        )
        if err != nil {
            continue
        }
        followers = append(followers, map[string]interface{}{
            "user":       user,
            "status":     status,
            "created_at": createdAt,
        })
    }

    if err = rows.Err(); err != nil {
        writeJSONError(w, http.StatusInternalServerError, "Error iterating followers")
        return
    }

    json.NewEncoder(w).Encode(followers)
}

// GetFollowing - Get list of users being followed
func (uh *UserHandler) GetFollowing(w http.ResponseWriter, r *http.Request) {
    currentUserID, _ := middleware.GetUserID(r)
    userIDStr := r.URL.Query().Get("id")
    userID, err := strconv.ParseInt(userIDStr, 10, 64)
    if err != nil {
        writeJSONError(w, http.StatusBadRequest, "Invalid user ID")
        return
    }

    canView, err := uh.canViewAccount(currentUserID, userID)
    if err != nil {
        writeJSONError(w, http.StatusInternalServerError, "Failed to verify access")
        return
    }
    if !canView {
        writeJSONError(w, http.StatusForbidden, "This profile is private")
        return
    }

    limit, offset := parsePagination(r, 200, 500)
    rows, err := uh.DB.Query(
        `SELECT u.id, u.email, u.first_name, u.last_name, u.nickname, u.avatar_path, u.is_public,
                f.status, f.created_at
         FROM follows f
         JOIN users u ON f.following_id = u.id
         WHERE f.follower_id = ? AND f.status = 'accepted'
         ORDER BY f.created_at DESC
         LIMIT ? OFFSET ?`,
        userID, limit, offset,
    )

    if err != nil {
        writeJSONError(w, http.StatusInternalServerError, "Failed to get following")
        return
    }
    defer rows.Close()

    following := []map[string]interface{}{}
    for rows.Next() {
        var user models.User
        var status string
        var createdAt time.Time
        err := rows.Scan(
            &user.ID, &user.Email, &user.FirstName, &user.LastName, &user.Nickname,
            &user.AvatarPath, &user.IsPublic,
            &status, &createdAt,
        )
        if err != nil {
            continue
        }
        following = append(following, map[string]interface{}{
            "user":       user,
            "status":     status,
            "created_at": createdAt,
        })
    }

    if err = rows.Err(); err != nil {
        writeJSONError(w, http.StatusInternalServerError, "Error iterating following")
        return
    }

    json.NewEncoder(w).Encode(following)
}
