package handlers

import (
    "database/sql"
    "net/http"
    "time"

    "social-network3/pkg/middleware"
    "social-network3/pkg/models"
    "social-network3/pkg/websocket"

    gorilla "github.com/gorilla/websocket"
)

var upgrader = gorilla.Upgrader{
    CheckOrigin: func(r *http.Request) bool {
        origin := r.Header.Get("Origin")
        // No Origin header at all means a non-browser client (e.g. a
        // native app or a test script), which CORS/Origin checks don't
        // apply to in the first place.
        return origin == "" || middleware.AllowedOrigins[origin]
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
    if middleware.AllowedOrigins[origin] {
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
        writeJSONError(w, http.StatusUnauthorized, "Unauthorized - No token")
        return
    }
    token := cookie.Value

    if token == "" {
        writeJSONError(w, http.StatusUnauthorized, "Unauthorized - Empty token")
        return
    }

    var userID int64
    var expiresAt time.Time
    err = wsh.DB.QueryRow(
        "SELECT user_id, expires_at FROM sessions WHERE session_token = ?",
        token,
    ).Scan(&userID, &expiresAt)

    if err != nil {
        writeJSONError(w, http.StatusUnauthorized, "Unauthorized - Invalid token")
        return
    }

    if time.Now().After(expiresAt) {
        writeJSONError(w, http.StatusUnauthorized, "Unauthorized - Token expired")
        return
    }

    conn, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        writeJSONError(w, http.StatusInternalServerError, "Failed to upgrade connection")
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
        writeJSONError(w, http.StatusNotFound, "User not found")
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

