package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"

	"social-network/pkg/middleware"
	"social-network/pkg/websocket"
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

	result, err := wsh.DB.Exec(
		"UPDATE notifications SET is_read = true WHERE id = ? AND user_id = ? AND is_read = false",
		req.NotificationID, userID,
	)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to mark notification as read")
		return
	}
	if n, _ := result.RowsAffected(); n > 0 {
		notifyNotificationsRead(wsh.Hub, userID, []int64{req.NotificationID}, false)
	}

	json.NewEncoder(w).Encode(map[string]string{
		"message": "Notification marked as read",
	})
}

// MarkAllNotificationsAsRead - Mark all notifications as read
func (wsh *WebSocketHandler) MarkAllNotificationsAsRead(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.GetUserID(r)

	result, err := wsh.DB.Exec(
		"UPDATE notifications SET is_read = true WHERE user_id = ? AND is_read = false",
		userID,
	)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to mark notifications as read")
		return
	}
	if n, _ := result.RowsAffected(); n > 0 {
		notifyNotificationsRead(wsh.Hub, userID, nil, true)
	}

	json.NewEncoder(w).Encode(map[string]string{
		"message": "All notifications marked as read",
	})
}

// notifyNotificationsRead tells every open tab of userID that some of their
// notifications were marked read - either the listed ids, or all of them -
// so the navbar bell and the notifications page stay in sync no matter
// where the read happened (the other of the two, another tab, or the
// server itself, e.g. answering a group invitation).
func notifyNotificationsRead(hub *websocket.Hub, userID int64, ids []int64, all bool) {
	if ids == nil {
		ids = []int64{}
	}
	hub.NotifyUserEvent(userID, "notifications_read", map[string]interface{}{
		"notification_ids": ids,
		"all":              all,
	})
}

// markNotificationsRead marks userID's unread notifications matching the
// extra WHERE clause (e.g. "type = 'group_request' AND related_id = ?")
// as read, and pushes notifications_read for exactly those ids. Used where
// the server itself resolves a notification - answering an invitation or
// a join request - so the bell doesn't keep counting it.
func markNotificationsRead(db *sql.DB, hub *websocket.Hub, userID int64, where string, args ...interface{}) {
	queryArgs := append([]interface{}{userID}, args...)
	rows, err := db.Query(
		"SELECT id FROM notifications WHERE user_id = ? AND is_read = 0 AND "+where,
		queryArgs...,
	)
	if err != nil {
		log.Printf("markNotificationsRead: query failed: %v", err)
		return
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	rows.Close()

	for _, id := range ids {
		db.Exec("UPDATE notifications SET is_read = 1 WHERE id = ?", id)
	}
	if len(ids) > 0 {
		notifyNotificationsRead(hub, userID, ids, false)
	}
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
