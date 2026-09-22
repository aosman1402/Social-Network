-- A follow request notification stays in the recipient's list even after
-- the sender cancels it (so there's a record it happened), but the UI
-- needs a way to tell it's no longer actionable - shown struck through
-- instead of removed outright.
ALTER TABLE notifications ADD COLUMN is_cancelled BOOLEAN DEFAULT FALSE;
