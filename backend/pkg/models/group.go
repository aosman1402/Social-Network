package models

import (
    "time"
)

type Group struct {
    ID          int64      `json:"id"`
    CreatorID   int64      `json:"creator_id"`
    Title       string     `json:"title"`
    Description *string    `json:"description"`
    CreatedAt   time.Time  `json:"created_at"`
    UpdatedAt   time.Time  `json:"updated_at"`
    Creator     *User      `json:"creator,omitempty"`
    Members     []GroupMember `json:"members,omitempty"`
    MemberCount int        `json:"member_count"`
}

type GroupMember struct {
    ID        int64     `json:"id"`
    GroupID   int64     `json:"group_id"`
    UserID    int64     `json:"user_id"`
    Status    string    `json:"status"` // pending, active, invited
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
    User      *User     `json:"user,omitempty"`
}

type Event struct {
    ID          int64      `json:"id"`
    GroupID     int64      `json:"group_id"`
    CreatorID   int64      `json:"creator_id"`
    Title       string     `json:"title"`
    Description *string    `json:"description"`
    EventTime   time.Time  `json:"event_time"`
    CreatedAt   time.Time  `json:"created_at"`
    UpdatedAt   time.Time  `json:"updated_at"`
    Creator     *User      `json:"creator,omitempty"`
    Responses   []EventResponse `json:"responses,omitempty"`
}

type EventResponse struct {
    ID       int64     `json:"id"`
    EventID  int64     `json:"event_id"`
    UserID   int64     `json:"user_id"`
    Response string    `json:"response"` // going, not_going
    CreatedAt time.Time `json:"created_at"`
    User     *User     `json:"user,omitempty"`
}