-- Migration to add multi-source chat support
-- This allows overlays to have multiple chat sources (Twitch, YouTube, Kick, TikTok)

-- Create overlay_chat_sources table
CREATE TABLE overlay_chat_sources (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    overlay_id UUID NOT NULL REFERENCES overlays(id) ON DELETE CASCADE,

    -- Platform identification
    platform VARCHAR(20) NOT NULL, -- 'twitch', 'youtube', 'kick', 'tiktok'
    channel_id VARCHAR(255) NOT NULL, -- Platform-specific channel/stream identifier
    channel_name VARCHAR(255), -- Human-readable name (e.g., Twitch username)

    -- Source status
    is_active BOOLEAN DEFAULT true,

    -- Platform-specific configuration (JSON for flexibility)
    platform_config JSONB DEFAULT '{}'::jsonb,
    -- Examples:
    -- Twitch: {"irc_enabled": true, "event_sub": false}
    -- YouTube: {"video_id": "xyz", "live_chat_id": "abc"}
    -- Kick: {"websocket_url": "wss://..."}

    -- Connection metadata
    last_connected_at TIMESTAMP,
    last_message_at TIMESTAMP,
    connection_status VARCHAR(50) DEFAULT 'disconnected', -- 'connected', 'disconnected', 'error', 'rate_limited'

    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),

    -- Ensure unique platform + channel per overlay
    UNIQUE(overlay_id, platform, channel_id)
);

CREATE INDEX idx_overlay_chat_sources_overlay_id ON overlay_chat_sources(overlay_id);
CREATE INDEX idx_overlay_chat_sources_platform ON overlay_chat_sources(platform);
CREATE INDEX idx_overlay_chat_sources_active ON overlay_chat_sources(is_active) WHERE is_active = true;
CREATE INDEX idx_overlay_chat_sources_status ON overlay_chat_sources(connection_status);

-- Add trigger for updated_at
CREATE TRIGGER update_overlay_chat_sources_updated_at BEFORE UPDATE ON overlay_chat_sources
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Remove twitch_channel from overlay_configs since sources are now in their own table
-- Note: This is a breaking change - data migration required if existing configs exist
ALTER TABLE overlay_configs DROP COLUMN IF EXISTS twitch_channel;

-- Update active_channels table to be more generic (supports all platforms)
ALTER TABLE active_channels RENAME TO active_platform_channels;

ALTER TABLE active_platform_channels ADD COLUMN IF NOT EXISTS platform VARCHAR(20) DEFAULT 'twitch';
ALTER TABLE active_platform_channels DROP CONSTRAINT IF EXISTS active_channels_pkey;
ALTER TABLE active_platform_channels ADD PRIMARY KEY (platform, channel_name);

CREATE INDEX IF NOT EXISTS idx_active_platform_channels_platform ON active_platform_channels(platform);
CREATE INDEX IF NOT EXISTS idx_active_platform_channels_listener ON active_platform_channels(listener_instance);

-- Add platform support table for future extensibility
CREATE TABLE supported_platforms (
    platform VARCHAR(20) PRIMARY KEY,
    display_name VARCHAR(100) NOT NULL,
    is_enabled BOOLEAN DEFAULT true,
    requires_oauth BOOLEAN DEFAULT false,
    config_schema JSONB, -- JSON schema for platform_config validation
    created_at TIMESTAMP DEFAULT NOW()
);

-- Insert initial platform definitions
INSERT INTO supported_platforms (platform, display_name, is_enabled, requires_oauth, config_schema) VALUES
('twitch', 'Twitch', true, false, '{"type": "object", "properties": {"irc_enabled": {"type": "boolean"}}}'::jsonb),
('youtube', 'YouTube', true, true, '{"type": "object", "properties": {"video_id": {"type": "string"}, "live_chat_id": {"type": "string"}}}'::jsonb),
('kick', 'Kick', false, false, '{"type": "object", "properties": {"websocket_url": {"type": "string"}}}'::jsonb),
('tiktok', 'TikTok', false, true, '{"type": "object", "properties": {"room_id": {"type": "string"}}}'::jsonb);

-- Add comment documentation
COMMENT ON TABLE overlay_chat_sources IS 'Stores multiple chat sources per overlay for multi-platform support';
COMMENT ON COLUMN overlay_chat_sources.platform IS 'Platform identifier: twitch, youtube, kick, tiktok';
COMMENT ON COLUMN overlay_chat_sources.channel_id IS 'Platform-specific channel/stream identifier (e.g., Twitch channel name, YouTube video ID)';
COMMENT ON COLUMN overlay_chat_sources.platform_config IS 'Platform-specific configuration in JSON format';
COMMENT ON TABLE supported_platforms IS 'Registry of supported streaming platforms and their configuration schemas';
