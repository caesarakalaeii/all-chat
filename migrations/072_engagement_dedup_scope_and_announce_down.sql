-- 072_engagement_dedup_scope_and_announce_down.sql
-- Reverses 071: drops the announce toggle + the LOWER(channel_id) functional index
-- and un-scopes the (source_message_id) replay-dedup indexes.
--
-- NOTE: the original 068/069 indexes were GLOBAL UNIQUE on source_message_id. Once 071
-- is live, one chat message legitimately fans out to several overlays' rounds sharing
-- one source_message_id, so restoring a GLOBAL UNIQUE index would abort on that data.
-- The down path therefore recreates these as plain (non-unique) indexes — the only
-- purpose of the source_message_id index is replay-dedup lookups, which the per-round
-- UNIQUE(round, source_message_id) scope already enforces. Global uniqueness across
-- rounds cannot be restored once multi-overlay dedup rows exist.

ALTER TABLE points_earn_config DROP COLUMN IF EXISTS announce_on_start;

DROP INDEX IF EXISTS idx_overlay_chat_sources_platform_lower_channel;

DROP INDEX IF EXISTS uniq_pred_entry_msg;
CREATE INDEX IF NOT EXISTS uniq_pred_entry_msg
    ON prediction_entries(source_message_id) WHERE source_message_id IS NOT NULL;

DROP INDEX IF EXISTS uniq_poll_vote_msg;
CREATE INDEX IF NOT EXISTS uniq_poll_vote_msg
    ON poll_votes(source_message_id) WHERE source_message_id IS NOT NULL;
