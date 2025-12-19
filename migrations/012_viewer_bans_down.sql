-- Rollback Migration 012: Remove banned flag from viewer_sessions

DROP INDEX IF EXISTS idx_viewer_sessions_banned;

ALTER TABLE viewer_sessions
DROP COLUMN IF EXISTS is_banned;

ALTER TABLE viewer_sessions
DROP COLUMN IF EXISTS banned_at;

ALTER TABLE viewer_sessions
DROP COLUMN IF EXISTS banned_reason;
