-- Migration: 079_twitch_watch_streak_event_setting (down)
-- Description: Remove enable_twitch_watch_streaks column from overlay_event_settings

ALTER TABLE overlay_event_settings
    DROP COLUMN IF EXISTS enable_twitch_watch_streaks;
