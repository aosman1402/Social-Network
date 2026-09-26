package websocket

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"strconv"
	"time"
)

// ErrNotAllowed is returned by SendPrivateMessage when senderID and
// recipientID aren't allowed to message each other (no accepted follow in
// either direction - see CanDirectMessage).
var ErrNotAllowed = errors.New("not allowed to message this user")

// SenderInfo is the subset of a user's profile attached to an outgoing
// chat message, identifying who sent it.
type SenderInfo struct {
	ID        int64  `json:"id"`
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Nickname  string `json:"nickname"`
	Avatar    string `json:"avatar_path"`
}

// SentPrivateMessage is what SendPrivateMessage returns on success - enough
// for a caller to also build its own response (e.g. the HTTP endpoint's
// JSON body) without re-querying anything.
type SentPrivateMessage struct {
	MessageID int64
	CreatedAt time.Time
	Sender    SenderInfo
}

// SendPrivateMessage validates that senderID may message recipientID (an
// accepted follow in either direction - see CanDirectMessage), saves the
// message, and broadcasts it as a "chat" event to the recipient's room.
//
// This is the single implementation shared by the HTTP /api/messages/send
// endpoint and the WebSocket "chat" message type - previously each
// reimplemented the permission check and broadcast independently, and had
// already drifted (only one of the two sent a delivery "ack", and they
// serialized slightly different sender fields).
//
// The WebSocket path sends the sender its own "ack" (see client.go), since
// only it knows the client's temporary id; the HTTP path's response is
// already the confirmation.
func SendPrivateMessage(db *sql.DB, hub *Hub, senderID, recipientID int64, content string) (*SentPrivateMessage, error) {
	allowed, err := CanDirectMessage(db, senderID, recipientID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, ErrNotAllowed
	}

	createdAt := time.Now()
	result, err := db.Exec(
		`INSERT INTO messages (sender_id, recipient_id, content, is_group_message, created_at)
         VALUES (?, ?, ?, ?, ?)`,
		senderID, recipientID, content, false, createdAt,
	)
	if err != nil {
		return nil, err
	}
	messageID, _ := result.LastInsertId()

	sender := loadSenderInfo(db, senderID)

	msg := map[string]interface{}{
		"id":           messageID,
		"sender_id":    senderID,
		"recipient_id": recipientID,
		"content":      content,
		"created_at":   createdAt,
		"sender":       sender,
	}
	broadcastEvent(hub, "user_"+strconv.FormatInt(recipientID, 10), "chat", msg)

	return &SentPrivateMessage{MessageID: messageID, CreatedAt: createdAt, Sender: sender}, nil
}

// loadSenderInfo fetches the profile fields attached to an outgoing chat
// message. nickname and avatar_path are nullable, and scanning a NULL into
// a plain string fails the whole Scan - which used to send every message
// from a user without a nickname with an empty sender - so both are
// COALESCEd to "".
func loadSenderInfo(db *sql.DB, userID int64) SenderInfo {
	var sender SenderInfo
	if err := db.QueryRow(
		`SELECT id, email, first_name, last_name, COALESCE(nickname, ''), COALESCE(avatar_path, '')
         FROM users WHERE id = ?`,
		userID,
	).Scan(&sender.ID, &sender.Email, &sender.FirstName, &sender.LastName, &sender.Nickname, &sender.Avatar); err != nil {
		log.Printf("Failed to load sender %d: %v", userID, err)
	}
	return sender
}

// CanDirectMessage reports whether senderID may send recipientID a direct
// message: an accepted follow in either direction (senderID follows
// recipientID, or recipientID follows senderID). A public account is
// otherwise open to everyone to *view*, but that's not the same as being
// open to receiving DMs from anyone - direct messages always require an
// actual follow relationship, regardless of the recipient's privacy
// setting. The single implementation shared by GetChatHistory and
// SendPrivateMessage (itself shared by the HTTP send endpoint and the
// WebSocket "chat" path), so all three enforce exactly the same rule.
func CanDirectMessage(db *sql.DB, senderID, recipientID int64) (bool, error) {
	var count int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM follows
         WHERE (follower_id = ? AND following_id = ? AND status = 'accepted')
         OR (follower_id = ? AND following_id = ? AND status = 'accepted')`,
		senderID, recipientID, recipientID, senderID,
	).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

// broadcastEvent marshals {"type": eventType, "data": data} and sends it to
// roomID via the hub. Errors are swallowed (matching the rest of this
// package's broadcast call sites) - a marshal failure here would mean a bug
// in the data being sent, not something the caller can act on.
func broadcastEvent(hub *Hub, roomID, eventType string, data interface{}) {
	payload, err := json.Marshal(data)
	if err != nil {
		return
	}
	wsMsg := map[string]interface{}{
		"type": eventType,
		"data": json.RawMessage(payload),
	}
	msgBytes, err := json.Marshal(wsMsg)
	if err != nil {
		return
	}
	hub.BroadcastToRoom(roomID, msgBytes)
}
