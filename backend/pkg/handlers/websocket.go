package handlers

import (
    "database/sql"
    "encoding/json"
    "net/http"
    "strconv"
    "time"

    "social-network3/pkg/middleware"
    "social-network3/pkg/models"
    "social-network3/pkg/websocket"

    gorilla "github.com/gorilla/websocket"
)

// websocketAllowedOrigins mirrors the CORS allowlist in server.go. The
// WebSocket handshake isn't covered by ordinary CORS/preflight checks, so
// the upgrader has to validate Origin itself instead of accepting any
// connection.
var websocketAllowedOrigins = map[string]bool{
    "http://localhost:3000": true,
    "http://localhost:3001": true,
    "http://127.0.0.1:3000": true,
    "http://127.0.0.1:3001": true,
}

var upgrader = gorilla.Upgrader{
    CheckOrigin: func(r *http.Request) bool {
        origin := r.Header.Get("Origin")
        // No Origin header at all means a non-browser client (e.g. a
        // native app or a test script), which CORS/Origin checks don't
        // apply to in the first place.
        return origin == "" || websocketAllowedOrigins[origin]
    },
    ReadBufferSize:  1024,
    WriteBufferSize: 1024,
}

type WebSocketHandler struct {
    DB  *sql.DB
    Hub *websocket.Hub
}

// ChatMessage represents a chat message
type ChatMessage struct {
    ID             int64        `json:"id"`
    SenderID       int64        `json:"sender_id"`
    RecipientID    *int64       `json:"recipient_id,omitempty"`
    GroupID        *int64       `json:"group_id,omitempty"`
    Content        string       `json:"content"`
    IsGroupMessage bool         `json:"is_group_message"`
    CreatedAt      time.Time    `json:"created_at"`
    Sender         *models.User `json:"sender,omitempty"`
}

// Notification represents a notification
type Notification struct {
    ID        int64     `json:"id"`
    UserID    int64     `json:"user_id"`
    Type      string    `json:"type"`
    Content   string    `json:"content"`
    IsRead    bool      `json:"is_read"`
    RelatedID *int64    `json:"related_id,omitempty"`
    CreatedAt time.Time `json:"created_at"`
}

// HandleWebSocket - Handle WebSocket connections
func (wsh *WebSocketHandler) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
    origin := r.Header.Get("Origin")
    if websocketAllowedOrigins[origin] {
        w.Header().Set("Access-Control-Allow-Origin", origin)
        w.Header().Set("Access-Control-Allow-Credentials", "true")
    }
    w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
    w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Accept, Cookie, Set-Cookie, Upgrade, Connection")
    w.Header().Set("Access-Control-Expose-Headers", "Set-Cookie")

    if r.Method == "OPTIONS" {
        w.WriteHeader(http.StatusOK)
        return
    }

    // Session cookie only - no query-string token fallback, which would
    // otherwise leak the session token into URLs (server logs, browser
    // history, proxies).
    cookie, err := r.Cookie("session_token")
    if err != nil {
        http.Error(w, "Unauthorized - No token", http.StatusUnauthorized)
        return
    }
    token := cookie.Value

    if token == "" {
        http.Error(w, "Unauthorized - Empty token", http.StatusUnauthorized)
        return
    }

    var userID int64
    var expiresAt time.Time
    err = wsh.DB.QueryRow(
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

    conn, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        http.Error(w, "Failed to upgrade connection", http.StatusInternalServerError)
        return
    }

    var user models.User
    err = wsh.DB.QueryRow(
        `SELECT id, email, first_name, last_name, nickname, avatar_path 
         FROM users WHERE id = ?`,
        userID,
    ).Scan(&user.ID, &user.Email, &user.FirstName, &user.LastName, &user.Nickname, &user.AvatarPath)
    if err != nil {
        conn.Close()
        http.Error(w, "User not found", http.StatusNotFound)
        return
    }

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

    rows, err := wsh.DB.Query(
        `SELECT DISTINCT 
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
             ORDER BY created_at DESC LIMIT 1) as last_message_at,
            (SELECT COUNT(*) FROM messages 
             WHERE sender_id = other_user_id AND recipient_id = ? AND read_at IS NULL) as unread_count
         FROM messages m
         WHERE m.sender_id = ? OR m.recipient_id = ?
         GROUP BY other_user_id
         ORDER BY last_message_at DESC`,
        userID, userID, userID, userID, userID, userID, userID, userID,
    )
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

    // Check if users can chat
    var count int
    err = wsh.DB.QueryRow(
        `SELECT COUNT(*) FROM follows 
         WHERE (follower_id = ? AND following_id = ? AND status = 'accepted')
         OR (follower_id = ? AND following_id = ? AND status = 'accepted')`,
        userID, otherUserID, otherUserID, userID,
    ).Scan(&count)

    if err != nil {
        http.Error(w, "Database error", http.StatusInternalServerError)
        return
    }

    var isPublic bool
    wsh.DB.QueryRow(
        "SELECT is_public FROM users WHERE id = ?",
        otherUserID,
    ).Scan(&isPublic)

    if count == 0 && !isPublic {
        http.Error(w, "Cannot chat with this user", http.StatusForbidden)
        return
    }

    rows, err := wsh.DB.Query(
        `SELECT m.id, m.sender_id, m.content, m.created_at,
                u.id, u.email, u.first_name, u.last_name, u.nickname, u.avatar_path
         FROM messages m
         JOIN users u ON m.sender_id = u.id
         WHERE (m.sender_id = ? AND m.recipient_id = ?)
            OR (m.sender_id = ? AND m.recipient_id = ?)
         ORDER BY m.created_at DESC
         LIMIT 50`,
        userID, otherUserID, otherUserID, userID,
    )
    if err != nil {
        http.Error(w, "Failed to get chat history", http.StatusInternalServerError)
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
        http.Error(w, "Error iterating messages", http.StatusInternalServerError)
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
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }

    if req.Content == "" {
        http.Error(w, "Content is required", http.StatusBadRequest)
        return
    }

    // Check if users can chat
    var count int
    err := wsh.DB.QueryRow(
        `SELECT COUNT(*) FROM follows 
         WHERE (follower_id = ? AND following_id = ? AND status = 'accepted')
         OR (follower_id = ? AND following_id = ? AND status = 'accepted')`,
        userID, req.RecipientID, req.RecipientID, userID,
    ).Scan(&count)

    if err != nil {
        http.Error(w, "Database error", http.StatusInternalServerError)
        return
    }

    var isPublic bool
    wsh.DB.QueryRow(
        "SELECT is_public FROM users WHERE id = ?",
        req.RecipientID,
    ).Scan(&isPublic)

    if count == 0 && !isPublic {
        http.Error(w, "Cannot message this user", http.StatusForbidden)
        return
    }

    // Save message
    result, err := wsh.DB.Exec(
        `INSERT INTO messages (sender_id, recipient_id, content, is_group_message, created_at)
         VALUES (?, ?, ?, ?, ?)`,
        userID, req.RecipientID, req.Content, false, time.Now(),
    )
    if err != nil {
        http.Error(w, "Failed to save message", http.StatusInternalServerError)
        return
    }

    messageID, _ := result.LastInsertId()

    // Get sender info
    var sender struct {
        ID        int64  `json:"id"`
        FirstName string `json:"first_name"`
        LastName  string `json:"last_name"`
        Nickname  string `json:"nickname"`
        Avatar    string `json:"avatar_path"`
    }
    wsh.DB.QueryRow(
        `SELECT id, first_name, last_name, nickname, avatar_path FROM users WHERE id = ?`,
        userID,
    ).Scan(&sender.ID, &sender.FirstName, &sender.LastName, &sender.Nickname, &sender.Avatar)

    // Create message response
    response := map[string]interface{}{
        "id":           messageID,
        "sender_id":    userID,
        "recipient_id": req.RecipientID,
        "content":      req.Content,
        "created_at":   time.Now(),
        "sender":       sender,
    }

    // Try to send via WebSocket. Routed through Hub.BroadcastToRoom rather
    // than reaching into wsh.Hub.Rooms directly - that map is mutated
    // (under lock) by the Hub's own goroutine on every connect/disconnect,
    // so reading it here without going through the same lock would be a
    // data race.
    roomID := "user_" + strconv.FormatInt(req.RecipientID, 10)
    data, _ := json.Marshal(response)
    wsMsg := map[string]interface{}{
        "type": "chat",
        "data": json.RawMessage(data),
    }
    msgBytes, _ := json.Marshal(wsMsg)
    wsh.Hub.BroadcastToRoom(roomID, msgBytes)

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "message":    "Message sent",
        "message_id": messageID,
        "data":       response,
    })
}

// GetGroupChatHistory - Get chat history for a group
func (wsh *WebSocketHandler) GetGroupChatHistory(w http.ResponseWriter, r *http.Request) {
    userID, _ := middleware.GetUserID(r)

    groupIDStr := r.URL.Query().Get("group_id")
    groupID, err := strconv.ParseInt(groupIDStr, 10, 64)
    if err != nil {
        http.Error(w, "Invalid group ID", http.StatusBadRequest)
        return
    }

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
         ORDER BY m.created_at DESC
         LIMIT 50`,
        groupID,
    )
    if err != nil {
        http.Error(w, "Failed to get group chat history", http.StatusInternalServerError)
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
        http.Error(w, "Error iterating messages", http.StatusInternalServerError)
        return
    }

    json.NewEncoder(w).Encode(messages)
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

    var notifications []Notification
    for rows.Next() {
        var n Notification
        err := rows.Scan(&n.ID, &n.Type, &n.Content, &n.IsRead, &n.RelatedID, &n.CreatedAt)
        if err != nil {
            continue
        }
        n.UserID = userID
        notifications = append(notifications, n)
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

    _, err := wsh.DB.Exec(
        "UPDATE notifications SET is_read = true WHERE id = ? AND user_id = ?",
        req.NotificationID, userID,
    )
    if err != nil {
        http.Error(w, "Failed to mark notification as read", http.StatusInternalServerError)
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
        http.Error(w, "Failed to mark notifications as read", http.StatusInternalServerError)
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
        http.Error(w, "Failed to get unread count", http.StatusInternalServerError)
        return
    }

    json.NewEncoder(w).Encode(map[string]int{
        "unread_count": count,
    })
}