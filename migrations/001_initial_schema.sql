-- All-Chat Initial Database Schema
-- Migration: 001
-- Description: Create users, overlays, overlay_configs, and overlay_chat_sources tables

-- Users table (Twitch OAuth)
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    twitch_id VARCHAR(50) UNIQUE NOT NULL,
    username VARCHAR(50) UNIQUE NOT NULL,
    display_name VARCHAR(100) NOT NULL,
    profile_image_url TEXT,
    email VARCHAR(255),
    access_token TEXT NOT NULL,           -- Encrypted OAuth token
    refresh_token TEXT NOT NULL,          -- Encrypted refresh token
    token_expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_users_twitch_id ON users(twitch_id);
CREATE INDEX idx_users_username ON users(username);

-- Overlays table
CREATE TABLE IF NOT EXISTS overlays (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_overlays_user_id ON overlays(user_id);
CREATE INDEX idx_overlays_is_active ON overlays(is_active);
CREATE INDEX idx_overlays_user_id_is_active ON overlays(user_id, is_active);

-- Overlay configurations
CREATE TABLE IF NOT EXISTS overlay_configs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    overlay_id UUID NOT NULL UNIQUE REFERENCES overlays(id) ON DELETE CASCADE,
    display_settings JSONB DEFAULT '{}'::jsonb,  -- Font, colors, animations
    filter_settings JSONB DEFAULT '{}'::jsonb,   -- Banned words, user filters
    enable_7tv BOOLEAN DEFAULT TRUE,
    enable_bttv BOOLEAN DEFAULT TRUE,
    enable_ffz BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_overlay_configs_overlay_id ON overlay_configs(overlay_id);

-- Supported platforms registry
CREATE TABLE IF NOT EXISTS supported_platforms (
    platform VARCHAR(50) PRIMARY KEY,           -- 'twitch', 'youtube', 'kick', 'tiktok'
    display_name VARCHAR(100) NOT NULL,         -- 'Twitch', 'YouTube', 'Kick', 'TikTok'
    is_enabled BOOLEAN DEFAULT FALSE,           -- Feature flag
    requires_oauth BOOLEAN DEFAULT FALSE,
    config_schema JSONB DEFAULT '{}'::jsonb,    -- JSON Schema for platform-specific config
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Insert initial platforms
INSERT INTO supported_platforms (platform, display_name, is_enabled, requires_oauth) VALUES
    ('twitch', 'Twitch', TRUE, FALSE),
    ('youtube', 'YouTube', TRUE, TRUE),
    ('kick', 'Kick', FALSE, FALSE),
    ('tiktok', 'TikTok', FALSE, TRUE)
ON CONFLICT (platform) DO NOTHING;

-- Overlay chat sources (multi-source support)
CREATE TABLE IF NOT EXISTS overlay_chat_sources (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    overlay_id UUID NOT NULL REFERENCES overlays(id) ON DELETE CASCADE,
    platform VARCHAR(50) NOT NULL REFERENCES supported_platforms(platform),
    channel_id VARCHAR(100) NOT NULL,           -- Platform-specific channel ID
    channel_name VARCHAR(100) NOT NULL,         -- Display name
    auth_required BOOLEAN DEFAULT FALSE,
    config JSONB DEFAULT '{}'::jsonb,           -- Platform-specific settings
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(overlay_id, platform, channel_id)    -- Prevent duplicate sources
);

CREATE INDEX idx_overlay_chat_sources_overlay_id ON overlay_chat_sources(overlay_id);
CREATE INDEX idx_overlay_chat_sources_platform ON overlay_chat_sources(platform);
CREATE INDEX idx_overlay_chat_sources_is_active ON overlay_chat_sources(is_active);
CREATE INDEX idx_overlay_chat_sources_overlay_platform ON overlay_chat_sources(overlay_id, platform, is_active);

-- Trigger to automatically create overlay_config when overlay is created
CREATE OR REPLACE FUNCTION create_overlay_config()
RETURNS TRIGGER AS $$
BEGIN
    INSERT INTO overlay_configs (overlay_id)
    VALUES (NEW.id);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_create_overlay_config
    AFTER INSERT ON overlays
    FOR EACH ROW
    EXECUTE FUNCTION create_overlay_config();

-- Updated_at trigger function
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Apply updated_at trigger to all tables
CREATE TRIGGER update_users_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_overlays_updated_at BEFORE UPDATE ON overlays
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_overlay_configs_updated_at BEFORE UPDATE ON overlay_configs
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_overlay_chat_sources_updated_at BEFORE UPDATE ON overlay_chat_sources
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_supported_platforms_updated_at BEFORE UPDATE ON supported_platforms
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
