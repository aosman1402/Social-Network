package handlers

import (
    "database/sql"
    "encoding/json"
    "net/http"
    "strconv"
    "time"

    "social-network3/pkg/middleware"
    "social-network3/pkg/models"
    "social-network3/pkg/websocket"
)

type UserHandler struct {
    DB  *sql.DB
    Hub *websocket.Hub
}

// notifyUser pushes a notification to userID's live websocket connection(s),
// if any are open, so the frontend can show it immediately instead of
// waiting for the next page load/poll of /api/notifications. Hub is nil in
// contexts (e.g. tests) that don't wire one up, and BroadcastToRoom is a
// no-op when nobody is connected to that room, so this is always safe to call.
func (uh *UserHandler) notifyUser(userID int64, notification map[string]interface{}) {
    if uh.Hub == nil {
        return
    }

    payload, err := json.Marshal(notification)
    if err != nil {
        return
    }

    message, err := json.Marshal(map[string]interface{}{
        "type": "notification",
        "data": string(payload),
    })
    if err != nil {
        return
    }

    uh.Hub.BroadcastToRoom("user_"+strconv.FormatInt(userID, 10), message)
}

// hasAcceptedFollow reports whether followerID has an accepted follow of
// followingID.
func (uh *UserHandler) hasAcceptedFollow(followerID, followingID int64) (bool, error) {
    var status string
    err := uh.DB.QueryRow(
        "SELECT status FROM follows WHERE follower_id = ? AND following_id = ?",
        followerID, followingID,
    ).Scan(&status)
    if err == sql.ErrNoRows {
        return false, nil
    }
    if err != nil {
        return false, err
    }
    return status == "accepted", nil
}

// canViewAccount reports whether viewerID may see targetID's private
// account details - profile fields, followers/following lists, and
// non-public posts. True for the account owner, anyone viewing a public
// account, or an accepted follower of a private one.
func (uh *UserHandler) canViewAccount(viewerID, targetID int64) (bool, error) {
    if viewerID == targetID {
        return true, nil
    }

    var isPublic bool
    if err := uh.DB.QueryRow(
        "SELECT is_public FROM users WHERE id = ?", targetID,
    ).Scan(&isPublic); err != nil {
        return false, err
    }
    if isPublic {
        return true, nil
    }

    return uh.hasAcceptedFollow(viewerID, targetID)
}

type UserProfile struct {
    models.User
    FollowersCount int  `json:"followers_count"`
    FollowingCount int  `json:"following_count"`
    IsFollowing    bool `json:"is_following"`
    IsFollowedBy   bool `json:"is_followed_by"`
    PostsCount     int  `json:"posts_count"`
}

// GetProfile - Get user profile
func (uh *UserHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")

    currentUserID, _ := middleware.GetUserID(r)
    userIDStr := r.URL.Query().Get("id")

    if userIDStr == "" {
        http.Error(w, "User ID is required", http.StatusBadRequest)
        return
    }

    userID, err := strconv.ParseInt(userIDStr, 10, 64)
    if err != nil {
        http.Error(w, "Invalid user ID", http.StatusBadRequest)
        return
    }

    var user models.User
    var isPublic bool

    err = uh.DB.QueryRow(`
        SELECT id, email, first_name, last_name, date_of_birth, 
               avatar_path, nickname, about_me, is_public, 
               created_at, updated_at
        FROM users 
        WHERE id = ?`,
        userID,
    ).Scan(
        &user.ID,
        &user.Email,
        &user.FirstName,
        &user.LastName,
        &user.DateOfBirth,
        &user.AvatarPath,
        &user.Nickname,
        &user.AboutMe,
        &isPublic,
        &user.CreatedAt,
        &user.UpdatedAt,
    )

    if err != nil {
        if err == sql.ErrNoRows {
            http.Error(w, "User not found", http.StatusNotFound)
        } else {
            http.Error(w, err.Error(), http.StatusInternalServerError)
        }
        return
    }

    user.IsPublic = isPublic

    // Private accounts are only visible to themselves and their accepted
    // followers - everyone else gets a 403, matching the frontend's
    // existing (previously unreachable) "This profile is private" handling.
    if !isPublic && currentUserID != userID {
        accepted, err := uh.hasAcceptedFollow(currentUserID, userID)
        if err != nil {
            http.Error(w, "Failed to verify access", http.StatusInternalServerError)
            return
        }
        if !accepted {
            http.Error(w, "This profile is private", http.StatusForbidden)
            return
        }
    }

    // Get followers count
    var followersCount int
    uh.DB.QueryRow(
        "SELECT COUNT(*) FROM follows WHERE following_id = ? AND status = 'accepted'",
        userID,
    ).Scan(&followersCount)

    // Get following count
    var followingCount int
    uh.DB.QueryRow(
        "SELECT COUNT(*) FROM follows WHERE follower_id = ? AND status = 'accepted'",
        userID,
    ).Scan(&followingCount)

    // Get posts count
    var postsCount int
    uh.DB.QueryRow(
        "SELECT COUNT(*) FROM posts WHERE user_id = ?",
        userID,
    ).Scan(&postsCount)

    // Check if current user is following this user, or has a follow
    // request pending on them (private accounts require acceptance).
    var isFollowing, isPending bool
    if currentUserID != userID {
        var status string
        err := uh.DB.QueryRow(
            "SELECT status FROM follows WHERE follower_id = ? AND following_id = ?",
            currentUserID, userID,
        ).Scan(&status)
        if err == nil {
            isFollowing = status == "accepted"
            isPending = status == "pending"
        }
    }

    response := map[string]interface{}{
        "id":              user.ID,
        "email":           user.Email,
        "first_name":      user.FirstName,
        "last_name":       user.LastName,
        "date_of_birth":   user.DateOfBirth,
        "avatar_path":     user.AvatarPath,
        "nickname":        user.Nickname,
        "about_me":        user.AboutMe,
        "is_public":       user.IsPublic,
        "created_at":      user.CreatedAt,
        "updated_at":      user.UpdatedAt,
        "followers_count": followersCount,
        "following_count": followingCount,
        "posts_count":     postsCount,
        "is_following":    isFollowing,
        "is_pending":      isPending,
        "is_followed_by":  false,
    }

    json.NewEncoder(w).Encode(response)
}

// UpdateProfile - Update user profile
func (uh *UserHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
    userID, _ := middleware.GetUserID(r)

    var updates struct {
        FirstName string `json:"first_name"`
        LastName  string `json:"last_name"`
        AboutMe   string `json:"about_me"`
        IsPublic  bool   `json:"is_public"`
    }

    if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }

    _, err := uh.DB.Exec(
        `UPDATE users SET first_name = ?, last_name = ?, about_me = ?, is_public = ?, updated_at = ?
         WHERE id = ?`,
        updates.FirstName, updates.LastName, updates.AboutMe, updates.IsPublic,
        time.Now(), userID,
    )

    if err != nil {
        http.Error(w, "Failed to update profile", http.StatusInternalServerError)
        return
    }

    json.NewEncoder(w).Encode(map[string]string{
        "message": "Profile updated successfully",
    })
}

// UploadAvatar - Upload/replace the current user's profile picture
func (uh *UserHandler) UploadAvatar(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    userID, _ := middleware.GetUserID(r)

    if err := r.ParseMultipartForm(5 << 20); err != nil {
        http.Error(w, "Failed to parse form", http.StatusBadRequest)
        return
    }

    file, header, err := r.FormFile("avatar")
    if err != nil {
        http.Error(w, "Avatar image is required", http.StatusBadRequest)
        return
    }
    defer file.Close()

    avatarPath, err := saveUploadedImage(file, header, 5*1024*1024)
    if err != nil {
        imageUploadErrorResponse(w, err)
        return
    }

    var oldAvatarPath sql.NullString
    uh.DB.QueryRow("SELECT avatar_path FROM users WHERE id = ?", userID).Scan(&oldAvatarPath)

    if _, err := uh.DB.Exec(
        "UPDATE users SET avatar_path = ?, updated_at = ? WHERE id = ?",
        avatarPath, time.Now(), userID,
    ); err != nil {
        http.Error(w, "Failed to update avatar", http.StatusInternalServerError)
        return
    }

    if oldAvatarPath.Valid {
        deleteUploadedImage(oldAvatarPath.String)
    }

    json.NewEncoder(w).Encode(map[string]string{
        "message":     "Avatar updated successfully",
        "avatar_path": avatarPath,
    })
}

// FollowUser - Follow a user
func (uh *UserHandler) FollowUser(w http.ResponseWriter, r *http.Request) {
    followerID, _ := middleware.GetUserID(r)

    var req struct {
        UserID int64 `json:"user_id"`
    }

    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }

    if followerID == req.UserID {
        http.Error(w, "Cannot follow yourself", http.StatusBadRequest)
        return
    }

    var isPublic bool
    err := uh.DB.QueryRow(
        "SELECT is_public FROM users WHERE id = ?",
        req.UserID,
    ).Scan(&isPublic)

    if err != nil {
        http.Error(w, "User not found", http.StatusNotFound)
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
        http.Error(w, "Failed to follow user", http.StatusInternalServerError)
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
        uh.notifyUser(req.UserID, map[string]interface{}{
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
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }

    if followerID == req.UserID {
        http.Error(w, "Cannot unfollow yourself", http.StatusBadRequest)
        return
    }

    result, err := uh.DB.Exec(
        "DELETE FROM follows WHERE follower_id = ? AND following_id = ?",
        followerID, req.UserID,
    )

    if err != nil {
        http.Error(w, "Failed to unfollow user", http.StatusInternalServerError)
        return
    }

    rowsAffected, _ := result.RowsAffected()
    if rowsAffected == 0 {
        http.Error(w, "Not following this user", http.StatusBadRequest)
        return
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
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }

    if req.Action != "accept" && req.Action != "decline" {
        http.Error(w, "Invalid action. Must be 'accept' or 'decline'", http.StatusBadRequest)
        return
    }

    var count int
    err := uh.DB.QueryRow(
        "SELECT COUNT(*) FROM follows WHERE follower_id = ? AND following_id = ? AND status = 'pending'",
        req.FollowerID, userID,
    ).Scan(&count)

    if err != nil || count == 0 {
        http.Error(w, "No pending follow request found", http.StatusNotFound)
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
        http.Error(w, "Failed to update follow request", http.StatusInternalServerError)
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
            uh.notifyUser(req.FollowerID, map[string]interface{}{
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

    rows, err := uh.DB.Query(
        `SELECT f.id, f.follower_id, f.status, f.created_at,
                u.id, u.email, u.first_name, u.last_name, u.nickname, u.avatar_path, u.is_public
         FROM follows f
         JOIN users u ON f.follower_id = u.id
         WHERE f.following_id = ? AND f.status = 'pending'
         ORDER BY f.created_at DESC`,
        userID,
    )

    if err != nil {
        http.Error(w, "Failed to get follow requests", http.StatusInternalServerError)
        return
    }
    defer rows.Close()

    var requests []map[string]interface{}
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
        http.Error(w, "Error iterating requests", http.StatusInternalServerError)
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
        http.Error(w, "Invalid user ID", http.StatusBadRequest)
        return
    }

    canView, err := uh.canViewAccount(currentUserID, userID)
    if err != nil {
        http.Error(w, "Failed to verify access", http.StatusInternalServerError)
        return
    }
    if !canView {
        http.Error(w, "This profile is private", http.StatusForbidden)
        return
    }

    rows, err := uh.DB.Query(
        `SELECT u.id, u.email, u.first_name, u.last_name, u.nickname, u.avatar_path, u.is_public,
                f.status, f.created_at
         FROM follows f
         JOIN users u ON f.follower_id = u.id
         WHERE f.following_id = ? AND f.status = 'accepted'
         ORDER BY f.created_at DESC`,
        userID,
    )

    if err != nil {
        http.Error(w, "Failed to get followers", http.StatusInternalServerError)
        return
    }
    defer rows.Close()

    var followers []map[string]interface{}
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
        http.Error(w, "Error iterating followers", http.StatusInternalServerError)
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
        http.Error(w, "Invalid user ID", http.StatusBadRequest)
        return
    }

    canView, err := uh.canViewAccount(currentUserID, userID)
    if err != nil {
        http.Error(w, "Failed to verify access", http.StatusInternalServerError)
        return
    }
    if !canView {
        http.Error(w, "This profile is private", http.StatusForbidden)
        return
    }

    rows, err := uh.DB.Query(
        `SELECT u.id, u.email, u.first_name, u.last_name, u.nickname, u.avatar_path, u.is_public,
                f.status, f.created_at
         FROM follows f
         JOIN users u ON f.following_id = u.id
         WHERE f.follower_id = ? AND f.status = 'accepted'
         ORDER BY f.created_at DESC`,
        userID,
    )

    if err != nil {
        http.Error(w, "Failed to get following", http.StatusInternalServerError)
        return
    }
    defer rows.Close()

    var following []map[string]interface{}
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
        http.Error(w, "Error iterating following", http.StatusInternalServerError)
        return
    }

    json.NewEncoder(w).Encode(following)
}

// GetUserPosts - Get posts by a specific user
func (uh *UserHandler) GetUserPosts(w http.ResponseWriter, r *http.Request) {
    currentUserID, _ := middleware.GetUserID(r)
    userIDStr := r.URL.Query().Get("id")
    userID, err := strconv.ParseInt(userIDStr, 10, 64)
    if err != nil {
        http.Error(w, "Invalid user ID", http.StatusBadRequest)
        return
    }

    canView, err := uh.canViewAccount(currentUserID, userID)
    if err != nil {
        http.Error(w, "Failed to verify access", http.StatusInternalServerError)
        return
    }
    if !canView {
        http.Error(w, "This profile is private", http.StatusForbidden)
        return
    }

    // Even for a viewable (public, or accepted-follower) account, each post
    // carries its own privacy_level - mirrors the same rule GetPosts (the
    // main feed) applies: public to everyone, almost_private to accepted
    // followers of the author, private/anything else to the author only.
    rows, err := uh.DB.Query(`
        SELECT p.id, p.user_id, p.content, p.image_path, p.privacy_level, p.group_id,
               p.created_at, p.updated_at,
               u.id, u.email, u.first_name, u.last_name, u.nickname, u.avatar_path, u.is_public,
               (SELECT COUNT(*) FROM comments c WHERE c.post_id = p.id) as comment_count
        FROM posts p
        JOIN users u ON p.user_id = u.id
        WHERE p.user_id = ?
          AND (
              p.privacy_level = 'public'
              OR p.user_id = ?
              OR (p.privacy_level = 'almost_private' AND EXISTS (
                  SELECT 1 FROM follows
                  WHERE follower_id = ? AND following_id = p.user_id AND status = 'accepted'
              ))
              OR (p.privacy_level = 'private' AND EXISTS (
                  SELECT 1 FROM post_allowed_users
                  WHERE post_id = p.id AND user_id = ?
              ))
          )
        ORDER BY p.created_at DESC
        LIMIT 50
    `, userID, currentUserID, currentUserID, currentUserID)

    if err != nil {
        http.Error(w, "Failed to get user posts", http.StatusInternalServerError)
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
        posts = append(posts, map[string]interface{}{
            "id":            post.ID,
            "user_id":       post.UserID,
            "content":       post.Content,
            "image_path":    post.ImagePath,
            "privacy_level": post.PrivacyLevel,
            "group_id":      post.GroupID,
            "created_at":    post.CreatedAt,
            "updated_at":    post.UpdatedAt,
            "user":          user,
            "comment_count": commentCount,
        })
    }

    if err = rows.Err(); err != nil {
        http.Error(w, "Error iterating posts", http.StatusInternalServerError)
        return
    }

    json.NewEncoder(w).Encode(posts)
}
