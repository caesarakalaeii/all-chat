-- Migration: 104_tiktok_new_events_event_settings
-- Description: Add per-overlay toggles for the new TikTok LIVE events to
--   overlay_event_settings: enable_tiktok_subscriptions (tiktok_subscription),
--   enable_tiktok_super_fans (super_fan_join + super_fan_box),
--   enable_tiktok_questions (question). Defaults to TRUE: the events never
--   reached an overlay before, so enabling them is the fix, not a surprise.
--
-- Idempotent (ADD COLUMN IF NOT EXISTS): the migration runner re-applies every
-- migration on each pod start, so a non-idempotent statement would crash-loop
-- fresh pods.

ALTER TABLE overlay_event_settings
    ADD COLUMN IF NOT EXISTS enable_tiktok_subscriptions BOOLEAN NOT NULL DEFAULT TRUE;
ALTER TABLE overlay_event_settings
    ADD COLUMN IF NOT EXISTS enable_tiktok_super_fans BOOLEAN NOT NULL DEFAULT TRUE;
ALTER TABLE overlay_event_settings
    ADD COLUMN IF NOT EXISTS enable_tiktok_questions BOOLEAN NOT NULL DEFAULT TRUE;
