package models

import (
    "time"
)

type Message struct {
    ID             int64      `json:"id"`
    SenderID       int64      `json:"sender_id"`
    RecipientID    *int64     `json:"recipient_id"`
    GroupID        *int64     `json:"group_id"`
    Content        string     `json:"content"`
    IsGroupMessage bool       `json:"is_group_message"`
    CreatedAt      time.Time  `json:"created_at"`
    Sender         *User      `json:"sender,omitempty"`
}

type Notification struct {
    ID        int64     `json:"id"`
    UserID    int64     `json:"user_id"`
    Type      string    `json:"type"` // follow_request, follow_accepted, new_follower, group_invitation, group_request, event_created
    Content   string    `json:"content"`
    IsRead    bool      `json:"is_read"`
    RelatedID *int64    `json:"related_id"`
    CreatedAt time.Time `json:"created_at"`
}