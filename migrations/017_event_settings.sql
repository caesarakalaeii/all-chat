-- Migration: 017_event_settings
-- Description: Create overlay_event_settings table for granular event display controls

-- Create overlay_event_settings table
CREATE TABLE IF NOT EXISTS overlay_event_settings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    overlay_id UUID NOT NULL REFERENCES overlays(id) ON DELETE CASCADE,

    -- Twitch Events
    enable_twitch_subs BOOLEAN DEFAULT TRUE,
    enable_twitch_resubs BOOLEAN DEFAULT TRUE,
    enable_twitch_gift_subs BOOLEAN DEFAULT TRUE,
    enable_twitch_bits BOOLEAN DEFAULT TRUE,
    enable_twitch_raids BOOLEAN DEFAULT TRUE,
    enable_twitch_channel_points BOOLEAN DEFAULT FALSE,  -- Requires EventSub

    -- YouTube Events
    enable_youtube_super_chat BOOLEAN DEFAULT TRUE,
    enable_youtube_super_sticker BOOLEAN DEFAULT TRUE,
    enable_youtube_members BOOLEAN DEFAULT TRUE,
    enable_youtube_member_milestones BOOLEAN DEFAULT TRUE,
    enable_youtube_member_gifts BOOLEAN DEFAULT TRUE,

    -- Kick Events
    enable_kick_subs BOOLEAN DEFAULT TRUE,
    enable_kick_gifts BOOLEAN DEFAULT TRUE,

    -- TikTok Events
    enable_tiktok_likes BOOLEAN DEFAULT TRUE,
    enable_tiktok_gifts BOOLEAN DEFAULT TRUE,
    enable_tiktok_follows BOOLEAN DEFAULT TRUE,
    enable_tiktok_shares BOOLEAN DEFAULT TRUE,

    -- Aggregation Settings
    tiktok_like_aggregation_window_seconds INT DEFAULT 30,

    -- Display Settings
    event_display_duration_multiplier FLOAT DEFAULT 1.0,

    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(overlay_id)
);

CREATE INDEX idx_overlay_event_settings_overlay_id ON overlay_event_settings(overlay_id);

-- Function to auto-create event settings for new overlays
CREATE OR REPLACE FUNCTION create_overlay_event_settings()
RETURNS TRIGGER AS $$
BEGIN
    INSERT INTO overlay_event_settings (overlay_id)
    VALUES (NEW.id)
    ON CONFLICT (overlay_id) DO NOTHING;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger to auto-create event settings when overlay is created
CREATE TRIGGER trigger_create_overlay_event_settings
    AFTER INSERT ON overlays
    FOR EACH ROW
    EXECUTE FUNCTION create_overlay_event_settings();

-- Backfill event settings for existing overlays
INSERT INTO overlay_event_settings (overlay_id)
SELECT id FROM overlays
ON CONFLICT (overlay_id) DO NOTHING;
