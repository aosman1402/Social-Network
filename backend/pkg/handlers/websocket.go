package handlers

import (
    "database/sql"
    "encoding/json"
    "log"
    "net/http"
    "strconv"
    "time"

    "social-network3/pkg/middleware"
    "social-network3/pkg/models"
    "social-network3/pkg/websocket"

    gorilla "github.com/gorilla/websocket"
)

var upgrader = gorilla.Upgrader{
    CheckOrigin: func(r *http.Request) bool {
        return true
    },
    ReadBufferSize:  1024,
    WriteBufferSize: 1024,
}

type WebSocketHandler struct {
    DB  *sql.DB
    Hub *websocket.Hub
}

// HandleWebSocket - Handle WebSocket connections
func (wsh *WebSocketHandler) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
    // Get token from query parameter
    token := r.URL.Query().Get("token")
    
    // If no token in query, try cookie
    if token == "" {
        cookie, err := r.Cookie("session_token")
        if err != nil {
            http.Error(w, "Unauthorized - No token", http.StatusUnauthorized)
            return
        }
        token = cookie.Value
    }

    if token == "" {
        http.Error(w, "Unauthorized - Empty token", http.StatusUnauthorized)
        return
    }

    // Validate token
    var userID int64
    var expiresAt time.Time
    err := wsh.DB.QueryRow(
        "SELECT user_id, expires_at FROM sessions WHERE session_token = ?",
        token,
    ).Scan(&userID, &expiresAt)

    if err != nil {
        http.Error(w, "Unauthorized - Invalid token", http.StatusUnauthorized)
        return
    }

    if time.Now().After(expiresAt) {
        http.Error(w, "Unauthorized - Token expired", http.StatusUnauthorized)
        return
    }

    // Upgrade to WebSocket
    conn, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        log.Printf("Failed to upgrade connection: %v", err)
        return
    }

    // Get user info
    var user models.User
    err = wsh.DB.QueryRow(
        `SELECT id, email, first_name, last_name, nickname, avatar_path 
         FROM users WHERE id = ?`,
        userID,
    ).Scan(&user.ID, &user.Email, &user.FirstName, &user.LastName, &user.Nickname, &user.AvatarPath)
    if err != nil {
        conn.Close()
        return
    }

    // Create client
    client := &websocket.Client{
        Hub:      wsh.Hub,
        Conn:     conn,
        Send:     make(chan []byte, 256),
        UserID:   userID,
        User:     &user,
        RoomID:   "",
        IsOnline: true,
        LastSeen: time.Now(),
    }

    wsh.Hub.Register <- client
    go client.WritePump()
    go client.ReadPump(wsh.DB)
}

// GetChats - Get all chats for a user
func (wsh *WebSocketHandler) GetChats(w http.ResponseWriter, r *http.Request) {
    userID, _ := middleware.GetUserID(r)

    rows, err := wsh.DB.Query(`
        SELECT DISTINCT 
            CASE 
                WHEN m.sender_id = ? THEN m.recipient_id
                ELSE m.sender_id
            END as other_user_id,
            (SELECT content FROM messages 
             WHERE (sender_id = ? AND recipient_id = other_user_id) 
                OR (sender_id = other_user_id AND recipient_id = ?)
             ORDER BY created_at DESC LIMIT 1) as last_message,
            (SELECT created_at FROM messages 
             WHERE (sender_id = ? AND recipient_id = other_user_id) 
                OR (sender_id = other_user_id AND recipient_id = ?)
             ORDER BY created_at DESC LIMIT 1) as last_message_at
         FROM messages m
         WHERE m.sender_id = ? OR m.recipient_id = ?
         GROUP BY other_user_id
         ORDER BY last_message_at DESC
    `, userID, userID, userID, userID, userID, userID)

    if err != nil {
        http.Error(w, "Failed to get chats", http.StatusInternalServerError)
        return
    }
    defer rows.Close()

    var chats []map[string]interface{}
    for rows.Next() {
        var otherUserID int64
        var lastMessage sql.NullString
        var lastMessageAt sql.NullTime

        err := rows.Scan(&otherUserID, &lastMessage, &lastMessageAt)
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
            "other_user":      user,
            "last_message":    lastMessage.String,
            "last_message_at": lastMessageAt.Time,
        })
    }

    if err = rows.Err(); err != nil {
        http.Error(w, "Error iterating chats", http.StatusInternalServerError)
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
        http.Error(w, "Invalid user ID", http.StatusBadRequest)
        return
    }

    rows, err := wsh.DB.Query(
        `SELECT m.id, m.sender_id, m.content, m.created_at,
                u.id, u.email, u.first_name, u.last_name, u.nickname, u.avatar_path
         FROM messages m
         JOIN users u ON m.sender_id = u.id
         WHERE (m.sender_id = ? AND m.recipient_id = ?)
            OR (m.sender_id = ? AND m.recipient_id = ?)
         ORDER BY m.created_at ASC
         LIMIT 50`,
        userID, otherUserID, otherUserID, userID,
    )
    if err != nil {
        http.Error(w, "Failed to get chat history", http.StatusInternalServerError)
        return
    }
    defer rows.Close()

    var messages []map[string]interface{}
    for rows.Next() {
        var msg struct {
            ID        int64
            SenderID  int64
            Content   string
            CreatedAt time.Time
        }
        var sender models.User

        err := rows.Scan(
            &msg.ID, &msg.SenderID, &msg.Content, &msg.CreatedAt,
            &sender.ID, &sender.Email, &sender.FirstName, &sender.LastName, &sender.Nickname, &sender.AvatarPath,
        )
        if err != nil {
            continue
        }

        messages = append(messages, map[string]interface{}{
            "id":         msg.ID,
            "sender_id":  msg.SenderID,
            "content":    msg.Content,
            "created_at": msg.CreatedAt,
            "sender":     sender,
        })
    }

    if err = rows.Err(); err != nil {
        http.Error(w, "Error iterating messages", http.StatusInternalServerError)
        return
    }

    json.NewEncoder(w).Encode(messages)
}

// SendMessage - Send a private message
func (wsh *WebSocketHandler) SendMessage(w http.ResponseWriter, r *http.Request) {
    userID, _ := middleware.GetUserID(r)

    var req struct {
        RecipientID int64  `json:"recipient_id"`
        Content     string `json:"content"`
    }

    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }

    if req.Content == "" {
        http.Error(w, "Content is required", http.StatusBadRequest)
        return
    }

    // Save message
    result, err := wsh.DB.Exec(
        `INSERT INTO messages (sender_id, recipient_id, content, created_at)
         VALUES (?, ?, ?, ?)`,
        userID, req.RecipientID, req.Content, time.Now(),
    )
    if err != nil {
        http.Error(w, "Failed to save message", http.StatusInternalServerError)
        return
    }

    messageID, _ := result.LastInsertId()

    // Get sender info
    var sender models.User
    wsh.DB.QueryRow(
        `SELECT id, email, first_name, last_name, nickname, avatar_path 
         FROM users WHERE id = ?`,
        userID,
    ).Scan(&sender.ID, &sender.Email, &sender.FirstName, &sender.LastName, &sender.Nickname, &sender.AvatarPath)

    // Create message response
    message := map[string]interface{}{
        "id":           messageID,
        "sender_id":    userID,
        "recipient_id": req.RecipientID,
        "content":      req.Content,
        "created_at":   time.Now(),
        "sender":       sender,
    }

    // Send via WebSocket if recipient is online
    roomID := "user_" + strconv.FormatInt(req.RecipientID, 10)
    data, _ := json.Marshal(message)
    wsMsg := map[string]interface{}{
        "type": "chat",
        "data": json.RawMessage(data),
    }
    msgBytes, _ := json.Marshal(wsMsg)

    if clients, ok := wsh.Hub.Rooms[roomID]; ok {
        for client := range clients {
            select {
            case client.Send <- msgBytes:
            default:
                // Skip if buffer is full
            }
        }
    }

    json.NewEncoder(w).Encode(map[string]interface{}{
        "message":    "Message sent",
        "message_id": messageID,
        "data":       message,
    })
}

// SendGroupMessage - Send a message to a group
func (wsh *WebSocketHandler) SendGroupMessage(w http.ResponseWriter, r *http.Request) {
    userID, _ := middleware.GetUserID(r)

    var req struct {
        GroupID int64  `json:"group_id"`
        Content string `json:"content"`
    }

    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }

    if req.Content == "" {
        http.Error(w, "Content is required", http.StatusBadRequest)
        return
    }

    // Check if user is a member
    var count int
    err := wsh.DB.QueryRow(
        "SELECT COUNT(*) FROM group_members WHERE group_id = ? AND user_id = ? AND status = 'active'",
        req.GroupID, userID,
    ).Scan(&count)

    if err != nil || count == 0 {
        http.Error(w, "You are not a member of this group", http.StatusForbidden)
        return
    }

    // Save message
    result, err := wsh.DB.Exec(
        `INSERT INTO messages (sender_id, group_id, content, is_group_message, created_at)
         VALUES (?, ?, ?, ?, ?)`,
        userID, req.GroupID, req.Content, true, time.Now(),
    )
    if err != nil {
        http.Error(w, "Failed to save message", http.StatusInternalServerError)
        return
    }

    messageID, _ := result.LastInsertId()

    // Get sender info
    var sender models.User
    wsh.DB.QueryRow(
        `SELECT id, email, first_name, last_name, nickname, avatar_path 
         FROM users WHERE id = ?`,
        userID,
    ).Scan(&sender.ID, &sender.Email, &sender.FirstName, &sender.LastName, &sender.Nickname, &sender.AvatarPath)

    // Create message response
    message := map[string]interface{}{
        "id":               messageID,
        "sender_id":        userID,
        "group_id":         req.GroupID,
        "content":          req.Content,
        "created_at":       time.Now(),
        "sender":           sender,
        "is_group_message": true,
    }

    // Send via WebSocket to all group members
    roomID := "group_" + strconv.FormatInt(req.GroupID, 10)
    data, _ := json.Marshal(message)
    wsMsg := map[string]interface{}{
        "type": "chat",
        "data": json.RawMessage(data),
    }
    msgBytes, _ := json.Marshal(wsMsg)

    if clients, ok := wsh.Hub.Rooms[roomID]; ok {
        for client := range clients {
            select {
            case client.Send <- msgBytes:
            default:
                // Skip if buffer is full
            }
        }
    }

    json.NewEncoder(w).Encode(map[string]interface{}{
        "message":    "Message sent",
        "message_id": messageID,
        "data":       message,
    })
}

// GetGroupMessages - Get message history for a group
func (wsh *WebSocketHandler) GetGroupMessages(w http.ResponseWriter, r *http.Request) {
    userID, _ := middleware.GetUserID(r)

    groupIDStr := r.URL.Query().Get("group_id")
    groupID, err := strconv.ParseInt(groupIDStr, 10, 64)
    if err != nil {
        http.Error(w, "Invalid group ID", http.StatusBadRequest)
        return
    }

    // Check if user is a member
    var count int
    err = wsh.DB.QueryRow(
        "SELECT COUNT(*) FROM group_members WHERE group_id = ? AND user_id = ? AND status = 'active'",
        groupID, userID,
    ).Scan(&count)
    if err != nil || count == 0 {
        http.Error(w, "Not a member of this group", http.StatusForbidden)
        return
    }

    rows, err := wsh.DB.Query(
        `SELECT m.id, m.sender_id, m.content, m.created_at,
                u.id, u.email, u.first_name, u.last_name, u.nickname, u.avatar_path
         FROM messages m
         JOIN users u ON m.sender_id = u.id
         WHERE m.group_id = ?
         ORDER BY m.created_at ASC
         LIMIT 50`,
        groupID,
    )
    if err != nil {
        http.Error(w, "Failed to get group messages", http.StatusInternalServerError)
        return
    }
    defer rows.Close()

    var messages []map[string]interface{}
    for rows.Next() {
        var msg struct {
            ID        int64
            SenderID  int64
            Content   string
            CreatedAt time.Time
        }
        var sender models.User

        err := rows.Scan(
            &msg.ID, &msg.SenderID, &msg.Content, &msg.CreatedAt,
            &sender.ID, &sender.Email, &sender.FirstName, &sender.LastName, &sender.Nickname, &sender.AvatarPath,
        )
        if err != nil {
            continue
        }

        messages = append(messages, map[string]interface{}{
            "id":          msg.ID,
            "sender_id":   msg.SenderID,
            "content":     msg.Content,
            "created_at":  msg.CreatedAt,
            "sender":      sender,
            "is_group_message": true,
        })
    }

    if err = rows.Err(); err != nil {
        http.Error(w, "Error iterating messages", http.StatusInternalServerError)
        return
    }

    json.NewEncoder(w).Encode(messages)
}

// ============================================
// ✅ NOTIFICATION METHODS - ONLY ONCE
// ============================================

// SendNotification - Send a notification to a user
func (wsh *WebSocketHandler) SendNotification(db *sql.DB, userID int64, notificationType string, content string, relatedID *int64) error {
    result, err := db.Exec(
        `INSERT INTO notifications (user_id, type, content, related_id, created_at, is_read)
         VALUES (?, ?, ?, ?, ?, ?)`,
        userID, notificationType, content, relatedID, time.Now(), false,
    )
    if err != nil {
        return err
    }

    notificationID, _ := result.LastInsertId()

    notification := map[string]interface{}{
        "id":         notificationID,
        "user_id":    userID,
        "type":       notificationType,
        "content":    content,
        "related_id": relatedID,
        "created_at": time.Now(),
        "is_read":    false,
    }

    // Send to user if online via WebSocket
    roomID := "user_" + strconv.FormatInt(userID, 10)
    if clients, ok := wsh.Hub.Rooms[roomID]; ok {
        data, _ := json.Marshal(notification)
        wsMsg := map[string]interface{}{
            "type": "notification",
            "data": json.RawMessage(data),
        }
        msgBytes, _ := json.Marshal(wsMsg)
        for client := range clients {
            select {
            case client.Send <- msgBytes:
            default:
                // Skip if buffer is full
            }
        }
    }

    return nil
}

// GetNotifications - Get user notifications
func (wsh *WebSocketHandler) GetNotifications(w http.ResponseWriter, r *http.Request) {
    userID, _ := middleware.GetUserID(r)

    rows, err := wsh.DB.Query(
        `SELECT id, type, content, is_read, related_id, created_at
         FROM notifications
         WHERE user_id = ?
         ORDER BY created_at DESC
         LIMIT 50`,
        userID,
    )
    if err != nil {
        http.Error(w, "Failed to get notifications: "+err.Error(), http.StatusInternalServerError)
        return
    }
    defer rows.Close()

    var notifications []map[string]interface{}
    for rows.Next() {
        var id int64
        var notifType string
        var content string
        var isRead bool
        var relatedID sql.NullInt64
        var createdAt time.Time

        err := rows.Scan(&id, &notifType, &content, &isRead, &relatedID, &createdAt)
        if err != nil {
            continue
        }

        notifications = append(notifications, map[string]interface{}{
            "id":         id,
            "type":       notifType,
            "content":    content,
            "is_read":    isRead,
            "related_id": relatedID.Int64,
            "created_at": createdAt,
        })
    }

    if err = rows.Err(); err != nil {
        http.Error(w, "Error iterating notifications", http.StatusInternalServerError)
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
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }

    result, err := wsh.DB.Exec(
        "UPDATE notifications SET is_read = true WHERE id = ? AND user_id = ?",
        req.NotificationID, userID,
    )
    if err != nil {
        http.Error(w, "Failed to mark notification as read: "+err.Error(), http.StatusInternalServerError)
        return
    }

    rowsAffected, _ := result.RowsAffected()
    if rowsAffected == 0 {
        http.Error(w, "Notification not found", http.StatusNotFound)
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
        http.Error(w, "Failed to mark notifications as read: "+err.Error(), http.StatusInternalServerError)
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
        http.Error(w, "Failed to get unread count: "+err.Error(), http.StatusInternalServerError)
        return
    }

    json.NewEncoder(w).Encode(map[string]int{
        "unread_count": count,
    })
}