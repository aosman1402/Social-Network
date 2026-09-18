package handlers

import (
    "encoding/json"
    "log"
    "net/http"
    "strconv"
    "time"

    "social-network3/pkg/middleware"
    "social-network3/pkg/models"
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

    if req.Title == "" || req.EventTime == "" {
        writeJSONError(w, http.StatusBadRequest, "Title and time are required")
        return
    }

    eventTime, err := time.Parse(time.RFC3339, req.EventTime)
    if err != nil {
        writeJSONError(w, http.StatusBadRequest, "Invalid event time format")
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
        req.GroupID, userID, req.Title, req.Description, eventTime, time.Now(), time.Now(),
    )
    if err != nil {
        log.Printf("CreateEvent: failed to insert event: %v", err)
        writeJSONError(w, http.StatusInternalServerError, "Failed to create event")
        return
    }

    eventID, _ := result.LastInsertId()

    var creator models.User
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
        for rows.Next() {
            var memberID int64
            rows.Scan(&memberID)
            gh.DB.Exec(
                `INSERT INTO notifications (user_id, type, content, related_id, created_at)
                 VALUES (?, 'event_created', ?, ?, ?)`,
                memberID, "New event in "+groupTitle+": "+req.Title, eventID, time.Now(),
            )
        }
        if err = rows.Err(); err != nil {
            writeJSONError(w, http.StatusInternalServerError, "Error iterating members")
            return
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
         ORDER BY e.event_time ASC
         LIMIT ? OFFSET ?`,
        groupID, limit, offset,
    )
    if err != nil {
        writeJSONError(w, http.StatusInternalServerError, "Failed to get events")
        return
    }
    defer rows.Close()

    var events []map[string]interface{}
    for rows.Next() {
        var event struct {
            ID          int64      `json:"id"`
            GroupID     int64      `json:"group_id"`
            CreatorID   int64      `json:"creator_id"`
            Title       string     `json:"title"`
            Description *string    `json:"description"`
            EventTime   time.Time  `json:"event_time"`
            CreatedAt   time.Time  `json:"created_at"`
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

        var responses []map[string]interface{}
        rows2, err := gh.DB.Query(
            `SELECT er.user_id, er.response, er.created_at,
                    u.id, u.email, u.first_name, u.last_name, u.nickname, u.avatar_path, u.is_public
             FROM event_responses er
             JOIN users u ON er.user_id = u.id
             WHERE er.event_id = ?`,
            event.ID,
        )
        if err == nil {
            defer rows2.Close()
            for rows2.Next() {
                var user models.User
                var response string
                var createdAt time.Time
                err := rows2.Scan(
                    &user.ID, &response, &createdAt,
                    &user.ID, &user.Email, &user.FirstName, &user.LastName,
                    &user.Nickname, &user.AvatarPath, &user.IsPublic,
                )
                if err == nil {
                    responses = append(responses, map[string]interface{}{
                        "user":      user,
                        "response":  response,
                        "created_at": createdAt,
                    })
                }
            }
            if err = rows2.Err(); err != nil {
                writeJSONError(w, http.StatusInternalServerError, "Error iterating responses")
                return
            }
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

    json.NewEncoder(w).Encode(map[string]string{
        "message": "Response recorded successfully",
    })
}
