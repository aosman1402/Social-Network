-- The user whose action produced the notification (e.g. who asked to join
-- a group). related_id already holds the group id for group notifications,
-- so without this there's no way to tell two join requests for the same
-- group apart - or to clear the right one once it has been answered.
-- Nullable: older rows and notification types that don't need it leave it
-- empty.
ALTER TABLE notifications ADD COLUMN actor_id INTEGER;
