-- External login (OAuth) accounts linked to local users - e.g. a GitHub
-- account id. One local user can have one link per provider, and each
-- provider account links to exactly one local user.
CREATE TABLE IF NOT EXISTS oauth_accounts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    provider TEXT NOT NULL,
    provider_user_id TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    UNIQUE(provider, provider_user_id),
    UNIQUE(user_id, provider)
);

CREATE INDEX IF NOT EXISTS idx_oauth_accounts_user ON oauth_accounts(user_id);

-- A first-time OAuth login that still has to finish signing up: the
-- provider gave us a verified email and a name, but registration also
-- requires a date of birth, so the user is sent to a short form. The row
-- is looked up by its random one-time token and removed once used; it
-- expires after a few minutes.
CREATE TABLE IF NOT EXISTS oauth_pending_signups (
    token TEXT PRIMARY KEY,
    provider TEXT NOT NULL,
    provider_user_id TEXT NOT NULL,
    email TEXT NOT NULL,
    first_name TEXT,
    last_name TEXT,
    avatar_url TEXT,
    username TEXT,
    expires_at DATETIME NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
