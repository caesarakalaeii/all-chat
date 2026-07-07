-- 072_engagement_dedup_scope_and_announce_down.sql
-- Reverses 072: drops the announce toggle + the LOWER(channel_id) functional index.
-- 072 no longer rescopes the replay-dedup indexes (069/070 create them per-round
-- directly as uniq_poll_vote_msg_round / uniq_pred_entry_msg_round), so this down path
-- deliberately does NOT touch them — they are dropped with poll_votes / prediction_entries
-- in 069/070's down migrations. It also does not recreate the retired global source_message_id
-- indexes: a global unique over real multi-overlay data would abort (P0-1).
ALTER TABLE points_earn_config DROP COLUMN IF EXISTS announce_on_start;
DROP INDEX IF EXISTS idx_overlay_chat_sources_platform_lower_channel;
