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
            Type   string          `json:"type"`
            Data   json.RawMessage `json:"data"`
            RoomID string          `json:"room_id"`
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
                go handlePrivateMessage(db, c.UserID, chatData.RecipientID, chatData.Content, c.Hub)
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

            // Broadcast typing status
            if typingData.RecipientID != 0 {
                roomID := "user_" + strconv.FormatInt(typingData.RecipientID, 10)
                c.Hub.BroadcastToRoom(roomID, message)
            } else if typingData.GroupID != 0 {
                roomID := "group_" + strconv.FormatInt(typingData.GroupID, 10)
                c.Hub.BroadcastToRoom(roomID, message)
            }

        case "read_receipt":
            var readData struct {
                MessageID int64 `json:"message_id"`
            }
            if err := json.Unmarshal(wsMsg.Data, &readData); err != nil {
                log.Printf("Failed to parse read receipt data: %v", err)
                continue
            }

            go updateMessageReadStatus(db, readData.MessageID, c.UserID)

        case "join_room":
            if wsMsg.RoomID != "" {
                c.Hub.mutex.Lock()
                if c.RoomID != "" {
                    if roomClients, ok := c.Hub.Rooms[c.RoomID]; ok {
                        delete(roomClients, c)
                        if len(roomClients) == 0 {
                            delete(c.Hub.Rooms, c.RoomID)
                        }
                    }
                }
                if c.Hub.Rooms[wsMsg.RoomID] == nil {
                    c.Hub.Rooms[wsMsg.RoomID] = make(map[*Client]bool)
                }
                c.Hub.Rooms[wsMsg.RoomID][c] = true
                c.RoomID = wsMsg.RoomID
                c.Hub.mutex.Unlock()
            }

        case "leave_room":
            if wsMsg.RoomID != "" && c.RoomID == wsMsg.RoomID {
                c.Hub.mutex.Lock()
                if roomClients, ok := c.Hub.Rooms[c.RoomID]; ok {
                    delete(roomClients, c)
                    if len(roomClients) == 0 {
                        delete(c.Hub.Rooms, c.RoomID)
                    }
                }
                c.RoomID = ""
                c.Hub.mutex.Unlock()
            }

        case "ping":
            pongMsg := map[string]string{"type": "pong"}
            data, _ := json.Marshal(pongMsg)
            c.Send <- data
        }
    }
}

func handlePrivateMessage(db *sql.DB, senderID, recipientID int64, content string, hub *Hub) {
    // Check if users can chat
    var count int
    db.QueryRow(
        `SELECT COUNT(*) FROM follows 
         WHERE (follower_id = ? AND following_id = ? AND status = 'accepted')
         OR (follower_id = ? AND following_id = ? AND status = 'accepted')`,
        senderID, recipientID, recipientID, senderID,
    ).Scan(&count)

    var isPublic bool
    db.QueryRow("SELECT is_public FROM users WHERE id = ?", recipientID).Scan(&isPublic)

    if count == 0 && !isPublic {
        return
    }

    // Save message
    result, err := db.Exec(
        `INSERT INTO messages (sender_id, recipient_id, content, is_group_message, created_at)
         VALUES (?, ?, ?, ?, ?)`,
        senderID, recipientID, content, false, time.Now(),
    )
    if err != nil {
        log.Printf("Failed to save message: %v", err)
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
        "id":           messageID,
        "sender_id":    senderID,
        "recipient_id": recipientID,
        "content":      content,
        "created_at":   time.Now(),
        "sender":       sender,
    }
    data, _ := json.Marshal(msg)

    wsMsg := map[string]interface{}{
        "type":    "chat",
        "data":    json.RawMessage(data),
        "room_id": "user_" + strconv.FormatInt(recipientID, 10),
    }
    msgBytes, _ := json.Marshal(wsMsg)

    // Send to recipient if online
    hub.BroadcastToRoom("user_"+strconv.FormatInt(recipientID, 10), msgBytes)

    // Send acknowledgment to sender
    ackMsg := map[string]interface{}{
        "type":    "ack",
        "data":    json.RawMessage(`{"message":"sent","message_id":`+strconv.FormatInt(messageID, 10)+`}`),
        "room_id": "user_" + strconv.FormatInt(senderID, 10),
    }
    ackBytes, _ := json.Marshal(ackMsg)
    hub.BroadcastToRoom("user_"+strconv.FormatInt(senderID, 10), ackBytes)
}

func handleGroupMessage(db *sql.DB, senderID, groupID int64, content string, hub *Hub) {
    // Check if user is a member
    var count int
    db.QueryRow(
        "SELECT COUNT(*) FROM group_members WHERE group_id = ? AND user_id = ? AND status = 'active'",
        groupID, senderID,
    ).Scan(&count)

    if count == 0 {
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
    data, _ := json.Marshal(msg)

    wsMsg := map[string]interface{}{
        "type":    "chat",
        "data":    json.RawMessage(data),
        "room_id": "group_" + strconv.FormatInt(groupID, 10),
    }
    msgBytes, _ := json.Marshal(wsMsg)

    // Broadcast to all group members
    hub.BroadcastToRoom("group_"+strconv.FormatInt(groupID, 10), msgBytes)
}

func updateMessageReadStatus(db *sql.DB, messageID int64, userID int64) {
    // Use the db parameter
    var recipientID sql.NullInt64
    err := db.QueryRow(
        "SELECT recipient_id FROM messages WHERE id = ?",
        messageID,
    ).Scan(&recipientID)
    
    if err != nil {
        log.Printf("Failed to get message recipient: %v", err)
        return
    }
    
    if recipientID.Valid && recipientID.Int64 == userID {
        log.Printf("Message %d marked as read by user %d", messageID, userID)
    }
}