-- Down migration for 052_overlay_last_connected_at.sql
--
-- Drops the index and the last_connected_at column from overlays.
--
-- Safe to run: dropping the column does not lose any user-configured state.
-- The column is purely an api-gateway-bumped activity marker; recreating it
-- and rolling forward simply restarts the grace period from the next deploy.

BEGIN;

DROP INDEX IF EXISTS idx_overlays_last_connected_at;

ALTER TABLE overlays
    DROP COLUMN IF EXISTS last_connected_at;

COMMIT;
