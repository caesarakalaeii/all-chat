-- Remove TikTok OAuth support (keep TikTok platform for username-based chat)

-- Drop OAuth tokens table
DROP TABLE IF EXISTS tiktok_oauth_tokens;

-- Remove OAuth ID from users table (keep platform support)
ALTER TABLE users DROP COLUMN IF EXISTS tiktok_open_id;

-- Note: Do NOT remove TikTok from supported_platforms
-- Note: Do NOT remove TikTok entries from chat_sources/active_sources
-- TikTok still works via username (like Twitch)
