-- All-Chat YouTube Support Migration
-- Migration: 003
-- Description: Add YouTube OAuth tokens, quota tracking, and supported platforms

BEGIN;

-- YouTube OAuth tokens table
-- Stores OAuth credentials per user/channel for YouTube authentication
CREATE TABLE IF NOT EXISTS youtube_oauth_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    channel_id VARCHAR(255) NOT NULL,           -- YouTube channel ID (UCxxxxxx)
    access_token TEXT NOT NULL,
    refresh_token TEXT NOT NULL,
    token_type VARCHAR(50) DEFAULT 'Bearer',
    expiry TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(user_id, channel_id)
);

CREATE INDEX idx_youtube_oauth_user_id ON youtube_oauth_tokens(user_id);
CREATE INDEX idx_youtube_oauth_channel_id ON youtube_oauth_tokens(channel_id);
CREATE INDEX idx_youtube_oauth_expiry ON youtube_oauth_tokens(expiry);

-- YouTube quota usage tracking
-- Tracks daily YouTube API quota consumption
CREATE TABLE IF NOT EXISTS youtube_quota_usage (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    date DATE NOT NULL UNIQUE,
    units_used INT NOT NULL DEFAULT 0,
    units_limit INT NOT NULL DEFAULT 10000,     -- Default YouTube API quota
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_youtube_quota_date ON youtube_quota_usage(date);

-- Supported platforms registry (optional - for future extensibility)
-- Tracks which platforms are enabled and their configuration
CREATE TABLE IF NOT EXISTS supported_platforms (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    platform VARCHAR(50) NOT NULL UNIQUE,       -- 'twitch', 'youtube', 'kick', 'tiktok'
    display_name VARCHAR(100) NOT NULL,         -- 'Twitch', 'YouTube', 'Kick', 'TikTok'
    is_enabled BOOLEAN DEFAULT TRUE,
    requires_oauth BOOLEAN DEFAULT TRUE,
    config_schema JSONB DEFAULT '{}'::jsonb,    -- Platform-specific config
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_supported_platforms_platform ON supported_platforms(platform);
CREATE INDEX idx_supported_platforms_is_enabled ON supported_platforms(is_enabled);

-- Insert default supported platforms
INSERT INTO supported_platforms (platform, display_name, is_enabled, requires_oauth, config_schema)
VALUES
    ('twitch', 'Twitch', true, true, '{
        "oauth_scopes": ["chat:read"],
        "irc_server": "irc.chat.twitch.tv:6667",
        "rate_limits": {
            "join": "20/10s",
            "messages": "100/30s"
        }
    }'::jsonb),
    ('youtube', 'YouTube', true, true, '{
        "oauth_scopes": ["https://www.googleapis.com/auth/youtube.readonly"],
        "quota_limit": 10000,
        "api_version": "v3"
    }'::jsonb),
    ('kick', 'Kick', false, false, '{
        "api_type": "websocket",
        "requires_auth": false
    }'::jsonb),
    ('tiktok', 'TikTok', false, true, '{
        "oauth_scopes": ["user.info.basic", "video.list"],
        "api_version": "v1"
    }'::jsonb)
ON CONFLICT (platform) DO UPDATE SET
    display_name = EXCLUDED.display_name,
    is_enabled = EXCLUDED.is_enabled,
    requires_oauth = EXCLUDED.requires_oauth,
    config_schema = EXCLUDED.config_schema,
    updated_at = NOW();

COMMIT;
