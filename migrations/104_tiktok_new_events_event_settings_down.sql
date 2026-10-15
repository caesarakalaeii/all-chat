-- Migration: 104_tiktok_new_events_event_settings_down
-- Rollback of 104: drop the new TikTok event toggles.

ALTER TABLE overlay_event_settings DROP COLUMN IF EXISTS enable_tiktok_subscriptions;
ALTER TABLE overlay_event_settings DROP COLUMN IF EXISTS enable_tiktok_super_fans;
ALTER TABLE overlay_event_settings DROP COLUMN IF EXISTS enable_tiktok_questions;
