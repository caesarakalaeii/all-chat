-- All-Chat Viewer Authentication Support
-- Migration: 011
-- Description: Add viewer authentication for sending messages to streamer chats

-- Viewer sessions table - stores OAuth tokens for viewers who want to send messages
CREATE TABLE IF NOT EXISTS viewer_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    platform VARCHAR(50) NOT NULL,              -- 'twitch', 'youtube', 'kick', 'tiktok'
    platform_user_id VARCHAR(100) NOT NULL,     -- Platform-specific user ID
    username VARCHAR(100) NOT NULL,             -- Platform username
    display_name VARCHAR(200) NOT NULL,         -- Display name on platform
    avatar_url TEXT,                            -- Profile picture URL
    access_token TEXT NOT NULL,                 -- Encrypted OAuth access token
    refresh_token TEXT,                         -- Encrypted OAuth refresh token
    token_expires_at TIMESTAMP NOT NULL,        -- Token expiration time

    -- Rate limiting fields
    last_message_at TIMESTAMP,                  -- Last message sent timestamp
    message_count_1min INTEGER DEFAULT 0,       -- Messages sent in last 1 minute
    message_count_1hour INTEGER DEFAULT 0,      -- Messages sent in last 1 hour
    rate_limit_reset_1min TIMESTAMP,            -- When 1-minute counter resets
    rate_limit_reset_1hour TIMESTAMP,           -- When 1-hour counter resets

    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),

    UNIQUE(platform, platform_user_id)          -- One session per platform user
);

CREATE INDEX idx_viewer_sessions_platform ON viewer_sessions(platform);
CREATE INDEX idx_viewer_sessions_platform_user_id ON viewer_sessions(platform, platform_user_id);
CREATE INDEX idx_viewer_sessions_token_expires_at ON viewer_sessions(token_expires_at);

-- Message history table - audit log of messages sent through All-Chat
CREATE TABLE IF NOT EXISTS viewer_message_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    viewer_session_id UUID NOT NULL REFERENCES viewer_sessions(id) ON DELETE CASCADE,
    streamer_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    overlay_id UUID REFERENCES overlays(id) ON DELETE SET NULL,
    platform VARCHAR(50) NOT NULL,              -- Platform message was sent to
    channel_id VARCHAR(100) NOT NULL,           -- Target channel ID
    channel_name VARCHAR(100) NOT NULL,         -- Target channel name
    message_text TEXT NOT NULL,                 -- Message content
    sent_at TIMESTAMP DEFAULT NOW(),            -- When message was sent
    success BOOLEAN DEFAULT TRUE,               -- Whether send was successful
    error_message TEXT,                         -- Error details if failed

    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_viewer_message_history_viewer_session_id ON viewer_message_history(viewer_session_id);
CREATE INDEX idx_viewer_message_history_streamer_user_id ON viewer_message_history(streamer_user_id);
CREATE INDEX idx_viewer_message_history_sent_at ON viewer_message_history(sent_at DESC);
CREATE INDEX idx_viewer_message_history_platform ON viewer_message_history(platform);

-- Apply updated_at trigger to viewer_sessions
CREATE TRIGGER update_viewer_sessions_updated_at BEFORE UPDATE ON viewer_sessions
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Comments for documentation
COMMENT ON TABLE viewer_sessions IS 'OAuth sessions for viewers who send messages through All-Chat';
COMMENT ON TABLE viewer_message_history IS 'Audit log of all messages sent through All-Chat viewer interface';
COMMENT ON COLUMN viewer_sessions.message_count_1min IS 'Rate limit: max 20 messages per minute';
COMMENT ON COLUMN viewer_sessions.message_count_1hour IS 'Rate limit: max 100 messages per hour';
