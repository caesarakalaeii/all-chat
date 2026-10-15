-- Migration: 103_twitch_channel_points_name (down)
-- Description: Remove twitch_channel_points_name column from overlay_event_settings

ALTER TABLE overlay_event_settings
    DROP COLUMN IF EXISTS twitch_channel_points_name;
