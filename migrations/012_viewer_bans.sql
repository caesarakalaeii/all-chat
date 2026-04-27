-- Migration 012: Add banned flag to viewer_sessions
-- Allows admins to ban viewers from sending messages

ALTER TABLE viewer_sessions
ADD COLUMN IF NOT EXISTS is_banned BOOLEAN NOT NULL DEFAULT false;

ALTER TABLE viewer_sessions
ADD COLUMN IF NOT EXISTS banned_at TIMESTAMP NULL;

ALTER TABLE viewer_sessions
ADD COLUMN IF NOT EXISTS banned_reason TEXT NULL;

-- Index for quickly finding banned users
CREATE INDEX IF NOT EXISTS idx_viewer_sessions_banned ON viewer_sessions(is_banned) WHERE is_banned = true;

-- Comments
COMMENT ON COLUMN viewer_sessions.is_banned IS 'Whether this viewer is banned from sending messages';
COMMENT ON COLUMN viewer_sessions.banned_at IS 'When the viewer was banned';
COMMENT ON COLUMN viewer_sessions.banned_reason IS 'Reason for banning (for admin reference)';
