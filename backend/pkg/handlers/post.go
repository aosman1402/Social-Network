package handlers

import (
    "database/sql"
    "encoding/json"
    "log"
    "net/http"
    "strconv"
    "time"

    "social-network3/pkg/middleware"
    "social-network3/pkg/models"
)

type PostHandler struct {
    DB *sql.DB
}

// CreatePost - Create a new post with image
func (ph *PostHandler) CreatePost(w http.ResponseWriter, r *http.Request) {
    userID, _ := middleware.GetUserID(r)

    // ✅ Parse multipart form (important for image uploads)
    err := r.ParseMultipartForm(10 << 20) // 10MB max
    if err != nil {
        log.Printf("CreatePost: failed to parse form: %v", err)
        writeJSONError(w, http.StatusBadRequest, "Failed to parse form")
        return
    }

    // ✅ Get form values
    content := r.FormValue("content")
    privacyLevel := r.FormValue("privacy_level")
    groupIDStr := r.FormValue("group_id")

    // ✅ Debug logging
    log.Printf("Creating post for user %d", userID)
    log.Printf("Content: %s", content)
    log.Printf("Privacy: %s", privacyLevel)

    if content == "" {
        writeJSONError(w, http.StatusBadRequest, "Content is required")
        return
    }

    if privacyLevel == "" {
        privacyLevel = "public"
    }

    var groupID *int64
    if groupIDStr != "" {
        id, err := strconv.ParseInt(groupIDStr, 10, 64)
        if err == nil {
            groupID = &id
        }
    }

    // Handle image upload
    var imagePath string
    file, header, err := r.FormFile("image")
    if err == nil {
        defer file.Close()

        imagePath, err = saveUploadedImage(file, header, 5*1024*1024)
        if err != nil {
            imageUploadErrorResponse(w, err)
            return
        }
    }

    // Insert post
    var result sql.Result
    if imagePath != "" {
        result, err = ph.DB.Exec(
            `INSERT INTO posts (user_id, content, image_path, privacy_level, group_id, created_at, updated_at)
             VALUES (?, ?, ?, ?, ?, ?, ?)`,
            userID, content, imagePath, privacyLevel, groupID, time.Now(), time.Now(),
        )
    } else {
        result, err = ph.DB.Exec(
            `INSERT INTO posts (user_id, content, privacy_level, group_id, created_at, updated_at)
             VALUES (?, ?, ?, ?, ?, ?)`,
            userID, content, privacyLevel, groupID, time.Now(), time.Now(),
        )
    }

    if err != nil {
        log.Printf("CreatePost: failed to insert post: %v", err)
        writeJSONError(w, http.StatusInternalServerError, "Failed to create post")
        return
    }

    postID, _ := result.LastInsertId()

    // For a 'private' post, the author picks specific viewers from their
    // followers - stored as an explicit allowlist. Re-check each chosen id
    // is actually an accepted follower server-side (never trust the form
    // alone) rather than letting the author name arbitrary user ids.
    var allowedUserIDs []int64
    if privacyLevel == "private" {
        for _, idStr := range r.Form["allowed_user_ids"] {
            id, err := strconv.ParseInt(idStr, 10, 64)
            if err != nil {
                continue
            }
            var isFollower bool
            ph.DB.QueryRow(
                "SELECT EXISTS(SELECT 1 FROM follows WHERE follower_id = ? AND following_id = ? AND status = 'accepted')",
                id, userID,
            ).Scan(&isFollower)
            if isFollower {
                allowedUserIDs = append(allowedUserIDs, id)
            }
        }
        for _, id := range allowedUserIDs {
            ph.DB.Exec(
                `INSERT INTO post_allowed_users (post_id, user_id, created_at) VALUES (?, ?, ?)
                 ON CONFLICT(post_id, user_id) DO NOTHING`,
                postID, id, time.Now(),
            )
        }
    }

    // Get user info
    var user models.User
    ph.DB.QueryRow(
        "SELECT id, email, first_name, last_name, nickname, avatar_path FROM users WHERE id = ?",
        userID,
    ).Scan(&user.ID, &user.Email, &user.FirstName, &user.LastName, &user.Nickname, &user.AvatarPath)

    var commentCount int
    ph.DB.QueryRow("SELECT COUNT(*) FROM comments WHERE post_id = ?", postID).Scan(&commentCount)
    var likeCount int
    ph.DB.QueryRow("SELECT COUNT(*) FROM post_likes WHERE post_id = ?", postID).Scan(&likeCount)

    post := map[string]interface{}{
        "id":               postID,
        "user_id":          userID,
        "content":          content,
        "image_path":       imagePath,
        "privacy_level":    privacyLevel,
        "group_id":         groupID,
        "allowed_user_ids": allowedUserIDs,
        "created_at":       time.Now(),
        "user":             user,
        "comment_count":    commentCount,
        "like_count":       likeCount,
        "is_liked":         false,
        "is_saved":         false,
    }

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusCreated)
    json.NewEncoder(w).Encode(map[string]interface{}{
        "message": "Post created successfully",
        "post_id": postID,
        "post":    post,
    })
}

// GetPosts - Get posts for feed
func (ph *PostHandler) GetPosts(w http.ResponseWriter, r *http.Request) {
    userID, _ := middleware.GetUserID(r)
    limit, offset := parsePagination(r, 50, 100)

    rows, err := ph.DB.Query(`
        SELECT `+postRowColumns+`
        FROM posts p
        JOIN users u ON p.user_id = u.id
        WHERE p.privacy_level = 'public'
           OR (p.privacy_level = 'almost_private' AND p.user_id IN (
               SELECT following_id FROM follows WHERE follower_id = ? AND status = 'accepted'
           ))
           OR (p.privacy_level = 'private' AND p.id IN (
               SELECT post_id FROM post_allowed_users WHERE user_id = ?
           ))
           OR p.user_id = ?
           OR (p.group_id IS NOT NULL AND p.group_id IN (
               SELECT group_id FROM group_members WHERE user_id = ? AND status = 'active'
           ))
        ORDER BY p.created_at DESC
        LIMIT ? OFFSET ?
    `, userID, userID, userID, userID, userID, userID, limit, offset)

    if err != nil {
        log.Printf("GetPosts: failed to query posts: %v", err)
        writeJSONError(w, http.StatusInternalServerError, "Failed to get posts")
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

// DeletePost - Delete a post
func (ph *PostHandler) DeletePost(w http.ResponseWriter, r *http.Request) {
    userID, _ := middleware.GetUserID(r)

    postIDStr := r.URL.Query().Get("id")
    postID, err := strconv.ParseInt(postIDStr, 10, 64)
    if err != nil {
        writeJSONError(w, http.StatusBadRequest, "Invalid post ID")
        return
    }

    var ownerID int64
    err = ph.DB.QueryRow("SELECT user_id FROM posts WHERE id = ?", postID).Scan(&ownerID)
    if err != nil {
        writeJSONError(w, http.StatusNotFound, "Post not found")
        return
    }

    if ownerID != userID {
        writeJSONError(w, http.StatusForbidden, "You don't have permission to delete this post")
        return
    }

    var imagePath sql.NullString
    ph.DB.QueryRow("SELECT image_path FROM posts WHERE id = ?", postID).Scan(&imagePath)
    if imagePath.Valid {
        deleteUploadedImage(imagePath.String)
    }

    _, err = ph.DB.Exec("DELETE FROM posts WHERE id = ?", postID)
    if err != nil {
        writeJSONError(w, http.StatusInternalServerError, "Failed to delete post")
        return
    }

    json.NewEncoder(w).Encode(map[string]string{
        "message": "Post deleted successfully",
    })
}

// UpdatePost - Update an existing post
func (ph *PostHandler) UpdatePost(w http.ResponseWriter, r *http.Request) {
    userID, _ := middleware.GetUserID(r)

    postIDStr := r.URL.Query().Get("id")
    postID, err := strconv.ParseInt(postIDStr, 10, 64)
    if err != nil {
        writeJSONError(w, http.StatusBadRequest, "Invalid post ID")
        return
    }

    // Check if post exists and belongs to user
    var ownerID int64
    err = ph.DB.QueryRow("SELECT user_id FROM posts WHERE id = ?", postID).Scan(&ownerID)
    if err != nil {
        writeJSONError(w, http.StatusNotFound, "Post not found")
        return
    }

    if ownerID != userID {
        writeJSONError(w, http.StatusForbidden, "You don't have permission to update this post")
        return
    }

    // Parse form data
    err = r.ParseMultipartForm(10 << 20)
    if err != nil {
        writeJSONError(w, http.StatusBadRequest, "Failed to parse form")
        return
    }

    content := r.FormValue("content")
    privacyLevel := r.FormValue("privacy_level")

    if content == "" {
        writeJSONError(w, http.StatusBadRequest, "Content is required")
        return
    }

    if privacyLevel == "" {
        privacyLevel = "public"
    }

    // Handle image update
    var imagePath string
    file, header, err := r.FormFile("image")
    if err == nil {
        defer file.Close()

        imagePath, err = saveUploadedImage(file, header, 5*1024*1024)
        if err != nil {
            imageUploadErrorResponse(w, err)
            return
        }

        // Delete old image now that the new one is safely saved
        var oldImagePath sql.NullString
        ph.DB.QueryRow("SELECT image_path FROM posts WHERE id = ?", postID).Scan(&oldImagePath)
        if oldImagePath.Valid {
            deleteUploadedImage(oldImagePath.String)
        }
    }

    // Update post
    var updateErr error
    if imagePath != "" {
        _, updateErr = ph.DB.Exec(
            `UPDATE posts SET content = ?, image_path = ?, privacy_level = ?, updated_at = ?
             WHERE id = ?`,
            content, imagePath, privacyLevel, time.Now(), postID,
        )
    } else {
        _, updateErr = ph.DB.Exec(
            `UPDATE posts SET content = ?, privacy_level = ?, updated_at = ?
             WHERE id = ?`,
            content, privacyLevel, time.Now(), postID,
        )
    }

    if updateErr != nil {
        writeJSONError(w, http.StatusInternalServerError, "Failed to update post: "+updateErr.Error())
        return
    }

    json.NewEncoder(w).Encode(map[string]string{
        "message": "Post updated successfully",
    })
}
