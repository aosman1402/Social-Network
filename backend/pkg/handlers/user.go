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

    // data is embedded as a raw JSON object (json.RawMessage), not a
    // string - matching the "chat"/"ack" events broadcast by
    // websocket.SendPrivateMessage and handleGroupMessage. The frontend's
    // WebSocket handler parses the whole envelope in one pass and expects
    // this to be consistent across every event type.
    message, err := json.Marshal(map[string]interface{}{
        "type": "notification",
        "data": json.RawMessage(payload),
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

    json.NewEncoder(w).Encode(map[string]string{
        "message": "Profile updated successfully",
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
    // carries its own privacy_level - mirrors the same rule GetPosts (the
    // main feed) applies: public to everyone, almost_private to accepted
    // followers of the author, private/anything else to the author only.
    rows, err := uh.DB.Query(`
        SELECT `+postRowColumns+`
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
        LIMIT ? OFFSET ?
    `, currentUserID, currentUserID, userID, currentUserID, currentUserID, currentUserID, limit, offset)

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
