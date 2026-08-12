-- Migration: 084_tiktok_treasure_chest_event_setting (down)
-- Description: Remove enable_tiktok_treasure_chests column from overlay_event_settings

ALTER TABLE overlay_event_settings
    DROP COLUMN IF EXISTS enable_tiktok_treasure_chests;
