package handlers

import (
    "encoding/json"
    "net/http"
    "strconv"
    "time"

    "social-network/pkg/middleware"
)

// LikePost - Like or unlike a post
func (ph *PostHandler) LikePost(w http.ResponseWriter, r *http.Request) {
    userID, _ := middleware.GetUserID(r)
    postIDStr := r.URL.Query().Get("id")

    postID, err := strconv.ParseInt(postIDStr, 10, 64)
    if err != nil {
        writeJSONError(w, http.StatusBadRequest, "Invalid post ID")
        return
    }

    // Check if post exists
    var exists bool
    err = ph.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM posts WHERE id = ?)", postID).Scan(&exists)
    if err != nil || !exists {
        writeJSONError(w, http.StatusNotFound, "Post not found")
        return
    }

    // Check if already liked
    var likeID int64
    err = ph.DB.QueryRow(
        "SELECT id FROM post_likes WHERE post_id = ? AND user_id = ?",
        postID, userID,
    ).Scan(&likeID)

    if err == nil {
        // Unlike
        _, err = ph.DB.Exec("DELETE FROM post_likes WHERE id = ?", likeID)
        if err != nil {
            writeJSONError(w, http.StatusInternalServerError, "Failed to unlike post")
            return
        }
        var likeCount int
        ph.DB.QueryRow("SELECT COUNT(*) FROM post_likes WHERE post_id = ?", postID).Scan(&likeCount)

        // ✅ Return JSON response
        json.NewEncoder(w).Encode(map[string]interface{}{
            "message":    "Post unliked",
            "liked":      false,
            "like_count": likeCount,
        })
        return
    }

    // Like
    _, err = ph.DB.Exec(
        "INSERT INTO post_likes (post_id, user_id, created_at) VALUES (?, ?, ?)",
        postID, userID, time.Now(),
    )
    if err != nil {
        writeJSONError(w, http.StatusInternalServerError, "Failed to like post")
        return
    }

    // Send notification
    var postUserID int64
    ph.DB.QueryRow("SELECT user_id FROM posts WHERE id = ?", postID).Scan(&postUserID)
    if postUserID != userID {
        var userName string
        ph.DB.QueryRow("SELECT first_name FROM users WHERE id = ?", userID).Scan(&userName)
        content := userName + " liked your post"
        createdAt := time.Now()
        result, err := ph.DB.Exec(
            `INSERT INTO notifications (user_id, type, content, related_id, post_id, created_at)
             VALUES (?, 'like', ?, ?, ?, ?)`,
            postUserID, content, postID, postID, createdAt,
        )
        if err == nil {
            notificationID, _ := result.LastInsertId()
            ph.Hub.NotifyUser(postUserID, map[string]interface{}{
                "id":         notificationID,
                "user_id":    postUserID,
                "type":       "like",
                "content":    content,
                "is_read":    false,
                "related_id": postID,
                "post_id":    postID,
                "created_at": createdAt,
            })
        }
    }

    var likeCount int
    ph.DB.QueryRow("SELECT COUNT(*) FROM post_likes WHERE post_id = ?", postID).Scan(&likeCount)

    // ✅ Return JSON response
    json.NewEncoder(w).Encode(map[string]interface{}{
        "message":    "Post liked",
        "liked":      true,
        "like_count": likeCount,
    })
}

// SavePost - Save or unsave a post
func (ph *PostHandler) SavePost(w http.ResponseWriter, r *http.Request) {
    userID, _ := middleware.GetUserID(r)
    postIDStr := r.URL.Query().Get("id")

    postID, err := strconv.ParseInt(postIDStr, 10, 64)
    if err != nil {
        writeJSONError(w, http.StatusBadRequest, "Invalid post ID")
        return
    }

    var exists bool
    err = ph.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM posts WHERE id = ?)", postID).Scan(&exists)
    if err != nil || !exists {
        writeJSONError(w, http.StatusNotFound, "Post not found")
        return
    }

    var saveID int64
    err = ph.DB.QueryRow(
        "SELECT id FROM saved_posts WHERE post_id = ? AND user_id = ?",
        postID, userID,
    ).Scan(&saveID)

    if err == nil {
        _, err = ph.DB.Exec("DELETE FROM saved_posts WHERE id = ?", saveID)
        if err != nil {
            writeJSONError(w, http.StatusInternalServerError, "Failed to unsave post")
            return
        }
        json.NewEncoder(w).Encode(map[string]interface{}{
            "message": "Post unsaved",
            "saved":   false,
        })
        return
    }

    _, err = ph.DB.Exec(
        "INSERT INTO saved_posts (post_id, user_id, created_at) VALUES (?, ?, ?)",
        postID, userID, time.Now(),
    )
    if err != nil {
        writeJSONError(w, http.StatusInternalServerError, "Failed to save post")
        return
    }

    json.NewEncoder(w).Encode(map[string]interface{}{
        "message": "Post saved",
        "saved":   true,
    })
}
