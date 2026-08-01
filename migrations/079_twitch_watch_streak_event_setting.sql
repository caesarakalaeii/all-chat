-- Migration: 079_twitch_watch_streak_event_setting
-- Description: Add enable_twitch_watch_streaks column to overlay_event_settings.
--   Watch streaks arrive on channel.chat.notification (ADR-0046) — the only
--   subscription that carries them, and their payload is the viewer's own chat
--   message. They fire once per returning viewer per stream, so streamers need a
--   toggle to keep them off busy overlays. Defaults to TRUE: the events were
--   previously dropped entirely, so enabling them is the fix, not a surprise.
--
-- Idempotent (ADD COLUMN IF NOT EXISTS): the migration runner re-applies every
-- migration on each pod start, so a non-idempotent statement would crash-loop
-- fresh pods.

ALTER TABLE overlay_event_settings
    ADD COLUMN IF NOT EXISTS enable_twitch_watch_streaks BOOLEAN NOT NULL DEFAULT TRUE;
