-- Tracks the IP a session was created from (logged/flagged on mismatch, not
-- invalidated - IPs legitimately change often on mobile/Wi-Fi) and the
-- client device fingerprint it was created with (invalidated on mismatch,
-- since that's a stronger signal the session is being used from a different
-- device than the one that logged in).
ALTER TABLE sessions ADD COLUMN ip_address TEXT;
ALTER TABLE sessions ADD COLUMN fingerprint TEXT;
