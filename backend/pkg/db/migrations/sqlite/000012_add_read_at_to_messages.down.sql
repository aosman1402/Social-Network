DROP INDEX IF EXISTS idx_messages_read_at;

ALTER TABLE messages DROP COLUMN read_at;
