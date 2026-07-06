-- 070_engagement_native_mirror_down.sql
-- Reverses 070: drops the mirror tally columns and restores the global
-- (source, external_id) mirror-idempotency indexes from 068/069.

DROP INDEX IF EXISTS uniq_poll_overlay_source_external;
CREATE UNIQUE INDEX IF NOT EXISTS uniq_poll_source_external
    ON polls(source, external_id) WHERE external_id IS NOT NULL;

DROP INDEX IF EXISTS uniq_prediction_overlay_source_external;
CREATE UNIQUE INDEX IF NOT EXISTS uniq_prediction_source_external
    ON predictions(source, external_id) WHERE external_id IS NOT NULL;

ALTER TABLE poll_options DROP COLUMN IF EXISTS mirror_votes;
ALTER TABLE prediction_outcomes DROP COLUMN IF EXISTS mirror_points;
ALTER TABLE prediction_outcomes DROP COLUMN IF EXISTS mirror_entrants;
