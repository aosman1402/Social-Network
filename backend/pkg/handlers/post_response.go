package handlers

import (
	"database/sql"

	"social-network/pkg/models"
)

// postRowColumns is the exact column list every "list of posts" query
// (GetPosts, GetUserPosts, GetGroupPosts) must select, in this order, so
// scanPostRow can scan any of them. The trailing four are always the same
// four correlated subqueries:
//
//	(SELECT COUNT(*) FROM comments WHERE post_id = p.id) as comment_count,
//	(SELECT COUNT(*) FROM post_likes WHERE post_id = p.id) as like_count,
//	(SELECT COUNT(*) FROM post_likes WHERE post_id = p.id AND user_id = ?) as is_liked,
//	(SELECT COUNT(*) FROM saved_posts WHERE post_id = p.id AND user_id = ?) as is_saved
const postRowColumns = `p.id, p.user_id, p.content, p.image_path, p.privacy_level, p.group_id,
               p.created_at, p.updated_at,
               u.id, u.email, u.first_name, u.last_name, u.nickname, u.avatar_path, u.is_public,
               (SELECT COUNT(*) FROM comments WHERE post_id = p.id) as comment_count,
               (SELECT COUNT(*) FROM post_likes WHERE post_id = p.id) as like_count,
               (SELECT COUNT(*) FROM post_likes WHERE post_id = p.id AND user_id = ?) as is_liked,
               (SELECT COUNT(*) FROM saved_posts WHERE post_id = p.id AND user_id = ?) as is_saved`

// scanPostRow scans one row shaped by postRowColumns into the response map
// every post-list endpoint returns. Centralizing this means GetPosts,
// GetUserPosts, and GetGroupPosts can't silently drift out of sync with
// each other on which fields a post carries - they previously did (only
// GetPosts included like_count/is_liked/is_saved, so post cards on a
// profile or group page silently lost their like/save state).
func scanPostRow(rows *sql.Rows) (map[string]interface{}, error) {
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
		return nil, err
	}

	return map[string]interface{}{
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
	}, nil
}

// postVisibilityClause is the visibility rule shared by every "can this
// user see this post" check (GetPosts, GetPost, GetUserPosts, canViewPost).
// A group post (group_id set) is visible ONLY to the author or an active
// member of that group - its privacy_level is irrelevant and never
// consulted, since CreatePost defaults an unset privacy_level to "public"
// and that clause would otherwise make every group post readable by
// everyone, defeating the point of it belonging to a group. A non-group
// post keeps the ordinary privacy_level-based rule (public, almost_private
// to an accepted follower, private to a named allowed_user, or the
// author). Expects, in order: userID (author check), userID (group
// membership), userID (almost_private follower check), userID (private
// allowlist check).
const postVisibilityClause = `(
    p.user_id = ?
    OR (p.group_id IS NOT NULL AND p.group_id IN (
        SELECT group_id FROM group_members WHERE user_id = ? AND status = 'active'
    ))
    OR (p.group_id IS NULL AND (
        p.privacy_level = 'public'
        OR (p.privacy_level = 'almost_private' AND p.user_id IN (
            SELECT following_id FROM follows WHERE follower_id = ? AND status = 'accepted'
        ))
        OR (p.privacy_level = 'private' AND p.id IN (
            SELECT post_id FROM post_allowed_users WHERE user_id = ?
        ))
    ))
)`

// canViewPost applies postVisibilityClause to a single post - shared so
// comment endpoints can't be used to read or write comments on a post the
// caller isn't otherwise allowed to see.
func canViewPost(db *sql.DB, postID, userID int64) (bool, error) {
	var canView bool
	err := db.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM posts p WHERE p.id = ? AND `+postVisibilityClause+`)`,
		postID, userID, userID, userID, userID,
	).Scan(&canView)
	return canView, err
}
