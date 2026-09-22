package handlers

import (
    "encoding/json"
    "log"
    "net/http"

    "social-network/pkg/middleware"
)

// GetNotifications - Get user notifications
func (wsh *WebSocketHandler) GetNotifications(w http.ResponseWriter, r *http.Request) {
    userID, _ := middleware.GetUserID(r)
    limit, offset := parsePagination(r, 50, 100)

    rows, err := wsh.DB.Query(
        `SELECT id, type, content, is_read, related_id, post_id, is_cancelled, created_at
         FROM notifications
         WHERE user_id = ?
         ORDER BY created_at DESC
         LIMIT ? OFFSET ?`,
        userID, limit, offset,
    )
    if err != nil {
        log.Printf("GetNotifications: failed to query notifications: %v", err)
        writeJSONError(w, http.StatusInternalServerError, "Failed to get notifications")
        return
    }
    defer rows.Close()

    notifications := []Notification{}
    for rows.Next() {
        var n Notification
        err := rows.Scan(&n.ID, &n.Type, &n.Content, &n.IsRead, &n.RelatedID, &n.PostID, &n.IsCancelled, &n.CreatedAt)
        if err != nil {
            continue
        }
        n.UserID = userID
        notifications = append(notifications, n)
    }

    if err = rows.Err(); err != nil {
        writeJSONError(w, http.StatusInternalServerError, "Error iterating notifications")
        return
    }

    json.NewEncoder(w).Encode(notifications)
}

// MarkNotificationAsRead - Mark a single notification as read
func (wsh *WebSocketHandler) MarkNotificationAsRead(w http.ResponseWriter, r *http.Request) {
    userID, _ := middleware.GetUserID(r)

    var req struct {
        NotificationID int64 `json:"notification_id"`
    }

    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeJSONError(w, http.StatusBadRequest, "Invalid request body")
        return
    }

    _, err := wsh.DB.Exec(
        "UPDATE notifications SET is_read = true WHERE id = ? AND user_id = ?",
        req.NotificationID, userID,
    )
    if err != nil {
        writeJSONError(w, http.StatusInternalServerError, "Failed to mark notification as read")
        return
    }

    json.NewEncoder(w).Encode(map[string]string{
        "message": "Notification marked as read",
    })
}

// MarkAllNotificationsAsRead - Mark all notifications as read
func (wsh *WebSocketHandler) MarkAllNotificationsAsRead(w http.ResponseWriter, r *http.Request) {
    userID, _ := middleware.GetUserID(r)

    _, err := wsh.DB.Exec(
        "UPDATE notifications SET is_read = true WHERE user_id = ?",
        userID,
    )
    if err != nil {
        writeJSONError(w, http.StatusInternalServerError, "Failed to mark notifications as read")
        return
    }

    json.NewEncoder(w).Encode(map[string]string{
        "message": "All notifications marked as read",
    })
}

// GetUnreadCount - Get unread notification count
func (wsh *WebSocketHandler) GetUnreadCount(w http.ResponseWriter, r *http.Request) {
    userID, _ := middleware.GetUserID(r)

    var count int
    err := wsh.DB.QueryRow(
        "SELECT COUNT(*) FROM notifications WHERE user_id = ? AND is_read = false",
        userID,
    ).Scan(&count)
    if err != nil {
        writeJSONError(w, http.StatusInternalServerError, "Failed to get unread count")
        return
    }

    json.NewEncoder(w).Encode(map[string]int{
        "unread_count": count,
    })
}
