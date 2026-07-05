-- All-Chat Migration 067 DOWN: remove time-limited admin premium override expiry.
-- Reverts 067_premium_admin_override_expiry.sql (ADR-0027). Dropping the columns
-- discards any pending expiry, reverting every live admin override to permanent —
-- acceptable for a rollback (it never removes premium, only its time limit).

BEGIN;

DROP INDEX IF EXISTS idx_viewers_premium_override_expiry;
ALTER TABLE viewers DROP COLUMN IF EXISTS premium_admin_override_expires_at;

DROP INDEX IF EXISTS idx_users_premium_override_expiry;
ALTER TABLE users DROP COLUMN IF EXISTS premium_admin_override_expires_at;

COMMIT;
