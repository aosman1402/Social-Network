package handlers

import (
    "database/sql"
    "encoding/json"
    "net/http"
    "strconv"

    "social-network/pkg/middleware"
    "social-network/pkg/models"
    "social-network/pkg/websocket"
)

// GetChats - Get all chats for a user
func (wsh *WebSocketHandler) GetChats(w http.ResponseWriter, r *http.Request) {
    userID, _ := middleware.GetUserID(r)
    limit, offset := parsePagination(r, 50, 100)

    // SQLite (unlike some other databases) won't let a correlated subquery
    // in the SELECT list reference a sibling SELECT-list alias - a CTE
    // materializes other_user_id as a real column first so the subqueries
    // below can join against it.
    rows, err := wsh.DB.Query(
        `WITH conversations AS (
            SELECT DISTINCT
                CASE WHEN sender_id = ? THEN recipient_id ELSE sender_id END as other_user_id
            FROM messages
            WHERE sender_id = ? OR recipient_id = ?
         )
         SELECT
            c.other_user_id,
            (SELECT content FROM messages
             WHERE (sender_id = ? AND recipient_id = c.other_user_id)
                OR (sender_id = c.other_user_id AND recipient_id = ?)
             ORDER BY created_at DESC LIMIT 1) as last_message,
            (SELECT created_at FROM messages
             WHERE (sender_id = ? AND recipient_id = c.other_user_id)
                OR (sender_id = c.other_user_id AND recipient_id = ?)
             ORDER BY created_at DESC LIMIT 1) as last_message_at,
            (SELECT COUNT(*) FROM messages
             WHERE sender_id = c.other_user_id AND recipient_id = ? AND read_at IS NULL) as unread_count
         FROM conversations c
         ORDER BY last_message_at DESC
         LIMIT ? OFFSET ?`,
        userID, userID, userID, userID, userID, userID, userID, userID, limit, offset,
    )
    if err != nil {
        writeJSONError(w, http.StatusInternalServerError, "Failed to get chats")
        return
    }
    defer rows.Close()

    var chats []map[string]interface{}
    for rows.Next() {
        var otherUserID int64
        var lastMessage sql.NullString
        var lastMessageAt sql.NullTime
        var unreadCount int
        err := rows.Scan(&otherUserID, &lastMessage, &lastMessageAt, &unreadCount)
        if err != nil {
            continue
        }

        var user models.User
        wsh.DB.QueryRow(
            `SELECT id, email, first_name, last_name, nickname, avatar_path, is_public
             FROM users WHERE id = ?`,
            otherUserID,
        ).Scan(&user.ID, &user.Email, &user.FirstName, &user.LastName, &user.Nickname, &user.AvatarPath, &user.IsPublic)

        chats = append(chats, map[string]interface{}{
            "other_user_id":   otherUserID,
            "other_user":      user,
            "last_message":    lastMessage.String,
            "last_message_at": lastMessageAt.Time,
            "unread_count":    unreadCount,
        })
    }

    if err = rows.Err(); err != nil {
        writeJSONError(w, http.StatusInternalServerError, "Error iterating chats")
        return
    }

    json.NewEncoder(w).Encode(chats)
}

// GetChatHistory - Get chat history between two users
func (wsh *WebSocketHandler) GetChatHistory(w http.ResponseWriter, r *http.Request) {
    userID, _ := middleware.GetUserID(r)

    otherUserIDStr := r.URL.Query().Get("user_id")
    otherUserID, err := strconv.ParseInt(otherUserIDStr, 10, 64)
    if err != nil {
        writeJSONError(w, http.StatusBadRequest, "Invalid user ID")
        return
    }

    // Check if users can chat
    var count int
    err = wsh.DB.QueryRow(
        `SELECT COUNT(*) FROM follows
         WHERE (follower_id = ? AND following_id = ? AND status = 'accepted')
         OR (follower_id = ? AND following_id = ? AND status = 'accepted')`,
        userID, otherUserID, otherUserID, userID,
    ).Scan(&count)

    if err != nil {
        writeJSONError(w, http.StatusInternalServerError, "Database error")
        return
    }

    var isPublic bool
    wsh.DB.QueryRow(
        "SELECT is_public FROM users WHERE id = ?",
        otherUserID,
    ).Scan(&isPublic)

    if count == 0 && !isPublic {
        writeJSONError(w, http.StatusForbidden, "Cannot chat with this user")
        return
    }

    limit, offset := parsePagination(r, 50, 100)
    rows, err := wsh.DB.Query(
        `SELECT m.id, m.sender_id, m.content, m.created_at,
                u.id, u.email, u.first_name, u.last_name, u.nickname, u.avatar_path
         FROM messages m
         JOIN users u ON m.sender_id = u.id
         WHERE (m.sender_id = ? AND m.recipient_id = ?)
            OR (m.sender_id = ? AND m.recipient_id = ?)
         ORDER BY m.created_at DESC
         LIMIT ? OFFSET ?`,
        userID, otherUserID, otherUserID, userID, limit, offset,
    )
    if err != nil {
        writeJSONError(w, http.StatusInternalServerError, "Failed to get chat history")
        return
    }
    defer rows.Close()

    var messages []ChatMessage
    for rows.Next() {
        var msg ChatMessage
        var sender models.User
        err := rows.Scan(
            &msg.ID, &msg.SenderID, &msg.Content, &msg.CreatedAt,
            &sender.ID, &sender.Email, &sender.FirstName, &sender.LastName, &sender.Nickname, &sender.AvatarPath,
        )
        if err != nil {
            continue
        }
        msg.Sender = &sender
        msg.RecipientID = &otherUserID
        messages = append(messages, msg)
    }

    if err = rows.Err(); err != nil {
        writeJSONError(w, http.StatusInternalServerError, "Error iterating messages")
        return
    }

    json.NewEncoder(w).Encode(messages)
}

// SendMessage - Send a message via HTTP
func (wsh *WebSocketHandler) SendMessage(w http.ResponseWriter, r *http.Request) {
    userID, _ := middleware.GetUserID(r)

    var req struct {
        RecipientID int64  `json:"recipient_id"`
        Content     string `json:"content"`
    }

    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeJSONError(w, http.StatusBadRequest, "Invalid request body")
        return
    }

    if req.Content == "" {
        writeJSONError(w, http.StatusBadRequest, "Content is required")
        return
    }

    // The permission check, save, and broadcast are shared with the
    // WebSocket "chat" message path (see websocket.SendPrivateMessage) so
    // the two can't drift out of sync on who's allowed to message whom or
    // what a message payload looks like. sendAck is false here because
    // this handler's own HTTP response already confirms delivery to the
    // caller - the WebSocket path additionally broadcasts an "ack" event
    // because the sender there is a live connection waiting for one.
    sent, err := websocket.SendPrivateMessage(wsh.DB, wsh.Hub, userID, req.RecipientID, req.Content, false)
    if err != nil {
        if err == websocket.ErrNotAllowed {
            writeJSONError(w, http.StatusForbidden, "Cannot message this user")
        } else {
            writeJSONError(w, http.StatusInternalServerError, "Failed to send message")
        }
        return
    }

    response := map[string]interface{}{
        "id":           sent.MessageID,
        "sender_id":    userID,
        "recipient_id": req.RecipientID,
        "content":      req.Content,
        "created_at":   sent.CreatedAt,
        "sender":       sent.Sender,
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "message":    "Message sent",
        "message_id": sent.MessageID,
        "data":       response,
    })
}

// GetGroupChatHistory - Get chat history for a group
func (wsh *WebSocketHandler) GetGroupChatHistory(w http.ResponseWriter, r *http.Request) {
    userID, _ := middleware.GetUserID(r)

    groupIDStr := r.URL.Query().Get("group_id")
    groupID, err := strconv.ParseInt(groupIDStr, 10, 64)
    if err != nil {
        writeJSONError(w, http.StatusBadRequest, "Invalid group ID")
        return
    }

    var count int
    err = wsh.DB.QueryRow(
        "SELECT COUNT(*) FROM group_members WHERE group_id = ? AND user_id = ? AND status = 'active'",
        groupID, userID,
    ).Scan(&count)
    if err != nil || count == 0 {
        writeJSONError(w, http.StatusForbidden, "Not a member of this group")
        return
    }

    limit, offset := parsePagination(r, 50, 100)
    rows, err := wsh.DB.Query(
        `SELECT m.id, m.sender_id, m.content, m.created_at,
                u.id, u.email, u.first_name, u.last_name, u.nickname, u.avatar_path
         FROM messages m
         JOIN users u ON m.sender_id = u.id
         WHERE m.group_id = ?
         ORDER BY m.created_at DESC
         LIMIT ? OFFSET ?`,
        groupID, limit, offset,
    )
    if err != nil {
        writeJSONError(w, http.StatusInternalServerError, "Failed to get group chat history")
        return
    }
    defer rows.Close()

    var messages []ChatMessage
    for rows.Next() {
        var msg ChatMessage
        var sender models.User
        err := rows.Scan(
            &msg.ID, &msg.SenderID, &msg.Content, &msg.CreatedAt,
            &sender.ID, &sender.Email, &sender.FirstName, &sender.LastName, &sender.Nickname, &sender.AvatarPath,
        )
        if err != nil {
            continue
        }
        msg.Sender = &sender
        msg.GroupID = &groupID
        msg.IsGroupMessage = true
        messages = append(messages, msg)
    }

    if err = rows.Err(); err != nil {
        writeJSONError(w, http.StatusInternalServerError, "Error iterating messages")
        return
    }

    json.NewEncoder(w).Encode(messages)
}
