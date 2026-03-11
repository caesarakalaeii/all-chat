-- All-Chat Multi-Platform Authentication Support
-- Migration: 005
-- Description: Update users table to support both Twitch and YouTube (Google) login

-- Make twitch_id nullable to support YouTube-only users
ALTER TABLE users ALTER COLUMN twitch_id DROP NOT NULL;

-- Add Google/YouTube ID column
ALTER TABLE users ADD COLUMN IF NOT EXISTS google_id VARCHAR(50) UNIQUE;

-- Add provider column to track which OAuth provider was used
ALTER TABLE users ADD COLUMN IF NOT EXISTS auth_provider VARCHAR(20) DEFAULT 'twitch';

-- Add constraint: at least one OAuth provider ID must be present
ALTER TABLE users ADD CONSTRAINT users_at_least_one_oauth_id
  CHECK (twitch_id IS NOT NULL OR google_id IS NOT NULL);

-- Create index on google_id
CREATE INDEX IF NOT EXISTS idx_users_google_id ON users(google_id);
CREATE INDEX IF NOT EXISTS idx_users_auth_provider ON users(auth_provider);

-- Update username constraint to allow longer provider usernames
ALTER TABLE users ALTER COLUMN username TYPE VARCHAR(100);

-- Comment on changes
COMMENT ON COLUMN users.google_id IS 'Google account ID for YouTube OAuth users';
COMMENT ON COLUMN users.auth_provider IS 'Primary OAuth provider: twitch or youtube';
COMMENT ON CONSTRAINT users_at_least_one_oauth_id ON users IS 'Ensures user has at least Twitch ID or Google ID';
