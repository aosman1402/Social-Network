package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"social-network/pkg/middleware"
	"social-network/pkg/models"
)

const (
	maxEventTitleLength       = 100
	maxEventDescriptionLength = 1000
)

// CreateEvent - Create a group event
func (gh *GroupHandler) CreateEvent(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.GetUserID(r)

	var req struct {
		GroupID     int64  `json:"group_id"`
		Title       string `json:"title"`
		Description string `json:"description"`
		EventTime   string `json:"event_time"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	req.Title = strings.TrimSpace(req.Title)
	req.Description = strings.TrimSpace(req.Description)
	if req.Title == "" || req.EventTime == "" {
		writeJSONError(w, http.StatusBadRequest, "Title and time are required")
		return
	}
	if utf8.RuneCountInString(req.Title) > maxEventTitleLength {
		writeJSONError(w, http.StatusBadRequest, "Title must be at most 100 characters")
		return
	}
	if utf8.RuneCountInString(req.Description) > maxEventDescriptionLength {
		writeJSONError(w, http.StatusBadRequest, "Description must be at most 1000 characters")
		return
	}

	// Stored in UTC (below) and sorted with datetime(): event_time is
	// compared as text by SQLite, so "10:00+05:00" (05:00 UTC) used to sort
	// after "08:00+00:00".
	eventTime, err := time.Parse(time.RFC3339, req.EventTime)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid event time format")
		return
	}
	// A minute of slack so an event set for "now" isn't rejected by the
	// time the request arrives.
	if eventTime.Before(time.Now().Add(-time.Minute)) {
		writeJSONError(w, http.StatusBadRequest, "Event time can't be in the past")
		return
	}

	var count int
	err = gh.DB.QueryRow(
		"SELECT COUNT(*) FROM group_members WHERE group_id = ? AND user_id = ? AND status = 'active'",
		req.GroupID, userID,
	).Scan(&count)
	if err != nil || count == 0 {
		writeJSONError(w, http.StatusForbidden, "You are not a member of this group")
		return
	}

	result, err := gh.DB.Exec(
		`INSERT INTO events (group_id, creator_id, title, description, event_time, created_at, updated_at)
         VALUES (?, ?, ?, ?, ?, ?, ?)`,
		req.GroupID, userID, req.Title, req.Description, eventTime.UTC(), time.Now(), time.Now(),
	)
	if err != nil {
		log.Printf("CreateEvent: failed to insert event: %v", err)
		writeJSONError(w, http.StatusInternalServerError, "Failed to create event")
		return
	}

	eventID, _ := result.LastInsertId()

	creator := models.User{ID: userID}
	gh.DB.QueryRow(
		"SELECT first_name, last_name FROM users WHERE id = ?",
		userID,
	).Scan(&creator.FirstName, &creator.LastName)

	var groupTitle string
	gh.DB.QueryRow("SELECT title FROM groups WHERE id = ?", req.GroupID).Scan(&groupTitle)

	rows, err := gh.DB.Query(
		"SELECT user_id FROM group_members WHERE group_id = ? AND status = 'active' AND user_id != ?",
		req.GroupID, userID,
	)
	if err == nil {
		defer rows.Close()
		var memberIDs []int64
		for rows.Next() {
			var memberID int64
			if err := rows.Scan(&memberID); err == nil {
				memberIDs = append(memberIDs, memberID)
			}
		}
		if err = rows.Err(); err != nil {
			writeJSONError(w, http.StatusInternalServerError, "Error iterating members")
			return
		}

		content := "New event in " + groupTitle + ": " + req.Title
		createdAt := time.Now()
		for _, memberID := range memberIDs {
			// related_id is the group (not the event - there's no
			// per-event page to link to, so a click takes members to the
			// group, same as group_invitation/group_request_accepted).
			result, err := gh.DB.Exec(
				`INSERT INTO notifications (user_id, type, content, related_id, created_at)
                 VALUES (?, 'event_created', ?, ?, ?)`,
				memberID, content, req.GroupID, createdAt,
			)
			if err != nil {
				continue
			}
			notificationID, _ := result.LastInsertId()
			gh.Hub.NotifyUser(memberID, map[string]interface{}{
				"id":         notificationID,
				"user_id":    memberID,
				"type":       "event_created",
				"content":    content,
				"is_read":    false,
				"related_id": req.GroupID,
				"created_at": createdAt,
			})
		}
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":  "Event created successfully",
		"event_id": eventID,
		"event": map[string]interface{}{
			"id":          eventID,
			"group_id":    req.GroupID,
			"title":       req.Title,
			"description": req.Description,
			"event_time":  eventTime,
			"created_at":  time.Now(),
			"creator":     creator,
		},
	})
}

// GetEvents - Get events for a group
func (gh *GroupHandler) GetEvents(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.GetUserID(r)

	groupIDStr := r.URL.Query().Get("group_id")
	groupID, err := strconv.ParseInt(groupIDStr, 10, 64)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid group ID")
		return
	}

	var count int
	err = gh.DB.QueryRow(
		"SELECT COUNT(*) FROM group_members WHERE group_id = ? AND user_id = ? AND status = 'active'",
		groupID, userID,
	).Scan(&count)
	if err != nil || count == 0 {
		writeJSONError(w, http.StatusForbidden, "You are not a member of this group")
		return
	}

	limit, offset := parsePagination(r, 50, 100)
	rows, err := gh.DB.Query(
		`SELECT e.id, e.group_id, e.creator_id, e.title, e.description, e.event_time, e.created_at,
                u.id, u.email, u.first_name, u.last_name, u.nickname, u.avatar_path, u.is_public
         FROM events e
         JOIN users u ON e.creator_id = u.id
         WHERE e.group_id = ?
         ORDER BY datetime(e.event_time) ASC
         LIMIT ? OFFSET ?`,
		groupID, limit, offset,
	)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to get events")
		return
	}
	defer rows.Close()

	events := []map[string]interface{}{}
	for rows.Next() {
		var event struct {
			ID          int64     `json:"id"`
			GroupID     int64     `json:"group_id"`
			CreatorID   int64     `json:"creator_id"`
			Title       string    `json:"title"`
			Description *string   `json:"description"`
			EventTime   time.Time `json:"event_time"`
			CreatedAt   time.Time `json:"created_at"`
		}
		var creator models.User

		err := rows.Scan(
			&event.ID, &event.GroupID, &event.CreatorID, &event.Title, &event.Description,
			&event.EventTime, &event.CreatedAt,
			&creator.ID, &creator.Email, &creator.FirstName, &creator.LastName,
			&creator.Nickname, &creator.AvatarPath, &creator.IsPublic,
		)
		if err != nil {
			continue
		}

		var userResponse string
		gh.DB.QueryRow(
			"SELECT response FROM event_responses WHERE event_id = ? AND user_id = ?",
			event.ID, userID,
		).Scan(&userResponse)

		responses, err := gh.eventResponses(event.ID)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "Failed to get event responses")
			return
		}

		events = append(events, map[string]interface{}{
			"id":            event.ID,
			"group_id":      event.GroupID,
			"creator":       creator,
			"title":         event.Title,
			"description":   event.Description,
			"event_time":    event.EventTime,
			"created_at":    event.CreatedAt,
			"user_response": userResponse,
			"responses":     responses,
		})
	}

	if err = rows.Err(); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Error iterating events")
		return
	}

	json.NewEncoder(w).Encode(events)
}

// DeleteEvent - DELETE /api/group/event/delete?id=N: the event's creator
// cancels it (the same rule as deleting a post: only its author). Its
// votes go with it (ON DELETE CASCADE), and members viewing the group get
// a live "event_deleted" so the event disappears from their list.
func (gh *GroupHandler) DeleteEvent(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.GetUserID(r)

	if r.Method != http.MethodDelete && r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	eventID, err := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid event ID")
		return
	}

	var creatorID, groupID int64
	err = gh.DB.QueryRow(
		"SELECT creator_id, group_id FROM events WHERE id = ?", eventID,
	).Scan(&creatorID, &groupID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "Event not found")
		return
	}
	if creatorID != userID {
		writeJSONError(w, http.StatusForbidden, "Only the event's creator can cancel it")
		return
	}

	if _, err := gh.DB.Exec("DELETE FROM events WHERE id = ?", eventID); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to cancel event")
		return
	}

	rows, err := gh.DB.Query(
		"SELECT user_id FROM group_members WHERE group_id = ? AND status = 'active'", groupID,
	)
	if err == nil {
		var memberIDs []int64
		for rows.Next() {
			var id int64
			if rows.Scan(&id) == nil {
				memberIDs = append(memberIDs, id)
			}
		}
		rows.Close()
		for _, id := range memberIDs {
			gh.Hub.NotifyUserEvent(id, "event_deleted", map[string]interface{}{
				"event_id": eventID,
				"group_id": groupID,
			})
		}
	}

	json.NewEncoder(w).Encode(map[string]string{
		"message": "Event cancelled",
	})
}

// eventResponses lists who answered eventID and how. Its own function so
// the rows are closed after each event - the loop in GetEvents used to
// defer every event's Close until the whole handler returned.
func (gh *GroupHandler) eventResponses(eventID int64) ([]map[string]interface{}, error) {
	rows, err := gh.DB.Query(
		`SELECT er.response, er.created_at,
                u.id, u.email, u.first_name, u.last_name, u.nickname, u.avatar_path, u.is_public
         FROM event_responses er
         JOIN users u ON er.user_id = u.id
         WHERE er.event_id = ?
         ORDER BY er.created_at ASC`,
		eventID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	responses := []map[string]interface{}{}
	for rows.Next() {
		var user models.User
		var response string
		var createdAt time.Time
		if err := rows.Scan(
			&response, &createdAt,
			&user.ID, &user.Email, &user.FirstName, &user.LastName,
			&user.Nickname, &user.AvatarPath, &user.IsPublic,
		); err != nil {
			continue
		}
		responses = append(responses, map[string]interface{}{
			"user":       user,
			"response":   response,
			"created_at": createdAt,
		})
	}
	return responses, rows.Err()
}

// RespondToEvent - Respond to an event (going/not going)
func (gh *GroupHandler) RespondToEvent(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.GetUserID(r)

	var req struct {
		EventID  int64  `json:"event_id"`
		Response string `json:"response"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Response != "going" && req.Response != "not_going" {
		writeJSONError(w, http.StatusBadRequest, "Invalid response. Must be 'going' or 'not_going'")
		return
	}

	var groupID int64
	err := gh.DB.QueryRow(
		"SELECT group_id FROM events WHERE id = ?",
		req.EventID,
	).Scan(&groupID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "Event not found")
		return
	}

	var count int
	err = gh.DB.QueryRow(
		"SELECT COUNT(*) FROM group_members WHERE group_id = ? AND user_id = ? AND status = 'active'",
		groupID, userID,
	).Scan(&count)
	if err != nil || count == 0 {
		writeJSONError(w, http.StatusForbidden, "You are not a member of this group")
		return
	}

	_, err = gh.DB.Exec(
		`INSERT INTO event_responses (event_id, user_id, response, created_at, updated_at)
         VALUES (?, ?, ?, ?, ?)
         ON CONFLICT(event_id, user_id) DO UPDATE SET response = ?, updated_at = ?`,
		req.EventID, userID, req.Response, time.Now(), time.Now(), req.Response, time.Now(),
	)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to respond to event")
		return
	}

	// Let members viewing the group refresh the event's going / not going
	// counts live. Not a notification - nothing is stored or counted.
	memberRows, err := gh.DB.Query(
		"SELECT user_id FROM group_members WHERE group_id = ? AND status = 'active'",
		groupID,
	)
	if err == nil {
		var memberIDs []int64
		for memberRows.Next() {
			var id int64
			if memberRows.Scan(&id) == nil {
				memberIDs = append(memberIDs, id)
			}
		}
		memberRows.Close()
		for _, id := range memberIDs {
			gh.Hub.NotifyUserEvent(id, "event_response", map[string]interface{}{
				"event_id": req.EventID,
				"group_id": groupID,
			})
		}
	}

	json.NewEncoder(w).Encode(map[string]string{
		"message": "Response recorded successfully",
	})
}
