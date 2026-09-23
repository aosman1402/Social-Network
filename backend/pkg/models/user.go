package models

import (
	"time"
)

type User struct {
	ID                int64     `json:"id"`
	Email             string    `json:"email"`
	PasswordHash      string    `json:"-"`
	FirstName         string    `json:"first_name"`
	LastName          string    `json:"last_name"`
	DateOfBirth       time.Time `json:"date_of_birth"`
	AvatarPath        *string   `json:"avatar_path"`
	Nickname          *string   `json:"nickname"`
	AboutMe           *string   `json:"about_me"`
	IsPublic          bool      `json:"is_public"`
	EducationProvider *string   `json:"education_provider"`
	QualificationName *string   `json:"qualification_name"`
	RqfLevel          *string   `json:"rqf_level"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type UserProfile struct {
	User
	FollowersCount int  `json:"followers_count"`
	FollowingCount int  `json:"following_count"`
	IsFollowing    bool `json:"is_following"`
	IsFollowedBy   bool `json:"is_followed_by"`
	PostsCount     int  `json:"posts_count"`
}

type Session struct {
	ID           int64     `json:"id"`
	UserID       int64     `json:"user_id"`
	SessionToken string    `json:"session_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	CreatedAt    time.Time `json:"created_at"`
}

type Follow struct {
	ID          int64     `json:"id"`
	FollowerID  int64     `json:"follower_id"`
	FollowingID int64     `json:"following_id"`
	Status      string    `json:"status"` // pending, accepted, declined
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
