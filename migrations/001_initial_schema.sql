-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Users table
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    twitch_id VARCHAR(50) UNIQUE NOT NULL,
    username VARCHAR(100) NOT NULL,
    display_name VARCHAR(100),
    avatar_url TEXT,
    access_token_encrypted TEXT, -- Encrypted token
    refresh_token_encrypted TEXT, -- Encrypted token
    token_expires_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    last_login_at TIMESTAMP
);

CREATE INDEX idx_users_twitch_id ON users(twitch_id);
CREATE INDEX idx_users_username ON users(username);

-- Overlays table
CREATE TABLE overlays (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_overlays_user_id ON overlays(user_id);
CREATE INDEX idx_overlays_active ON overlays(is_active) WHERE is_active = true;

-- Overlay configurations
CREATE TABLE overlay_configs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    overlay_id UUID NOT NULL REFERENCES overlays(id) ON DELETE CASCADE,

    -- Chat source configuration
    twitch_channel VARCHAR(100) NOT NULL,

    -- Emote settings
    enable_7tv BOOLEAN DEFAULT true,
    enable_bttv BOOLEAN DEFAULT true,
    enable_ffz BOOLEAN DEFAULT false,

    -- Display settings (JSON for flexibility)
    display_settings JSONB DEFAULT '{
        "max_messages": 50,
        "message_duration": 10,
        "font_size": 16,
        "animation": "slide",
        "theme": "dark"
    }'::jsonb,

    -- Filtering settings
    filter_settings JSONB DEFAULT '{
        "blocked_users": [],
        "blocked_words": [],
        "subscriber_only": false,
        "moderator_only": false,
        "min_chat_delay": 0
    }'::jsonb,

    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),

    UNIQUE(overlay_id)
);

CREATE INDEX idx_overlay_configs_overlay_id ON overlay_configs(overlay_id);
CREATE INDEX idx_overlay_configs_channel ON overlay_configs(twitch_channel);

-- Active channels being monitored (for chat listener coordination)
CREATE TABLE active_channels (
    channel_name VARCHAR(100) PRIMARY KEY,
    overlay_count INTEGER DEFAULT 0,
    last_message_at TIMESTAMP,
    listener_instance VARCHAR(100), -- Instance ID for distributed coordination
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_active_channels_listener ON active_channels(listener_instance);

-- Emote cache (optional, can also use Redis)
CREATE TABLE emote_cache (
    id SERIAL PRIMARY KEY,
    emote_code VARCHAR(100) NOT NULL,
    channel VARCHAR(100), -- NULL for global emotes
    provider VARCHAR(20) NOT NULL, -- 'twitch', '7tv', 'bttv', 'ffz'
    emote_url TEXT NOT NULL,
    emote_data JSONB, -- Additional metadata (sizes, animated, etc)
    expires_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW(),

    UNIQUE (emote_code, channel, provider)
);

CREATE INDEX idx_emote_cache_channel ON emote_cache(channel);
CREATE INDEX idx_emote_cache_expires ON emote_cache(expires_at);
CREATE INDEX idx_emote_cache_provider ON emote_cache(provider);

-- Update timestamp triggers
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_users_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_overlays_updated_at BEFORE UPDATE ON overlays
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_overlay_configs_updated_at BEFORE UPDATE ON overlay_configs
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_active_channels_updated_at BEFORE UPDATE ON active_channels
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Optional: Overlay analytics table for future use
CREATE TABLE overlay_analytics (
    id BIGSERIAL PRIMARY KEY,
    overlay_id UUID NOT NULL REFERENCES overlays(id) ON DELETE CASCADE,
    event_type VARCHAR(50) NOT NULL, -- 'view', 'message_displayed', 'connection_established'
    event_data JSONB,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_analytics_overlay_id ON overlay_analytics(overlay_id);
CREATE INDEX idx_analytics_created_at ON overlay_analytics(created_at DESC);
CREATE INDEX idx_analytics_event_type ON overlay_analytics(event_type);
