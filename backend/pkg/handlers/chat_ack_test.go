package handlers

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// TestDirectMessageDeliveredOnceWithAck - over the WebSocket, the recipient
// gets the message exactly once, and the sender gets no copy of it (their
// page shows its own optimistic copy) but exactly one ack carrying back the
// client_id they sent, so the page can swap in the real message id.
func TestDirectMessageDeliveredOnceWithAck(t *testing.T) {
	env := newWSTestEnv(t)
	senderID, senderConn := env.connect("sender@example.com")
	recipientID, recipientConn := env.connect("recipient@example.com")
	insertAcceptedFollow(t, env.wsh.DB, senderID, recipientID)

	writeFrame(t, senderConn, map[string]interface{}{
		"type": "chat",
		"data": map[string]interface{}{
			"recipient_id": recipientID,
			"content":      "just once please",
			"client_id":    "tmp-42",
		},
	})

	var ack struct {
		Type string `json:"type"`
		Data struct {
			MessageID   int64  `json:"message_id"`
			ClientID    string `json:"client_id"`
			RecipientID int64  `json:"recipient_id"`
		} `json:"data"`
	}
	senderFrames := receivedFrames(senderConn, 400*time.Millisecond)
	if len(senderFrames) != 1 {
		t.Fatalf("sender got %d frames, want exactly one ack: %v", len(senderFrames), senderFrames)
	}
	if err := json.Unmarshal([]byte(senderFrames[0]), &ack); err != nil || ack.Type != "ack" {
		t.Fatalf("sender frame is not an ack: %s", senderFrames[0])
	}
	if ack.Data.ClientID != "tmp-42" || ack.Data.MessageID == 0 || ack.Data.RecipientID != recipientID {
		t.Errorf("ack = %+v, want client_id tmp-42, a message id and the recipient", ack.Data)
	}

	if n := chatFramesContaining(receivedFrames(recipientConn, 200*time.Millisecond), "just once please"); n != 1 {
		t.Errorf("recipient got %d copies, want 1", n)
	}
}

// TestBlockedDirectMessageErrorCarriesClientID - so the page removes only
// the failed optimistic message, not every pending one.
func TestBlockedDirectMessageErrorCarriesClientID(t *testing.T) {
	env := newWSTestEnv(t)
	_, senderConn := env.connect("sender@example.com")
	strangerID, _ := env.connect("stranger@example.com")

	writeFrame(t, senderConn, map[string]interface{}{
		"type": "chat",
		"data": map[string]interface{}{"recipient_id": strangerID, "content": "hi", "client_id": "tmp-7"},
	})

	frames := receivedFrames(senderConn, 400*time.Millisecond)
	if len(frames) != 1 || !strings.Contains(frames[0], `"type":"chat_error"`) || !strings.Contains(frames[0], `"client_id":"tmp-7"`) {
		t.Fatalf("got %v, want one chat_error with client_id tmp-7", frames)
	}
}
