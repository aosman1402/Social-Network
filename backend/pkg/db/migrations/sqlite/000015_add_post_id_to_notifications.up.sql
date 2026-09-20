-- Lets "like"/"comment" notifications carry the target post's ID directly.
-- related_id already serves double duty across notification types (the
-- other user's ID for follows, the group ID for group notifications) - for
-- "comment" it's the comment's own ID (used to highlight that specific
-- comment), so the post it belongs to needs its own column rather than
-- overloading related_id further.
ALTER TABLE notifications ADD COLUMN post_id INTEGER REFERENCES posts(id) ON DELETE CASCADE;

CREATE INDEX idx_notifications_post ON notifications(post_id);
