-- 072_engagement_dedup_scope_and_announce.sql
-- Description: two engagement follow-ups (PR #524 review).
--
-- 1. Case-fold the channel lookup. The native-mirror producer stores channel_id as
--    the lowercase Twitch login, but overlay_chat_sources.channel_id keeps the
--    streamer's original casing, so a case-sensitive '=' can miss and the round
--    mirrors to zero overlays. A functional index supports LOWER(channel_id) lookups
--    (see OverlaysForChannel).
--
-- 2. announce_on_start: opt-in "post the round question + numbered options + the
--    participate link to chat" toggle. Default FALSE — it needs the Twitch send
--    scope (user:write:chat), so it stays off unless the streamer enables it.
--
-- The chat replay-dedup indexes are created in final PER-ROUND scope directly in
-- 069/070 (one chat message fans out to several overlays sourcing the same channel,
-- ADR-0028, so a GLOBAL unique on source_message_id would drop the 2nd+ overlay's vote).
-- This file NO LONGER rescopes them: a DROP + CREATE rescope rebuilds a global unique
-- from 069/070 on every migration re-run, which aborts over real data (P0-1). It only
-- drops the RETIRED global index names as an idempotent no-op safety net for any
-- dev/staging DB that applied an earlier revision of 069/070.

-- Retire the global replay-dedup indexes from an earlier revision of 069/070. No-op on
-- a fresh DB (069/070 now create uniq_poll_vote_msg_round / uniq_pred_entry_msg_round).
DROP INDEX IF EXISTS uniq_poll_vote_msg;
DROP INDEX IF EXISTS uniq_pred_entry_msg;

CREATE INDEX IF NOT EXISTS idx_overlay_chat_sources_platform_lower_channel
    ON overlay_chat_sources(platform, LOWER(channel_id));

ALTER TABLE points_earn_config ADD COLUMN IF NOT EXISTS announce_on_start BOOLEAN NOT NULL DEFAULT FALSE;
