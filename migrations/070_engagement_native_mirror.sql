-- 070_engagement_native_mirror.sql
-- Description: Twitch-native poll/prediction mirroring (issue #523, task H).
-- Twitch owns the individual votes/wagers of a native round, so tallies arrive
-- as per-option aggregates; the mirror_* columns store them (0 for allchat rows,
-- whose tallies stay computed from poll_votes / prediction_entries — read paths
-- report the sum, which is exact because each source only ever populates one side).
-- Also rescopes the mirror-idempotency indexes to per-overlay: one Twitch poll
-- fans out to every overlay sourcing the channel, so (source, external_id) must
-- be unique per overlay, not globally.

ALTER TABLE poll_options ADD COLUMN IF NOT EXISTS mirror_votes BIGINT NOT NULL DEFAULT 0;
ALTER TABLE prediction_outcomes ADD COLUMN IF NOT EXISTS mirror_points BIGINT NOT NULL DEFAULT 0;
ALTER TABLE prediction_outcomes ADD COLUMN IF NOT EXISTS mirror_entrants BIGINT NOT NULL DEFAULT 0;

DROP INDEX IF EXISTS uniq_poll_source_external;
CREATE UNIQUE INDEX IF NOT EXISTS uniq_poll_overlay_source_external
    ON polls(overlay_id, source, external_id) WHERE external_id IS NOT NULL;

DROP INDEX IF EXISTS uniq_prediction_source_external;
CREATE UNIQUE INDEX IF NOT EXISTS uniq_prediction_overlay_source_external
    ON predictions(overlay_id, source, external_id) WHERE external_id IS NOT NULL;
