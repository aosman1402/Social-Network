package handlers

import (
    "database/sql"

    "social-network3/pkg/models"
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
