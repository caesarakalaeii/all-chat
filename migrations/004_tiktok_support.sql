-- All-Chat TikTok Support Migration
-- Migration: 004
-- Description: Add TikTok OAuth tokens table and enable TikTok platform

BEGIN;

-- TikTok OAuth tokens table
-- Stores OAuth credentials per user for TikTok authentication
-- Note: TikTok uses open_id and union_id instead of channel IDs
CREATE TABLE IF NOT EXISTS tiktok_oauth_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    open_id VARCHAR(255) NOT NULL,              -- User's app-specific ID
    union_id VARCHAR(255),                      -- User's cross-app ID (optional)
    access_token TEXT NOT NULL,
    refresh_token TEXT NOT NULL,
    token_type VARCHAR(50) DEFAULT 'Bearer',
    scope TEXT,                                 -- Granted scopes (e.g., "user.info.basic,video.list")
    expiry TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(user_id, open_id)
);

CREATE INDEX IF NOT EXISTS idx_tiktok_oauth_user_id ON tiktok_oauth_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_tiktok_oauth_open_id ON tiktok_oauth_tokens(open_id);
CREATE INDEX IF NOT EXISTS idx_tiktok_oauth_union_id ON tiktok_oauth_tokens(union_id);
CREATE INDEX IF NOT EXISTS idx_tiktok_oauth_expiry ON tiktok_oauth_tokens(expiry);

-- Enable TikTok platform (currently in BETA - using unofficial API)
UPDATE supported_platforms
SET
    is_enabled = true,
    config_schema = '{
        "oauth_scopes": ["user.info.basic", "video.list"],
        "api_version": "v2",
        "status": "beta",
        "note": "Using unofficial TikTok-Live-Connector library for chat. Official live chat API not yet available.",
        "auth_endpoint": "https://www.tiktok.com/v2/auth/authorize/",
        "token_endpoint": "https://open.tiktokapis.com/v2/oauth/token/",
        "user_info_endpoint": "https://open.tiktokapis.com/v2/user/info/"
    }'::jsonb
WHERE platform = 'tiktok';

COMMIT;
