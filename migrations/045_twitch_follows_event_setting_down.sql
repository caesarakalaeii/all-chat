-- Migration: 045_twitch_follows_event_setting (down)
-- Description: Remove enable_twitch_follows column from overlay_event_settings

ALTER TABLE overlay_event_settings
    DROP COLUMN IF EXISTS enable_twitch_follows;
