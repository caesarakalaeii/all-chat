-- 071_engagement_dedup_scope_and_announce.sql
-- Description: three engagement follow-ups (PR #524 review).
--
-- 1. Rescope the chat replay-dedup indexes to the round. A single chat message can
--    fan out to several overlays that source the same channel (ADR-0027); the vote
--    or wager recorded for each overlay carries the SAME source_message_id, so a
--    GLOBAL unique index on source_message_id trips a unique_violation on the 2nd+
--    overlay and silently drops those votes/wagers. Scope the dedup per round so
--    replay dedup still holds within a poll/prediction while the same message id is
--    allowed across overlays. Mirrors 070's per-overlay rescope of (source, external_id).
--
-- 2. Case-fold the channel lookup. The native-mirror producer stores channel_id as
--    the lowercase Twitch login, but overlay_chat_sources.channel_id keeps the
--    streamer's original casing, so a case-sensitive '=' can miss and the round
--    mirrors to zero overlays. A functional index supports LOWER(channel_id) lookups
--    (see OverlaysForChannel).
--
-- 3. announce_on_start: opt-in "post the round question + numbered options + the
--    participate link to chat" toggle. Default FALSE — it needs the Twitch send
--    scope (user:write:chat), so it stays off unless the streamer enables it.

DROP INDEX IF EXISTS uniq_poll_vote_msg;
CREATE UNIQUE INDEX IF NOT EXISTS uniq_poll_vote_msg
    ON poll_votes(poll_id, source_message_id) WHERE source_message_id IS NOT NULL;

DROP INDEX IF EXISTS uniq_pred_entry_msg;
CREATE UNIQUE INDEX IF NOT EXISTS uniq_pred_entry_msg
    ON prediction_entries(prediction_id, source_message_id) WHERE source_message_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_overlay_chat_sources_platform_lower_channel
    ON overlay_chat_sources(platform, LOWER(channel_id));

ALTER TABLE points_earn_config ADD COLUMN IF NOT EXISTS announce_on_start BOOLEAN NOT NULL DEFAULT FALSE;
