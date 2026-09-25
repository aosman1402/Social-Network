package websocket

import (
	"database/sql"
	"encoding/json"
	"log"
	"strconv"
	"time"

	gorilla "github.com/gorilla/websocket" // Import with alias "gorilla"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512 * 1024 // 512KB
)

type Client struct {
	Hub      *Hub
	Conn     *gorilla.Conn // Use gorilla.Conn, not websocket.Conn
	Send     chan []byte
	UserID   int64
	User     interface{}
	RoomID   string
	IsOnline bool
	LastSeen time.Time
}

// WritePump pumps messages from the hub to the websocket connection
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel
				c.Conn.WriteMessage(gorilla.CloseMessage, []byte{}) // Use gorilla.CloseMessage
				return
			}

			w, err := c.Conn.NextWriter(gorilla.TextMessage) // Use gorilla.TextMessage
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued messages to the same websocket message
			n := len(c.Send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.Send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(gorilla.PingMessage, nil); err != nil { // Use gorilla.PingMessage
				return
			}
		}
	}
}

// ReadPump pumps messages from the websocket connection to the hub
func (c *Client) ReadPump(db *sql.DB) {
	defer func() {
		c.Hub.Unregister <- c
		c.Conn.Close()
	}()

	c.Conn.SetReadLimit(maxMessageSize)
	c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if gorilla.IsUnexpectedCloseError(err, gorilla.CloseGoingAway, gorilla.CloseAbnormalClosure) { // Use gorilla.IsUnexpectedCloseError
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		// Parse message
		var wsMsg struct {
			Type string          `json:"type"`
			Data json.RawMessage `json:"data"`
		}

		if err := json.Unmarshal(message, &wsMsg); err != nil {
			log.Printf("Failed to parse message: %v", err)
			continue
		}

		// Handle different message types
		switch wsMsg.Type {
		case "chat":
			var chatData struct {
				RecipientID int64  `json:"recipient_id"`
				GroupID     int64  `json:"group_id"`
				Content     string `json:"content"`
			}
			if err := json.Unmarshal(wsMsg.Data, &chatData); err != nil {
				log.Printf("Failed to parse chat data: %v", err)
				continue
			}

			// Handle private or group chat
			if chatData.RecipientID != 0 {
				go func() {
					if _, err := SendPrivateMessage(db, c.Hub, c.UserID, chatData.RecipientID, chatData.Content, true); err != nil {
						log.Printf("Failed to send private message: %v", err)

						// The WebSocket path previously failed silently from
						// the sender's point of view - the frontend already
						// shows the message as "sent" optimistically, so
						// without this the sender never learns a blocked
						// send didn't actually go through.
						errMsg := "Failed to send message"
						if err == ErrNotAllowed {
							errMsg = "You can only message users who follow you or that you follow"
						}
						broadcastEvent(c.Hub, "user_"+strconv.FormatInt(c.UserID, 10), "chat_error", map[string]interface{}{
							"recipient_id": chatData.RecipientID,
							"error":        errMsg,
						})
					}
				}()
			} else if chatData.GroupID != 0 {
				go handleGroupMessage(db, c.UserID, chatData.GroupID, chatData.Content, c.Hub)
			}

		case "typing":
			var typingData struct {
				RecipientID int64 `json:"recipient_id"`
				GroupID     int64 `json:"group_id"`
				IsTyping    bool  `json:"is_typing"`
			}
			if err := json.Unmarshal(wsMsg.Data, &typingData); err != nil {
				log.Printf("Failed to parse typing data: %v", err)
				continue
			}

			go relayTyping(db, c.Hub, c.UserID, typingData.RecipientID, typingData.GroupID, typingData.IsTyping)

		case "read_receipt":
			var readData struct {
				MessageID int64 `json:"message_id"`
			}
			if err := json.Unmarshal(wsMsg.Data, &readData); err != nil {
				log.Printf("Failed to parse read receipt data: %v", err)
				continue
			}

			go updateMessageReadStatus(db, c.Hub, readData.MessageID, c.UserID)

		case "ping":
			pongMsg := map[string]string{"type": "pong"}
			data, _ := json.Marshal(pongMsg)
			c.Send <- data
		}
	}
}

func handleGroupMessage(db *sql.DB, senderID, groupID int64, content string, hub *Hub) {
	if !isActiveGroupMember(db, groupID, senderID) {
		return
	}

	// Save message
	result, err := db.Exec(
		`INSERT INTO messages (sender_id, group_id, content, is_group_message, created_at)
         VALUES (?, ?, ?, ?, ?)`,
		senderID, groupID, content, true, time.Now(),
	)
	if err != nil {
		log.Printf("Failed to save group message: %v", err)
		return
	}

	messageID, _ := result.LastInsertId()

	// Get sender info
	var sender struct {
		ID        int64  `json:"id"`
		Email     string `json:"email"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		Nickname  string `json:"nickname"`
		Avatar    string `json:"avatar_path"`
	}
	db.QueryRow(
		`SELECT id, email, first_name, last_name, nickname, avatar_path 
         FROM users WHERE id = ?`,
		senderID,
	).Scan(&sender.ID, &sender.Email, &sender.FirstName, &sender.LastName, &sender.Nickname, &sender.Avatar)

	// Create message object
	msg := map[string]interface{}{
		"id":         messageID,
		"sender_id":  senderID,
		"group_id":   groupID,
		"content":    content,
		"created_at": time.Now(),
		"sender":     sender,
	}
	for _, memberID := range activeGroupMemberIDs(db, groupID) {
		broadcastEvent(hub, "user_"+strconv.FormatInt(memberID, 10), "chat", msg)
	}
}

// isActiveGroupMember reports whether userID is an active member of groupID.
func isActiveGroupMember(db *sql.DB, groupID, userID int64) bool {
	var count int
	db.QueryRow(
		"SELECT COUNT(*) FROM group_members WHERE group_id = ? AND user_id = ? AND status = 'active'",
		groupID, userID,
	).Scan(&count)
	return count > 0
}

// activeGroupMemberIDs lists groupID's active members. Group events are
// delivered to each member's own user_<id> channel rather than a shared
// "group_<id>" room: clients used to be able to join any room by name
// (a "join_room" message with no membership check), which let anyone read
// another user's DMs or a group's chat live.
func activeGroupMemberIDs(db *sql.DB, groupID int64) []int64 {
	rows, err := db.Query(
		"SELECT user_id FROM group_members WHERE group_id = ? AND status = 'active'",
		groupID,
	)
	if err != nil {
		log.Printf("Failed to list members of group %d: %v", groupID, err)
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

// relayTyping forwards a typing indicator only where a message could be
// sent too - to a user the sender may DM (CanDirectMessage), or to the
// other members of a group the sender belongs to. It sends a fixed payload
// rather than echoing the client's raw frame, which previously let any
// user push arbitrary data to anyone's browser.
func relayTyping(db *sql.DB, hub *Hub, senderID, recipientID, groupID int64, isTyping bool) {
	if recipientID != 0 {
		if recipientID == senderID {
			return
		}
		allowed, err := CanDirectMessage(db, senderID, recipientID)
		if err != nil || !allowed {
			return
		}
		broadcastEvent(hub, "user_"+strconv.FormatInt(recipientID, 10), "typing", map[string]interface{}{
			"sender_id":    senderID,
			"recipient_id": recipientID,
			"is_typing":    isTyping,
		})
		return
	}

	if groupID != 0 && isActiveGroupMember(db, groupID, senderID) {
		for _, memberID := range activeGroupMemberIDs(db, groupID) {
			if memberID == senderID {
				continue
			}
			broadcastEvent(hub, "user_"+strconv.FormatInt(memberID, 10), "typing", map[string]interface{}{
				"sender_id": senderID,
				"group_id":  groupID,
				"is_typing": isTyping,
			})
		}
	}
}

func updateMessageReadStatus(db *sql.DB, hub *Hub, messageID int64, userID int64) {
	var recipientID sql.NullInt64
	err := db.QueryRow(
		"SELECT recipient_id FROM messages WHERE id = ?",
		messageID,
	).Scan(&recipientID)

	if err != nil {
		log.Printf("Failed to get message recipient: %v", err)
		return
	}

	// Only the actual recipient can mark a message as read.
	if !recipientID.Valid || recipientID.Int64 != userID {
		return
	}

	result, err := db.Exec(
		"UPDATE messages SET read_at = ? WHERE id = ? AND read_at IS NULL",
		time.Now(), messageID,
	)
	if err != nil {
		log.Printf("Failed to mark message %d as read: %v", messageID, err)
		return
	}

	// Let every open tab of the reader refresh its Chat unread badge.
	// Re-sent receipts for already-read messages change nothing and stay
	// silent, so reopening an old conversation doesn't spam refetches.
	if n, _ := result.RowsAffected(); n > 0 {
		hub.NotifyUserEvent(userID, "messages_read", map[string]interface{}{
			"message_id": messageID,
		})
	}

	log.Printf("Message %d marked as read by user %d", messageID, userID)
}
