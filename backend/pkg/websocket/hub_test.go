package websocket

import "testing"

// TestBroadcastToRoom_RemovesClientOnFullBuffer guards against a real bug:
// when a client's Send buffer was full, the old code closed the channel
// and removed the client from h.Clients but never removed it from
// h.Rooms. The next broadcast to that room then did `client.Send <-
// message` on an already-closed channel, which panics in Go and would
// take the whole server down under any real load.
func TestBroadcastToRoom_RemovesClientOnFullBuffer(t *testing.T) {
    h := NewHub()
    client := &Client{Hub: h, Send: make(chan []byte, 1), UserID: 1, RoomID: "room1"}
    h.Clients[client] = true
    h.Rooms["room1"] = map[*Client]bool{client: true}

    h.BroadcastToRoom("room1", []byte("msg1")) // fills the buffer (capacity 1)
    h.BroadcastToRoom("room1", []byte("msg2")) // buffer still full -> must remove the client

    if _, ok := h.Clients[client]; ok {
        t.Error("client should have been removed from h.Clients after a full send buffer")
    }
    if _, ok := h.Rooms["room1"]; ok {
        t.Error("room should have been cleaned up (no members left) after the client was removed")
    }

    // This used to panic ("send on closed channel") because the room
    // still held a reference to a client whose Send channel was closed.
    h.BroadcastToRoom("room1", []byte("msg3"))
}

// TestRemoveClient_Idempotent guards against a related risk introduced by
// the fix above: if a full buffer is hit from two different call sites in
// quick succession (Run's own Broadcast case and a concurrent
// BroadcastToRoom), removeClient must be a safe no-op the second time,
// not a double-close panic.
func TestRemoveClient_Idempotent(t *testing.T) {
    h := NewHub()
    client := &Client{Hub: h, Send: make(chan []byte, 1), UserID: 1, RoomID: "room1"}
    h.Clients[client] = true
    h.Rooms["room1"] = map[*Client]bool{client: true}

    h.mutex.Lock()
    h.removeClient(client)
    h.removeClient(client) // must not double-close client.Send
    h.mutex.Unlock()
}

func TestBroadcastToRoom_UnknownRoomIsNoop(t *testing.T) {
    h := NewHub()
    // Must not panic when the room doesn't exist (e.g. everyone already
    // disconnected).
    h.BroadcastToRoom("nonexistent-room", []byte("msg"))
}
