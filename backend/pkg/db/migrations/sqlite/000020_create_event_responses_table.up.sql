-- Each group member's answer to a group event. GetEvents and RespondToEvent
-- (pkg/handlers/group_events.go) have always used this table, but no
-- migration created it, so responding to an event failed.
CREATE TABLE IF NOT EXISTS event_responses (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    event_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    response TEXT NOT NULL CHECK (response IN ('going', 'not_going')),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (event_id) REFERENCES events(id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    UNIQUE(event_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_event_responses_event ON event_responses(event_id);
CREATE INDEX IF NOT EXISTS idx_event_responses_user ON event_responses(user_id);
