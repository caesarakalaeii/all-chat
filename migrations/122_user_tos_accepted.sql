-- Migration: 122_user_tos_accepted
-- Description: Add users.tos_accepted / users.tos_accepted_at for the post-login
-- Terms of Service and Privacy Policy acceptance prompt.
--
-- FALSE means the frontend shows the blocking acceptance dialog after login;
-- accepting sets TRUE and stamps tos_accepted_at.
--
-- Existing users start at TRUE: adding the column with DEFAULT TRUE fills every
-- existing row, so nobody is prompted when this ships. The default then drops
-- to FALSE, so every account created afterwards accepts on its first login.
--
-- When the terms change, re-arm the prompt for everyone with:
--     UPDATE users SET tos_accepted = FALSE;
-- tos_accepted_at is deliberately left alone by that reset: it keeps the time
-- of the last acceptance until the user accepts the new version (NULL for
-- users backfilled here who have never seen the prompt).
--
-- Re-runs (scripts/run-migrations.sh re-runs every migration on every pod
-- start) never touch user state: ADD COLUMN IF NOT EXISTS is a no-op once the
-- column exists, and SET DEFAULT only affects rows inserted later.

ALTER TABLE users ADD COLUMN IF NOT EXISTS tos_accepted BOOLEAN NOT NULL DEFAULT TRUE;
ALTER TABLE users ALTER COLUMN tos_accepted SET DEFAULT FALSE;
ALTER TABLE users ADD COLUMN IF NOT EXISTS tos_accepted_at TIMESTAMP NULL;

COMMENT ON COLUMN users.tos_accepted IS
    'Whether the user accepted the current Terms of Service and Privacy Policy; FALSE = acceptance prompt is shown. Reset to FALSE when the terms change';
COMMENT ON COLUMN users.tos_accepted_at IS
    'When the user last accepted the Terms of Service and Privacy Policy';
