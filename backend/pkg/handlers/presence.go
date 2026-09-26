package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"social-network/pkg/middleware"
	"social-network/pkg/models"
	"social-network/pkg/websocket"
)

// Online status ("presence"). A user is online while they have at least
// one live WebSocket connection (any open tab). Presence is shared only
// with people they have an accepted follow with in either direction - the
// same people allowed to message them (websocket.CanDirectMessage) - so a
// stranger can't watch when someone is online.

// EnablePresence makes hub send a live "presence" event
// {"user_id": X, "online": bool} to X's connected contacts whenever X comes
// online (first tab opens) or goes offline (last tab closes, or logout).
func EnablePresence(hub *websocket.Hub, db *sql.DB) {
	hub.OnPresenceChange = func(userID int64) {
		online := hub.IsUserOnline(userID)
		for _, contactID := range presenceContacts(db, userID) {
			if hub.IsUserOnline(contactID) {
				hub.NotifyUserEvent(contactID, "presence", map[string]interface{}{
					"user_id": userID,
					"online":  online,
				})
			}
		}
	}
}

// presenceContacts lists everyone with an accepted follow to or from userID.
func presenceContacts(db *sql.DB, userID int64) []int64 {
	rows, err := db.Query(
		`SELECT following_id FROM follows WHERE follower_id = ? AND status = 'accepted'
         UNION
         SELECT follower_id FROM follows WHERE following_id = ? AND status = 'accepted'`,
		userID, userID,
	)
	if err != nil {
		log.Printf("presence: listing contacts of %d: %v", userID, err)
		return nil
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	return ids
}

// GetPresence - GET /api/user/presence?id=X: whether X is online right now.
// Only for X themselves or someone with a follow link to X (403 otherwise).
func (wsh *WebSocketHandler) GetPresence(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	viewerID, _ := middleware.GetUserID(r)

	targetID, err := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}
	if targetID != viewerID {
		allowed, err := websocket.CanDirectMessage(wsh.DB, viewerID, targetID)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "Database error")
			return
		}
		if !allowed {
			writeJSONError(w, http.StatusForbidden, "You can only see the status of people you follow or who follow you")
			return
		}
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"user_id": targetID,
		"online":  wsh.Hub.IsUserOnline(targetID),
	})
}

// GetChatContacts - GET /api/chat/contacts: everyone the viewer can message
// (an accepted follow in either direction), with whether they're online -
// for starting a new conversation from the chat page.
func (wsh *WebSocketHandler) GetChatContacts(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	viewerID, _ := middleware.GetUserID(r)

	contacts := []map[string]interface{}{}
	for _, id := range presenceContacts(wsh.DB, viewerID) {
		var u models.User
		if err := wsh.DB.QueryRow(
			`SELECT id, first_name, last_name, nickname, avatar_path FROM users WHERE id = ?`, id,
		).Scan(&u.ID, &u.FirstName, &u.LastName, &u.Nickname, &u.AvatarPath); err != nil {
			continue
		}
		contacts = append(contacts, map[string]interface{}{
			"user":   u,
			"online": wsh.Hub.IsUserOnline(id),
		})
	}
	json.NewEncoder(w).Encode(contacts)
}
