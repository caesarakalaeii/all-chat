-- Migration: 101_alert_events (down)
-- Description: Remove the alert-event history table. This is destructive to
--   alert history by design; the down migration exists to make the pair
--   reversible in development only.

BEGIN;

DROP INDEX IF EXISTS idx_alert_events_overlay_occurred;
DROP TABLE IF EXISTS alert_events;

COMMIT;
