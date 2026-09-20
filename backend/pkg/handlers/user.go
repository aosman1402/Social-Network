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

type UserHandler struct {
    DB  *sql.DB
    Hub *websocket.Hub
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

// GetProfile - Get user profile
func (uh *UserHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")

    currentUserID, _ := middleware.GetUserID(r)
    userIDStr := r.URL.Query().Get("id")

    if userIDStr == "" {
        writeJSONError(w, http.StatusBadRequest, "User ID is required")
        return
    }

    userID, err := strconv.ParseInt(userIDStr, 10, 64)
    if err != nil {
        writeJSONError(w, http.StatusBadRequest, "Invalid user ID")
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
            writeJSONError(w, http.StatusNotFound, "User not found")
        } else {
            log.Printf("GetProfile: failed to load user %d: %v", userID, err)
            writeJSONError(w, http.StatusInternalServerError, "Failed to load profile")
        }
        return
    }

    user.IsPublic = isPublic

    // Private accounts are only visible to themselves and their accepted
    // followers - everyone else gets a 403. The body still carries enough
    // (name, avatar, follow status) for the frontend to render an identity
    // card with a follow-request button, rather than a bare error with
    // nothing to do on it.
    if !isPublic && currentUserID != userID {
        accepted, err := uh.hasAcceptedFollow(currentUserID, userID)
        if err != nil {
            writeJSONError(w, http.StatusInternalServerError, "Failed to verify access")
            return
        }
        if !accepted {
            var isPending bool
            if currentUserID != 0 {
                var status string
                if err := uh.DB.QueryRow(
                    "SELECT status FROM follows WHERE follower_id = ? AND following_id = ?",
                    currentUserID, userID,
                ).Scan(&status); err == nil {
                    isPending = status == "pending"
                }
            }
            w.WriteHeader(http.StatusForbidden)
            json.NewEncoder(w).Encode(map[string]interface{}{
                "error":        "This profile is private",
                "private":      true,
                "id":           user.ID,
                "first_name":   user.FirstName,
                "last_name":    user.LastName,
                "avatar_path":  user.AvatarPath,
                "nickname":     user.Nickname,
                "is_public":    false,
                "is_following": false,
                "is_pending":   isPending,
            })
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
        writeJSONError(w, http.StatusBadRequest, "Invalid request body")
        return
    }

    // Read the current visibility before overwriting it, so a private ->
    // public transition can be detected below. canViewAccount grants full
    // view access to anyone once an account is public, regardless of
    // follow status - so a still-"pending" follow request left over from
    // when this account was private would otherwise silently gain full
    // access the moment it goes public, without the requester ever being
    // formally accepted, without a notification to either side, and
    // without ever counting towards followers_count or showing up in
    // GetFollowers (both of which only count status = 'accepted').
    var wasPublic bool
    uh.DB.QueryRow("SELECT is_public FROM users WHERE id = ?", userID).Scan(&wasPublic)

    _, err := uh.DB.Exec(
        `UPDATE users SET first_name = ?, last_name = ?, about_me = ?, is_public = ?, updated_at = ?
         WHERE id = ?`,
        updates.FirstName, updates.LastName, updates.AboutMe, updates.IsPublic,
        time.Now(), userID,
    )

    if err != nil {
        writeJSONError(w, http.StatusInternalServerError, "Failed to update profile")
        return
    }

    if !wasPublic && updates.IsPublic {
        uh.autoAcceptPendingFollowsOnGoingPublic(userID)
    }

    json.NewEncoder(w).Encode(map[string]string{
        "message": "Profile updated successfully",
    })
}

// autoAcceptPendingFollowsOnGoingPublic bulk-accepts every pending follow
// request against userID once their account switches from private to
// public - a public account has no concept of "pending" (a fresh follow
// request against a public account is accepted immediately, see
// FollowUser), so a request left dangling from when the account was
// private is brought in line with that the moment it goes public, rather
// than staying silently stuck in the follows table forever. Notifies each
// affected follower that their request was accepted, and the account
// owner that their profile is now public.
func (uh *UserHandler) autoAcceptPendingFollowsOnGoingPublic(userID int64) {
    rows, err := uh.DB.Query(
        "SELECT follower_id FROM follows WHERE following_id = ? AND status = 'pending'",
        userID,
    )
    if err != nil {
        log.Printf("autoAcceptPendingFollowsOnGoingPublic: failed to query pending follows for user %d: %v", userID, err)
        return
    }
    var followerIDs []int64
    for rows.Next() {
        var followerID int64
        if err := rows.Scan(&followerID); err == nil {
            followerIDs = append(followerIDs, followerID)
        }
    }
    rows.Close()

    if len(followerIDs) == 0 {
        uh.notifyProfileNowPublic(userID, 0)
        return
    }

    if _, err := uh.DB.Exec(
        "UPDATE follows SET status = 'accepted', updated_at = ? WHERE following_id = ? AND status = 'pending'",
        time.Now(), userID,
    ); err != nil {
        log.Printf("autoAcceptPendingFollowsOnGoingPublic: failed to accept pending follows for user %d: %v", userID, err)
        return
    }

    var ownerName string
    uh.DB.QueryRow(
        "SELECT first_name || ' ' || last_name FROM users WHERE id = ?", userID,
    ).Scan(&ownerName)

    for _, followerID := range followerIDs {
        content := ownerName + " accepted your follow request"
        createdAt := time.Now()
        result, err := uh.DB.Exec(
            `INSERT INTO notifications (user_id, type, content, related_id, created_at)
             VALUES (?, 'follow_accepted', ?, ?, ?)`,
            followerID, content, userID, createdAt,
        )
        if err != nil {
            continue
        }
        notificationID, _ := result.LastInsertId()
        uh.Hub.NotifyUser(followerID, map[string]interface{}{
            "id":         notificationID,
            "user_id":    followerID,
            "type":       "follow_accepted",
            "content":    content,
            "is_read":    false,
            "related_id": userID,
            "created_at": createdAt,
        })
    }

    uh.notifyProfileNowPublic(userID, len(followerIDs))
}

// notifyProfileNowPublic tells userID their profile switched to public,
// mentioning how many previously-pending follow requests (if any) were
// automatically approved as a result.
func (uh *UserHandler) notifyProfileNowPublic(userID int64, approvedCount int) {
    content := "Your profile is now public"
    if approvedCount > 0 {
        plural := "s"
        if approvedCount == 1 {
            plural = ""
        }
        content += ". " + strconv.Itoa(approvedCount) + " pending follow request" + plural + " " +
            map[bool]string{true: "was", false: "were"}[approvedCount == 1] + " automatically approved"
    }
    content += "."

    createdAt := time.Now()
    result, err := uh.DB.Exec(
        `INSERT INTO notifications (user_id, type, content, created_at)
         VALUES (?, 'profile_public', ?, ?)`,
        userID, content, createdAt,
    )
    if err != nil {
        return
    }
    notificationID, _ := result.LastInsertId()
    uh.Hub.NotifyUser(userID, map[string]interface{}{
        "id":         notificationID,
        "user_id":    userID,
        "type":       "profile_public",
        "content":    content,
        "is_read":    false,
        "created_at": createdAt,
    })
}

// UploadAvatar - Upload/replace the current user's profile picture
func (uh *UserHandler) UploadAvatar(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    userID, _ := middleware.GetUserID(r)

    if err := r.ParseMultipartForm(5 << 20); err != nil {
        writeJSONError(w, http.StatusBadRequest, "Failed to parse form")
        return
    }

    file, header, err := r.FormFile("avatar")
    if err != nil {
        writeJSONError(w, http.StatusBadRequest, "Avatar image is required")
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
        writeJSONError(w, http.StatusInternalServerError, "Failed to update avatar")
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

// GetUserPosts - Get posts by a specific user
func (uh *UserHandler) GetUserPosts(w http.ResponseWriter, r *http.Request) {
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

    limit, offset := parsePagination(r, 50, 100)

    // Even for a viewable (public, or accepted-follower) account, each post
    // carries its own visibility - mirrors the same rule GetPosts (the main
    // feed) applies, including that a group post is only shown here to a
    // member of that group, never to the profile's other visitors.
    rows, err := uh.DB.Query(`
        SELECT `+postRowColumns+`
        FROM posts p
        JOIN users u ON p.user_id = u.id
        WHERE p.user_id = ? AND `+postVisibilityClause+`
        ORDER BY p.created_at DESC
        LIMIT ? OFFSET ?
    `, currentUserID, currentUserID, userID, currentUserID, currentUserID, currentUserID, currentUserID, limit, offset)

    if err != nil {
        writeJSONError(w, http.StatusInternalServerError, "Failed to get user posts")
        return
    }
    defer rows.Close()

    var posts []map[string]interface{}
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
