-- Migration: 098_youtube_subscribers_event_setting (down)
-- Description: Remove enable_youtube_subscribers column from overlay_event_settings

ALTER TABLE overlay_event_settings
    DROP COLUMN IF EXISTS enable_youtube_subscribers;
