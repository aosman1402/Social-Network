package websocket

import (
    "strconv"
    "sync"
)

type Hub struct {
    Clients    map[*Client]bool
    Rooms      map[string]map[*Client]bool
    Register   chan *Client
    Unregister chan *Client
    Broadcast  chan []byte
    mutex      sync.RWMutex
}

func NewHub() *Hub {
    return &Hub{
        Clients:    make(map[*Client]bool),
        Rooms:      make(map[string]map[*Client]bool),
        Register:   make(chan *Client),
        Unregister: make(chan *Client),
        Broadcast:  make(chan []byte),
    }
}

func (h *Hub) Run() {
    for {
        select {
        case client := <-h.Register:
            h.mutex.Lock()
            h.Clients[client] = true
            
            // Add to user room - FIXED: use strconv.FormatInt
            userRoom := "user_" + strconv.FormatInt(client.UserID, 10)
            if h.Rooms[userRoom] == nil {
                h.Rooms[userRoom] = make(map[*Client]bool)
            }
            h.Rooms[userRoom][client] = true
            client.RoomID = userRoom
            h.mutex.Unlock()

        case client := <-h.Unregister:
            h.mutex.Lock()
            if _, ok := h.Clients[client]; ok {
                delete(h.Clients, client)
                close(client.Send)
                
                // Remove from rooms
                if client.RoomID != "" {
                    if roomClients, ok := h.Rooms[client.RoomID]; ok {
                        delete(roomClients, client)
                        if len(roomClients) == 0 {
                            delete(h.Rooms, client.RoomID)
                        }
                    }
                }
            }
            h.mutex.Unlock()

        case message := <-h.Broadcast:
            h.mutex.RLock()
            for client := range h.Clients {
                select {
                case client.Send <- message:
                default:
                    close(client.Send)
                    delete(h.Clients, client)
                }
            }
            h.mutex.RUnlock()
        }
    }
}

func (h *Hub) BroadcastToRoom(roomID string, message []byte) {
    h.mutex.RLock()
    if clients, ok := h.Rooms[roomID]; ok {
        for client := range clients {
            select {
            case client.Send <- message:
            default:
                close(client.Send)
                delete(h.Clients, client)
            }
        }
    }
    h.mutex.RUnlock()
}

func (h *Hub) GetClientsInRoom(roomID string) []*Client {
    h.mutex.RLock()
    defer h.mutex.RUnlock()
    
    var clients []*Client
    if roomClients, ok := h.Rooms[roomID]; ok {
        for client := range roomClients {
            clients = append(clients, client)
        }
    }
    return clients
}

func (h *Hub) IsUserOnline(userID int64) bool {
    h.mutex.RLock()
    defer h.mutex.RUnlock()
    
    // FIXED: use strconv.FormatInt
    roomID := "user_" + strconv.FormatInt(userID, 10)
    if clients, ok := h.Rooms[roomID]; ok {
        return len(clients) > 0
    }
    return false
}