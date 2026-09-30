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
)

// CreateComment - Create a comment
func (ph *PostHandler) CreateComment(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.GetUserID(r)

	err := r.ParseMultipartForm(10 << 20)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "Failed to parse form")
		return
	}

	content := r.FormValue("content")
	postIDStr := r.FormValue("post_id")

	if msg := contentError(content, hasImageUpload(r)); msg != "" {
		writeJSONError(w, http.StatusBadRequest, msg)
		return
	}

	postID, err := strconv.ParseInt(postIDStr, 10, 64)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid post ID")
		return
	}

	// Check if post exists
	var postUserID int64
	err = ph.DB.QueryRow("SELECT user_id FROM posts WHERE id = ?", postID).Scan(&postUserID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "Post not found")
		return
	}

	canView, err := canViewPost(ph.DB, postID, userID)
	if err != nil || !canView {
		writeJSONError(w, http.StatusForbidden, "You cannot comment on this post")
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
		log.Printf("CreateComment: failed to insert comment: %v", err)
		writeJSONError(w, http.StatusInternalServerError, "Failed to create comment")
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

	// Send notification. related_id is the comment itself (so a click can
	// scroll to and highlight this specific comment); post_id is the post
	// it belongs to (so a click can navigate/scroll there first) - the two
	// are different rows, so both need to travel with the notification.
	if postUserID != userID {
		content := user.FirstName + " commented on your post"
		notifCreatedAt := time.Now()
		result, err := ph.DB.Exec(
			`INSERT INTO notifications (user_id, type, content, related_id, post_id, created_at)
             VALUES (?, 'comment', ?, ?, ?, ?)`,
			postUserID, content, commentID, postID, notifCreatedAt,
		)
		if err == nil {
			notificationID, _ := result.LastInsertId()
			ph.Hub.NotifyUser(postUserID, map[string]interface{}{
				"id":         notificationID,
				"user_id":    postUserID,
				"type":       "comment",
				"content":    content,
				"is_read":    false,
				"related_id": commentID,
				"post_id":    postID,
				"created_at": notifCreatedAt,
			})
		}
	}

	var commentCount int
	ph.DB.QueryRow("SELECT COUNT(*) FROM comments WHERE post_id = ?", postID).Scan(&commentCount)
	broadcastToPostViewers(ph.DB, ph.Hub, postID, "comment_created", map[string]interface{}{
		"post_id":       postID,
		"comment":       comment,
		"comment_count": commentCount,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Comment created successfully",
		"comment": comment,
	})
}

// GetComments - Get comments for a post
func (ph *PostHandler) GetComments(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.GetUserID(r)

	postIDStr := r.URL.Query().Get("post_id")
	postID, err := strconv.ParseInt(postIDStr, 10, 64)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid post ID")
		return
	}

	canView, err := canViewPost(ph.DB, postID, userID)
	if err != nil || !canView {
		writeJSONError(w, http.StatusForbidden, "You cannot view this post's comments")
		return
	}

	limit, offset := parsePagination(r, 200, 500)
	rows, err := ph.DB.Query(
		`SELECT c.id, c.post_id, c.user_id, c.content, c.image_path, c.created_at, c.updated_at,
                u.id, u.email, u.first_name, u.last_name, u.nickname, u.avatar_path, u.is_public
         FROM comments c
         JOIN users u ON c.user_id = u.id
         WHERE c.post_id = ?
         ORDER BY c.created_at ASC
         LIMIT ? OFFSET ?`,
		postID, limit, offset,
	)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to get comments")
		return
	}
	defer rows.Close()

	comments := []map[string]interface{}{}
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
		writeJSONError(w, http.StatusInternalServerError, "Error iterating comments")
		return
	}

	json.NewEncoder(w).Encode(comments)
}
