-- Migration: 045_twitch_follows_event_setting
-- Description: Add enable_twitch_follows column to overlay_event_settings.
--   The column was referenced by filter/event_filter.go but never created in
--   migration 017, causing SQL errors that made follow events bypass per-overlay
--   filtering and appear on all overlays regardless of their settings.

ALTER TABLE overlay_event_settings
    ADD COLUMN IF NOT EXISTS enable_twitch_follows BOOLEAN DEFAULT TRUE;
