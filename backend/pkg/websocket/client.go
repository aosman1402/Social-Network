package websocket

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	gorilla "github.com/gorilla/websocket" // Import with alias "gorilla"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512 * 1024 // 512KB
)

type Client struct {
	Hub    *Hub
	Conn   *gorilla.Conn // Use gorilla.Conn, not websocket.Conn
	Send   chan []byte
	UserID int64
	// SessionToken is the login session this connection was opened with,
	// so logging out can close it (Hub.DisconnectSession).
	SessionToken string
	User         interface{}
	RoomID       string
	IsOnline     bool
	LastSeen     time.Time
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

			// One event per WebSocket frame. Queued events used to be
			// packed into a single frame separated by newlines, which the
			// browser can't parse as JSON - so whenever two or more events
			// reached a user at nearly the same moment (a burst of chat
			// messages, a notification plus a typing event...), all of
			// them were silently dropped.
			if err := c.Conn.WriteMessage(gorilla.TextMessage, message); err != nil {
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
				// ClientID is the sender's temporary id for its optimistic
				// copy, echoed back in the ack / chat_error so the page can
				// swap in the real message (or drop that one message) even
				// if replies arrive out of order.
				ClientID string `json:"client_id"`
			}
			if err := json.Unmarshal(wsMsg.Data, &chatData); err != nil {
				log.Printf("Failed to parse chat data: %v", err)
				continue
			}

			// Handle private or group chat. Handled inline, not in a new
			// goroutine per message: goroutines race each other, so a
			// quick run of messages from one sender used to be saved and
			// delivered in a shuffled order.
			if chatData.RecipientID != 0 {
				func() {
					sent, err := SendPrivateMessage(db, c.Hub, c.UserID, chatData.RecipientID, chatData.Content)
					if err == nil {
						broadcastEvent(c.Hub, "user_"+strconv.FormatInt(c.UserID, 10), "ack", map[string]interface{}{
							"message":      "sent",
							"message_id":   sent.MessageID,
							"client_id":    chatData.ClientID,
							"recipient_id": chatData.RecipientID,
							"created_at":   sent.CreatedAt,
						})
					} else {
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
							"client_id":    chatData.ClientID,
							"error":        errMsg,
						})
					}
				}()
			} else if chatData.GroupID != 0 {
				func() {
					if err := SendGroupMessage(db, c.Hub, c.UserID, chatData.GroupID, chatData.Content); err != nil {
						log.Printf("Failed to send group message: %v", err)
						errMsg := "Failed to send message"
						switch err {
						case ErrInvalidContent:
							errMsg = "Messages must be between 1 and 1000 characters"
						case ErrNotAllowed:
							errMsg = "You are not a member of this group"
						}
						broadcastEvent(c.Hub, "user_"+strconv.FormatInt(c.UserID, 10), "chat_error", map[string]interface{}{
							"group_id": chatData.GroupID,
							"error":    errMsg,
						})
					}
				}()
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

// MaxChatMessageLength caps a chat message's length in characters (runes,
// so emoji count as one each rather than as their 4 UTF-8 bytes).
const MaxChatMessageLength = 1000

// ErrInvalidContent is returned for a chat message that is empty,
// whitespace-only, or longer than MaxChatMessageLength.
var ErrInvalidContent = errors.New("message must be between 1 and 1000 characters")

// validateChatContent trims content and checks it against the rules above.
func validateChatContent(content string) (string, error) {
	content = strings.TrimSpace(content)
	if content == "" || utf8.RuneCountInString(content) > MaxChatMessageLength {
		return "", ErrInvalidContent
	}
	return content, nil
}

// SendGroupMessage saves a group chat message and delivers it as a "chat"
// event to every active member of the group - including the sender, whose
// other open tabs need it too, and whose group chat panel shows the
// server's copy rather than an optimistic one.
func SendGroupMessage(db *sql.DB, hub *Hub, senderID, groupID int64, content string) error {
	content, err := validateChatContent(content)
	if err != nil {
		return err
	}
	if !isActiveGroupMember(db, groupID, senderID) {
		return ErrNotAllowed
	}

	createdAt := time.Now()
	result, err := db.Exec(
		`INSERT INTO messages (sender_id, group_id, content, is_group_message, created_at)
         VALUES (?, ?, ?, ?, ?)`,
		senderID, groupID, content, true, createdAt,
	)
	if err != nil {
		return err
	}
	messageID, _ := result.LastInsertId()

	msg := map[string]interface{}{
		"id":         messageID,
		"sender_id":  senderID,
		"group_id":   groupID,
		"content":    content,
		"created_at": createdAt,
		"sender":     loadSenderInfo(db, senderID),
	}
	for _, memberID := range activeGroupMemberIDs(db, groupID) {
		broadcastEvent(hub, "user_"+strconv.FormatInt(memberID, 10), "chat", msg)
	}
	return nil
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
		if err := rows.Scan(&id); err != nil {
			log.Printf("Failed to scan member of group %d: %v", groupID, err)
			continue
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		log.Printf("Listing members of group %d stopped early: %v", groupID, err)
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
