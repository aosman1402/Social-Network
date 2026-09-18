package websocket

import (
    "database/sql"
    "encoding/json"
    "errors"
    "strconv"
    "time"
)

// ErrNotAllowed is returned by SendPrivateMessage when senderID and
// recipientID aren't allowed to message each other (no accepted mutual
// follow, and the recipient's account isn't public).
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

// SendPrivateMessage validates that senderID may message recipientID
// (an accepted mutual follow, or a public recipient account), saves the
// message, and broadcasts it as a "chat" event to the recipient's room.
//
// This is the single implementation shared by the HTTP /api/messages/send
// endpoint and the WebSocket "chat" message type - previously each
// reimplemented the permission check and broadcast independently, and had
// already drifted (only one of the two sent a delivery "ack", and they
// serialized slightly different sender fields).
//
// sendAck additionally broadcasts an "ack" event to the sender's own room,
// which the WebSocket path wants (the sender is a live connection waiting
// for delivery confirmation) and the HTTP path doesn't (its own response
// already is the confirmation).
func SendPrivateMessage(db *sql.DB, hub *Hub, senderID, recipientID int64, content string, sendAck bool) (*SentPrivateMessage, error) {
    allowed, err := canDirectMessage(db, senderID, recipientID)
    if err != nil {
        return nil, err
    }
    if !allowed {
        return nil, ErrNotAllowed
    }

    result, err := db.Exec(
        `INSERT INTO messages (sender_id, recipient_id, content, is_group_message, created_at)
         VALUES (?, ?, ?, ?, ?)`,
        senderID, recipientID, content, false, time.Now(),
    )
    if err != nil {
        return nil, err
    }
    messageID, _ := result.LastInsertId()
    createdAt := time.Now()

    var sender SenderInfo
    db.QueryRow(
        `SELECT id, email, first_name, last_name, nickname, avatar_path FROM users WHERE id = ?`,
        senderID,
    ).Scan(&sender.ID, &sender.Email, &sender.FirstName, &sender.LastName, &sender.Nickname, &sender.Avatar)

    msg := map[string]interface{}{
        "id":           messageID,
        "sender_id":    senderID,
        "recipient_id": recipientID,
        "content":      content,
        "created_at":   createdAt,
        "sender":       sender,
    }
    broadcastEvent(hub, "user_"+strconv.FormatInt(recipientID, 10), "chat", msg)

    if sendAck {
        broadcastEvent(hub, "user_"+strconv.FormatInt(senderID, 10), "ack", map[string]interface{}{
            "message":    "sent",
            "message_id": messageID,
        })
    }

    return &SentPrivateMessage{MessageID: messageID, CreatedAt: createdAt, Sender: sender}, nil
}

// canDirectMessage reports whether senderID may send recipientID a direct
// message: either an accepted mutual follow in either direction, or
// recipientID's account is public.
func canDirectMessage(db *sql.DB, senderID, recipientID int64) (bool, error) {
    var count int
    if err := db.QueryRow(
        `SELECT COUNT(*) FROM follows
         WHERE (follower_id = ? AND following_id = ? AND status = 'accepted')
         OR (follower_id = ? AND following_id = ? AND status = 'accepted')`,
        senderID, recipientID, recipientID, senderID,
    ).Scan(&count); err != nil {
        return false, err
    }
    if count > 0 {
        return true, nil
    }

    var isPublic bool
    if err := db.QueryRow(
        "SELECT is_public FROM users WHERE id = ?", recipientID,
    ).Scan(&isPublic); err != nil {
        return false, err
    }
    return isPublic, nil
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
