-- Migration: 103_twitch_channel_points_name
-- Description: Add twitch_channel_points_name column to overlay_event_settings.
--   Streamers rename their channel points ("Kappa Coins"), but neither Helix nor
--   the redemption EventSub payload exposes that name, so the streamer enters it
--   once per overlay. The message-processor uses it in the channel-points event
--   text and the credit roll in its Top Channel Points totals. Empty means
--   "points", the label used before this column existed, so existing overlays
--   are unchanged.
--
-- Idempotent (ADD COLUMN IF NOT EXISTS): the migration runner re-applies every
-- migration on each pod start, so a non-idempotent statement would crash-loop
-- fresh pods.

ALTER TABLE overlay_event_settings
    ADD COLUMN IF NOT EXISTS twitch_channel_points_name TEXT NOT NULL DEFAULT ''
        CHECK (char_length(twitch_channel_points_name) <= 50);
