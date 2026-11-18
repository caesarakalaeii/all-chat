-- Migration: 118_retention_sweeper (down)
-- Description: Reverses 118_retention_sweeper.sql by dropping its indexes.
--
-- The dropped cleanup functions need no restore here: 016, 046 and 063 recreate them on the
-- next run of the migration set once 118 is gone, exactly as they did before it. Stop the
-- retention-sweeper CronJob as well, or the periods stay enforced.

BEGIN;

DROP INDEX IF EXISTS idx_alert_events_created_at;
DROP INDEX IF EXISTS idx_moderation_actions_created_at;

COMMIT;
