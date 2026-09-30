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

	// OnPresenceChange, if set, is called (in its own goroutine, never
	// while the hub is locked) when a user's first connection opens or
	// their last one closes - i.e. when they come online or go offline.
	// It should read the current state with IsUserOnline rather than
	// assume one, since two quick changes can run in either order.
	OnPresenceChange func(userID int64)
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
			cameOnline := len(h.Rooms[userRoom]) == 0
			if h.Rooms[userRoom] == nil {
				h.Rooms[userRoom] = make(map[*Client]bool)
			}
			h.Rooms[userRoom][client] = true
			client.RoomID = userRoom
			h.mutex.Unlock()

			if cameOnline {
				h.presenceChanged(client.UserID)
			}

		case client := <-h.Unregister:
			h.mutex.Lock()
			h.removeClient(client)
			h.mutex.Unlock()

		case message := <-h.Broadcast:
			// Lock, not RLock: a full send buffer below closes and deletes
			// the client, which is a write. RLock only excludes writers
			// from readers - it does not stop two RLock holders (e.g. this
			// case and a concurrent BroadcastToRoom call) from both trying
			// to mutate the map at once.
			h.mutex.Lock()
			for client := range h.Clients {
				select {
				case client.Send <- message:
				default:
					h.removeClient(client)
				}
			}
			h.mutex.Unlock()
		}
	}
}

// removeClient closes the client's Send channel and removes it from both
// h.Clients and its room (if any). The caller must hold h.mutex (Lock, not
// RLock - this mutates the maps). Safe to call more than once for the same
// client - a no-op after the first call, which also guards against a
// double-close panic if a full send buffer is hit from two different call
// sites (Run's Broadcast case and a concurrent BroadcastToRoom) in quick
// succession.
func (h *Hub) removeClient(client *Client) {
	if _, ok := h.Clients[client]; !ok {
		return
	}
	delete(h.Clients, client)
	close(client.Send)

	if client.RoomID != "" {
		if roomClients, ok := h.Rooms[client.RoomID]; ok {
			delete(roomClients, client)
			if len(roomClients) == 0 {
				delete(h.Rooms, client.RoomID)
				// That was the user's last open connection.
				h.presenceChanged(client.UserID)
			}
		}
	}
}

// presenceChanged runs OnPresenceChange in its own goroutine, so it can
// take the hub lock itself (e.g. via IsUserOnline) - callers may hold it.
func (h *Hub) presenceChanged(userID int64) {
	if h.OnPresenceChange != nil {
		go h.OnPresenceChange(userID)
	}
}

// DisconnectSession closes every connection opened with sessionToken. Used
// on logout: a connection is only authenticated when it opens, so without
// this another tab of the logged-out session would keep receiving that
// user's messages and notifications. Returns how many were closed.
func (h *Hub) DisconnectSession(sessionToken string) int {
	if h == nil || sessionToken == "" {
		return 0
	}
	return h.disconnectWhere(func(c *Client) bool { return c.SessionToken == sessionToken })
}

// DisconnectUser closes every connection userID has open, e.g. after a
// password change signs out all of their sessions. Returns how many were
// closed.
func (h *Hub) DisconnectUser(userID int64) int {
	if h == nil {
		return 0
	}
	return h.disconnectWhere(func(c *Client) bool { return c.UserID == userID })
}

func (h *Hub) disconnectWhere(match func(*Client) bool) int {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	n := 0
	for client := range h.Clients {
		if match(client) {
			// Closing Send makes WritePump send a close frame and close
			// the socket, which in turn ends ReadPump.
			h.removeClient(client)
			n++
		}
	}
	return n
}

func (h *Hub) BroadcastToRoom(roomID string, message []byte) {
	// Lock, not RLock - see the comment in Run()'s Broadcast case.
	h.mutex.Lock()
	if clients, ok := h.Rooms[roomID]; ok {
		// removeClient mutates h.Rooms[roomID] (== clients) mid-iteration,
		// which Go allows for maps as long as we only delete the current
		// key - never add - during range.
		for client := range clients {
			select {
			case client.Send <- message:
			default:
				h.removeClient(client)
			}
		}
	}
	h.mutex.Unlock()
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

// NotifyUser pushes a notification to userID's live websocket connection(s),
// if any are open, so the frontend can show it immediately instead of
// waiting for the next page load/poll of /api/notifications. Every handler
// that creates a notification (posts, comments, follows, groups, events)
// calls this right after inserting it, so nothing requires a manual
// refresh to appear.
//
// h may be nil (contexts, e.g. tests, that don't wire a hub up) - safe to
// call unconditionally either way.
func (h *Hub) NotifyUser(userID int64, notification map[string]interface{}) {
	h.NotifyUserEvent(userID, "notification", notification)
}

// NotifyUserEvent is NotifyUser generalised to an arbitrary event type, for
// live updates that aren't themselves a new notification row - e.g. a
// cancelled follow request, which needs to update/remove something the
// client already has rather than being appended as a new list entry.
func (h *Hub) NotifyUserEvent(userID int64, eventType string, data interface{}) {
	if h == nil {
		return
	}
	broadcastEvent(h, "user_"+strconv.FormatInt(userID, 10), eventType, data)
}

// OnlineUserIDs returns the id of every user with at least one open
// connection, each listed once however many tabs they have open. Used to
// fan out live feed updates (new posts, comments, likes) to exactly the
// connected users allowed to see them, rather than to everyone.
func (h *Hub) OnlineUserIDs() []int64 {
	if h == nil {
		return nil
	}
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	seen := make(map[int64]bool)
	var ids []int64
	for client := range h.Clients {
		if !seen[client.UserID] {
			seen[client.UserID] = true
			ids = append(ids, client.UserID)
		}
	}
	return ids
}
