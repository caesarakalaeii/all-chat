-- Migration 005: Add Kick platform support
-- Description: Adds Kick.com integration support to the platform

-- Add kick_id column to users table
ALTER TABLE users ADD COLUMN IF NOT EXISTS kick_id VARCHAR(255);

-- Create index on kick_id for efficient lookups
CREATE INDEX IF NOT EXISTS idx_users_kick_id ON users(kick_id);

-- Add unique constraint to ensure one account per Kick ID
ALTER TABLE users ADD CONSTRAINT users_kick_id_unique UNIQUE (kick_id);

-- Update auth_provider check constraint to include 'kick'
-- First drop the existing constraint
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_auth_provider_check;

-- Recreate with 'kick' included
ALTER TABLE users ADD CONSTRAINT users_auth_provider_check
    CHECK (auth_provider IN ('twitch', 'youtube', 'tiktok', 'kick'));

-- Create kick_oauth_tokens table for storing Kick OAuth tokens
-- Similar to youtube_oauth_tokens, this allows the Kick listener to access tokens
CREATE TABLE IF NOT EXISTS kick_oauth_tokens (
    id SERIAL PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    channel_id VARCHAR(255) NOT NULL, -- Kick channel/chatroom ID
    access_token TEXT NOT NULL,
    refresh_token TEXT NOT NULL,
    token_type VARCHAR(50) DEFAULT 'Bearer',
    expiry TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, channel_id)
);

-- Create index on user_id for efficient lookups
CREATE INDEX IF NOT EXISTS idx_kick_oauth_tokens_user_id ON kick_oauth_tokens(user_id);

-- Create index on channel_id for efficient lookups
CREATE INDEX IF NOT EXISTS idx_kick_oauth_tokens_channel_id ON kick_oauth_tokens(channel_id);

-- Add comments for documentation
COMMENT ON TABLE kick_oauth_tokens IS 'Stores Kick OAuth tokens for the Kick Listener service';
COMMENT ON COLUMN kick_oauth_tokens.channel_id IS 'Kick channel slug or chatroom ID';
COMMENT ON COLUMN kick_oauth_tokens.user_id IS 'Reference to the user who authorized this channel';
