-- Explicit viewer allowlist for posts with privacy_level = 'private' - the
-- set of users (chosen from the author's followers) who may see the post,
-- in addition to the author themselves.
CREATE TABLE IF NOT EXISTS post_allowed_users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    post_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    UNIQUE(post_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_post_allowed_users_post ON post_allowed_users(post_id);
CREATE INDEX IF NOT EXISTS idx_post_allowed_users_user ON post_allowed_users(user_id);
