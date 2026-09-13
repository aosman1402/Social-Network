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
        http.Error(w, "Failed to parse form: "+err.Error(), http.StatusBadRequest)
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
        http.Error(w, "Content is required", http.StatusBadRequest)
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
        http.Error(w, "Failed to create post: "+err.Error(), http.StatusInternalServerError)
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

    rows, err := ph.DB.Query(`
        SELECT p.id, p.user_id, p.content, p.image_path, p.privacy_level, p.group_id, p.created_at, p.updated_at,
               u.id, u.email, u.first_name, u.last_name, u.nickname, u.avatar_path, u.is_public,
               (SELECT COUNT(*) FROM comments WHERE post_id = p.id) as comment_count,
               (SELECT COUNT(*) FROM post_likes WHERE post_id = p.id) as like_count,
               (SELECT COUNT(*) FROM post_likes WHERE post_id = p.id AND user_id = ?) as is_liked,
               (SELECT COUNT(*) FROM saved_posts WHERE post_id = p.id AND user_id = ?) as is_saved
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
        LIMIT 50
    `, userID, userID, userID, userID, userID, userID)

    if err != nil {
        http.Error(w, "Failed to get posts: "+err.Error(), http.StatusInternalServerError)
        return
    }
    defer rows.Close()

    var posts []map[string]interface{}
    for rows.Next() {
        var post models.Post
        var user models.User
        var commentCount, likeCount int
        var isLiked, isSaved bool

        err := rows.Scan(
            &post.ID, &post.UserID, &post.Content, &post.ImagePath, &post.PrivacyLevel,
            &post.GroupID, &post.CreatedAt, &post.UpdatedAt,
            &user.ID, &user.Email, &user.FirstName, &user.LastName, &user.Nickname,
            &user.AvatarPath, &user.IsPublic,
            &commentCount, &likeCount, &isLiked, &isSaved,
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
            "like_count":    likeCount,
            "is_liked":      isLiked,
            "is_saved":      isSaved,
        })
    }

    if err = rows.Err(); err != nil {
        http.Error(w, "Error iterating posts", http.StatusInternalServerError)
        return
    }

    json.NewEncoder(w).Encode(posts)
}

// CreateComment - Create a comment
func (ph *PostHandler) CreateComment(w http.ResponseWriter, r *http.Request) {
    userID, _ := middleware.GetUserID(r)

    err := r.ParseMultipartForm(10 << 20)
    if err != nil {
        http.Error(w, "Failed to parse form", http.StatusBadRequest)
        return
    }

    content := r.FormValue("content")
    postIDStr := r.FormValue("post_id")

    if content == "" {
        http.Error(w, "Content is required", http.StatusBadRequest)
        return
    }

    postID, err := strconv.ParseInt(postIDStr, 10, 64)
    if err != nil {
        http.Error(w, "Invalid post ID", http.StatusBadRequest)
        return
    }

    // Check if post exists
    var postUserID int64
    err = ph.DB.QueryRow("SELECT user_id FROM posts WHERE id = ?", postID).Scan(&postUserID)
    if err != nil {
        http.Error(w, "Post not found", http.StatusNotFound)
        return
    }

    // Handle image upload for comment
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

    var result sql.Result
    if imagePath != "" {
        result, err = ph.DB.Exec(
            `INSERT INTO comments (post_id, user_id, content, image_path, created_at, updated_at)
             VALUES (?, ?, ?, ?, ?, ?)`,
            postID, userID, content, imagePath, time.Now(), time.Now(),
        )
    } else {
        result, err = ph.DB.Exec(
            `INSERT INTO comments (post_id, user_id, content, created_at, updated_at)
             VALUES (?, ?, ?, ?, ?)`,
            postID, userID, content, time.Now(), time.Now(),
        )
    }

    if err != nil {
        http.Error(w, "Failed to create comment: "+err.Error(), http.StatusInternalServerError)
        return
    }

    commentID, _ := result.LastInsertId()

    // Get user info
    var user models.User
    ph.DB.QueryRow(
        "SELECT id, email, first_name, last_name, nickname, avatar_path FROM users WHERE id = ?",
        userID,
    ).Scan(&user.ID, &user.Email, &user.FirstName, &user.LastName, &user.Nickname, &user.AvatarPath)

    comment := map[string]interface{}{
        "id":         commentID,
        "post_id":    postID,
        "user_id":    userID,
        "content":    content,
        "image_path": imagePath,
        "created_at": time.Now(),
        "user":       user,
    }

    // Send notification
    if postUserID != userID {
        ph.DB.Exec(
            `INSERT INTO notifications (user_id, type, content, related_id, created_at)
             VALUES (?, 'comment', ?, ?, ?)`,
            postUserID, user.FirstName+" commented on your post", commentID, time.Now(),
        )
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "message": "Comment created successfully",
        "comment": comment,
    })
}

// GetComments - Get comments for a post
func (ph *PostHandler) GetComments(w http.ResponseWriter, r *http.Request) {
    postIDStr := r.URL.Query().Get("post_id")
    postID, err := strconv.ParseInt(postIDStr, 10, 64)
    if err != nil {
        http.Error(w, "Invalid post ID", http.StatusBadRequest)
        return
    }

    rows, err := ph.DB.Query(
        `SELECT c.id, c.post_id, c.user_id, c.content, c.image_path, c.created_at, c.updated_at,
                u.id, u.email, u.first_name, u.last_name, u.nickname, u.avatar_path, u.is_public
         FROM comments c
         JOIN users u ON c.user_id = u.id
         WHERE c.post_id = ?
         ORDER BY c.created_at ASC`,
        postID,
    )
    if err != nil {
        http.Error(w, "Failed to get comments", http.StatusInternalServerError)
        return
    }
    defer rows.Close()

    var comments []map[string]interface{}
    for rows.Next() {
        var comment models.Comment
        var user models.User
        err := rows.Scan(
            &comment.ID, &comment.PostID, &comment.UserID, &comment.Content, &comment.ImagePath,
            &comment.CreatedAt, &comment.UpdatedAt,
            &user.ID, &user.Email, &user.FirstName, &user.LastName, &user.Nickname,
            &user.AvatarPath, &user.IsPublic,
        )
        if err != nil {
            continue
        }
        comments = append(comments, map[string]interface{}{
            "id":         comment.ID,
            "post_id":    comment.PostID,
            "user_id":    comment.UserID,
            "content":    comment.Content,
            "image_path": comment.ImagePath,
            "created_at": comment.CreatedAt,
            "updated_at": comment.UpdatedAt,
            "user":       user,
        })
    }

    if err = rows.Err(); err != nil {
        http.Error(w, "Error iterating comments", http.StatusInternalServerError)
        return
    }

    json.NewEncoder(w).Encode(comments)
}

// LikePost - Like or unlike a post
func (ph *PostHandler) LikePost(w http.ResponseWriter, r *http.Request) {
    userID, _ := middleware.GetUserID(r)
    postIDStr := r.URL.Query().Get("id")

    postID, err := strconv.ParseInt(postIDStr, 10, 64)
    if err != nil {
        http.Error(w, "Invalid post ID", http.StatusBadRequest)
        return
    }

    // Check if post exists
    var exists bool
    err = ph.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM posts WHERE id = ?)", postID).Scan(&exists)
    if err != nil || !exists {
        http.Error(w, "Post not found", http.StatusNotFound)
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
            http.Error(w, "Failed to unlike post", http.StatusInternalServerError)
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
        http.Error(w, "Failed to like post", http.StatusInternalServerError)
        return
    }

    // Send notification
    var postUserID int64
    ph.DB.QueryRow("SELECT user_id FROM posts WHERE id = ?", postID).Scan(&postUserID)
    if postUserID != userID {
        var userName string
        ph.DB.QueryRow("SELECT first_name FROM users WHERE id = ?", userID).Scan(&userName)
        ph.DB.Exec(
            `INSERT INTO notifications (user_id, type, content, related_id, created_at)
             VALUES (?, 'like', ?, ?, ?)`,
            postUserID, userName+" liked your post", postID, time.Now(),
        )
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
        http.Error(w, "Invalid post ID", http.StatusBadRequest)
        return
    }

    var exists bool
    err = ph.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM posts WHERE id = ?)", postID).Scan(&exists)
    if err != nil || !exists {
        http.Error(w, "Post not found", http.StatusNotFound)
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
            http.Error(w, "Failed to unsave post", http.StatusInternalServerError)
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
        http.Error(w, "Failed to save post", http.StatusInternalServerError)
        return
    }

    json.NewEncoder(w).Encode(map[string]interface{}{
        "message": "Post saved",
        "saved":   true,
    })
}

// DeletePost - Delete a post
func (ph *PostHandler) DeletePost(w http.ResponseWriter, r *http.Request) {
    userID, _ := middleware.GetUserID(r)

    postIDStr := r.URL.Query().Get("id")
    postID, err := strconv.ParseInt(postIDStr, 10, 64)
    if err != nil {
        http.Error(w, "Invalid post ID", http.StatusBadRequest)
        return
    }

    var ownerID int64
    err = ph.DB.QueryRow("SELECT user_id FROM posts WHERE id = ?", postID).Scan(&ownerID)
    if err != nil {
        http.Error(w, "Post not found", http.StatusNotFound)
        return
    }

    if ownerID != userID {
        http.Error(w, "You don't have permission to delete this post", http.StatusForbidden)
        return
    }

    var imagePath sql.NullString
    ph.DB.QueryRow("SELECT image_path FROM posts WHERE id = ?", postID).Scan(&imagePath)
    if imagePath.Valid {
        deleteUploadedImage(imagePath.String)
    }

    _, err = ph.DB.Exec("DELETE FROM posts WHERE id = ?", postID)
    if err != nil {
        http.Error(w, "Failed to delete post", http.StatusInternalServerError)
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
        http.Error(w, "Invalid post ID", http.StatusBadRequest)
        return
    }

    // Check if post exists and belongs to user
    var ownerID int64
    err = ph.DB.QueryRow("SELECT user_id FROM posts WHERE id = ?", postID).Scan(&ownerID)
    if err != nil {
        http.Error(w, "Post not found", http.StatusNotFound)
        return
    }

    if ownerID != userID {
        http.Error(w, "You don't have permission to update this post", http.StatusForbidden)
        return
    }

    // Parse form data
    err = r.ParseMultipartForm(10 << 20)
    if err != nil {
        http.Error(w, "Failed to parse form", http.StatusBadRequest)
        return
    }

    content := r.FormValue("content")
    privacyLevel := r.FormValue("privacy_level")

    if content == "" {
        http.Error(w, "Content is required", http.StatusBadRequest)
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
        http.Error(w, "Failed to update post: "+updateErr.Error(), http.StatusInternalServerError)
        return
    }

    json.NewEncoder(w).Encode(map[string]string{
        "message": "Post updated successfully",
    })
}
