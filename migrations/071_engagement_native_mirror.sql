-- 071_engagement_native_mirror.sql
-- Description: Twitch-native poll/prediction mirroring (issue #523, task H).
-- Twitch owns the individual votes/wagers of a native round, so tallies arrive
-- as per-option aggregates; the mirror_* columns store them (0 for allchat rows,
-- whose tallies stay computed from poll_votes / prediction_entries — read paths
-- report the sum, which is exact because each source only ever populates one side).
--
-- The mirror-idempotency indexes are created in final PER-OVERLAY scope directly in
-- 069/070 (a Twitch round fans out to every overlay sourcing the channel, so
-- (source, external_id) must be unique per overlay, not globally). This file NO LONGER
-- rescopes them: rescoping via DROP + CREATE would rebuild a GLOBAL unique from 069/070
-- on every migration re-run, which aborts once real multi-overlay data exists (P0-1).
-- Instead it only drops the RETIRED global index names as an idempotent no-op safety
-- net, for any dev/staging DB that applied an earlier revision of this branch's 069/070
-- (which created the global names). On a fresh DB those names never exist and the DROPs
-- are no-ops.

ALTER TABLE poll_options ADD COLUMN IF NOT EXISTS mirror_votes BIGINT NOT NULL DEFAULT 0;
ALTER TABLE prediction_outcomes ADD COLUMN IF NOT EXISTS mirror_points BIGINT NOT NULL DEFAULT 0;
ALTER TABLE prediction_outcomes ADD COLUMN IF NOT EXISTS mirror_entrants BIGINT NOT NULL DEFAULT 0;

-- Retire the global mirror-idempotency indexes from an earlier revision of 069/070.
-- No-op on a fresh DB (069/070 now create the per-overlay indexes directly).
DROP INDEX IF EXISTS uniq_poll_source_external;
DROP INDEX IF EXISTS uniq_prediction_source_external;
