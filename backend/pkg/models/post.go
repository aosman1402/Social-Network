package models

import (
    "time"
)

type Post struct {
    ID           int64      `json:"id"`
    UserID       int64      `json:"user_id"`
    Content      string     `json:"content"`
    ImagePath    *string    `json:"image_path"`
    PrivacyLevel string     `json:"privacy_level"` // public, almost_private, private
    GroupID      *int64     `json:"group_id"`
    CreatedAt    time.Time  `json:"created_at"`
    UpdatedAt    time.Time  `json:"updated_at"`
    User         *User      `json:"user,omitempty"`
    Comments     []Comment  `json:"comments,omitempty"`
    CommentCount int        `json:"comment_count"`
    LikeCount    int        `json:"like_count"`
    IsLiked      bool       `json:"is_liked"`
}

type Comment struct {
    ID         int64      `json:"id"`
    PostID     int64      `json:"post_id"`
    UserID     int64      `json:"user_id"`
    Content    string     `json:"content"`
    ImagePath  *string    `json:"image_path"`
    CreatedAt  time.Time  `json:"created_at"`
    UpdatedAt  time.Time  `json:"updated_at"`
    User       *User      `json:"user,omitempty"`
}