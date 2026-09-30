package handlers

import (
	"database/sql"

	"social-network/pkg/websocket"
)

// onlinePostViewers returns every currently-connected user allowed to see
// postID under the same visibility rule the feed/profile/group endpoints
// use (canViewPost), including the author's own other tabs. Call it
// before deleting a post - once the row is gone nobody can view it, so
// the viewer set has to be captured first.
func onlinePostViewers(db *sql.DB, hub *websocket.Hub, postID int64) []int64 {
	var viewers []int64
	for _, uid := range hub.OnlineUserIDs() {
		if ok, err := canViewPost(db, postID, uid); err == nil && ok {
			viewers = append(viewers, uid)
		}
	}
	return viewers
}

// broadcastToPostViewers pushes a live update about postID (a new post,
// comment, or like) to every connected user allowed to see that post, so
// open feeds, profiles and group pages update without a refresh - and
// nobody receives an event about a post they couldn't otherwise see.
func broadcastToPostViewers(db *sql.DB, hub *websocket.Hub, postID int64, eventType string, data interface{}) {
	sendToUsers(hub, onlinePostViewers(db, hub, postID), eventType, data)
}

func sendToUsers(hub *websocket.Hub, userIDs []int64, eventType string, data interface{}) {
	for _, uid := range userIDs {
		hub.NotifyUserEvent(uid, eventType, data)
	}
}
