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

type UserHandler struct {
    DB *sql.DB
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

    // Check if current user is following this user
    var isFollowing bool
    if currentUserID != userID {
        var count int
        uh.DB.QueryRow(
            "SELECT COUNT(*) FROM follows WHERE follower_id = ? AND following_id = ? AND status = 'accepted'",
            currentUserID, userID,
        ).Scan(&count)
        isFollowing = count > 0
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

    if !isPublic {
        var followerName string
        uh.DB.QueryRow(
            "SELECT first_name || ' ' || last_name FROM users WHERE id = ?",
            followerID,
        ).Scan(&followerName)

        uh.DB.Exec(
            `INSERT INTO notifications (user_id, type, content, related_id, created_at)
             VALUES (?, 'follow_request', ?, ?, ?)`,
            req.UserID, followerName+" requested to follow you", followerID, time.Now(),
        )
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

        uh.DB.Exec(
            `INSERT INTO notifications (user_id, type, content, related_id, created_at)
             VALUES (?, 'follow_accepted', ?, ?, ?)`,
            req.FollowerID, userName+" accepted your follow request", userID, time.Now(),
        )
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
    userIDStr := r.URL.Query().Get("id")
    userID, err := strconv.ParseInt(userIDStr, 10, 64)
    if err != nil {
        http.Error(w, "Invalid user ID", http.StatusBadRequest)
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
    userIDStr := r.URL.Query().Get("id")
    userID, err := strconv.ParseInt(userIDStr, 10, 64)
    if err != nil {
        http.Error(w, "Invalid user ID", http.StatusBadRequest)
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
    userIDStr := r.URL.Query().Get("id")
    userID, err := strconv.ParseInt(userIDStr, 10, 64)
    if err != nil {
        http.Error(w, "Invalid user ID", http.StatusBadRequest)
        return
    }

    rows, err := uh.DB.Query(`
        SELECT p.id, p.user_id, p.content, p.image_path, p.privacy_level, p.group_id, 
               p.created_at, p.updated_at,
               u.id, u.email, u.first_name, u.last_name, u.nickname, u.avatar_path, u.is_public,
               (SELECT COUNT(*) FROM comments c WHERE c.post_id = p.id) as comment_count
        FROM posts p
        JOIN users u ON p.user_id = u.id
        WHERE p.user_id = ?
        ORDER BY p.created_at DESC
        LIMIT 50
    `, userID)

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
            "id":             post.ID,
            "user_id":        post.UserID,
            "content":        post.Content,
            "image_path":     post.ImagePath,
            "privacy_level":  post.PrivacyLevel,
            "group_id":       post.GroupID,
            "created_at":     post.CreatedAt,
            "updated_at":     post.UpdatedAt,
            "user":           user,
            "comment_count":  commentCount,
        })
    }

    if err = rows.Err(); err != nil {
        http.Error(w, "Error iterating posts", http.StatusInternalServerError)
        return
    }

    json.NewEncoder(w).Encode(posts)
}