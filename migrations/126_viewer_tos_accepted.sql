-- Migration: 126_viewer_tos_accepted
-- Description: Add viewers.tos_accepted / viewers.tos_accepted_at for the Terms of
-- Service and Privacy Policy acceptance prompt, the viewer counterpart of
-- users.tos_accepted (migration 122).
--
-- FALSE means the frontend shows the blocking acceptance dialog to a signed-in
-- viewer; accepting sets TRUE and stamps tos_accepted_at.
--
-- Unlike migration 122, existing viewers are NOT backfilled to TRUE: no viewer has
-- ever been shown the terms, and many viewers rows were created by
-- engagement-service for chatters who never signed in at all. Every viewer is
-- therefore prompted once, on their next visit while signed in.
--
-- When the terms change, re-arm the prompt for everyone with:
--     UPDATE viewers SET tos_accepted = FALSE;
-- tos_accepted_at keeps the time of the last acceptance until the viewer accepts
-- the new version.
--
-- Re-runs (scripts/run-migrations.sh re-runs every migration on every pod start)
-- never touch viewer state: ADD COLUMN IF NOT EXISTS is a no-op once the column
-- exists.

ALTER TABLE viewers ADD COLUMN IF NOT EXISTS tos_accepted BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE viewers ADD COLUMN IF NOT EXISTS tos_accepted_at TIMESTAMP NULL;

COMMENT ON COLUMN viewers.tos_accepted IS
    'Whether the viewer accepted the current Terms of Service and Privacy Policy; FALSE = acceptance prompt is shown. Reset to FALSE when the terms change';
COMMENT ON COLUMN viewers.tos_accepted_at IS
    'When the viewer last accepted the Terms of Service and Privacy Policy';
