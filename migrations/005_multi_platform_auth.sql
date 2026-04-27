-- All-Chat Multi-Platform Authentication Support
-- Migration: 005
-- Description: Update users table to support both Twitch and YouTube (Google) login

-- Make twitch_id nullable to support YouTube-only users
ALTER TABLE users ALTER COLUMN twitch_id DROP NOT NULL;

-- Add Google/YouTube ID column
ALTER TABLE users ADD COLUMN IF NOT EXISTS google_id VARCHAR(50) UNIQUE;

-- Add provider column to track which OAuth provider was used
ALTER TABLE users ADD COLUMN IF NOT EXISTS auth_provider VARCHAR(20) DEFAULT 'twitch';

-- Define the auth_provider CHECK constraint here, where the column is created.
-- Includes 'kick' (added in 005_kick_support.sql) and 'tiktok' (planned). The
-- DROP IF EXISTS makes this safe on re-run and lets future migrations widen
-- the allowed set without an ordering bug — see git history of 005_kick_support
-- for the original (broken) split. Wrapped in a DO block so the DROP+ADD pair
-- is atomic and idempotent.
DO $$
BEGIN
    ALTER TABLE users DROP CONSTRAINT IF EXISTS users_auth_provider_check;
    ALTER TABLE users ADD CONSTRAINT users_auth_provider_check
        CHECK (auth_provider IN ('twitch', 'youtube', 'tiktok', 'kick'));
END $$;

-- Add constraint: at least one OAuth provider ID must be present.
-- Drop + recreate so we can widen the predicate as new providers are added
-- (kick was added in 005_kick_support.sql; tiktok will follow). Without
-- this widening, kick-only users (twitch_id NULL, google_id NULL,
-- kick_id NOT NULL) violate the original predicate and ON_ERROR_STOP=1
-- aborts the migration. The pattern keeps this single source of truth
-- here, where the column lives.
DO $$
BEGIN
    ALTER TABLE users DROP CONSTRAINT IF EXISTS users_at_least_one_oauth_id;
    ALTER TABLE users ADD CONSTRAINT users_at_least_one_oauth_id
      CHECK (twitch_id IS NOT NULL OR google_id IS NOT NULL OR kick_id IS NOT NULL);
END $$;

-- Create index on google_id
CREATE INDEX IF NOT EXISTS idx_users_google_id ON users(google_id);
CREATE INDEX IF NOT EXISTS idx_users_auth_provider ON users(auth_provider);

-- Update username constraint to allow longer provider usernames
ALTER TABLE users ALTER COLUMN username TYPE VARCHAR(100);

-- Comment on changes
COMMENT ON COLUMN users.google_id IS 'Google account ID for YouTube OAuth users';
COMMENT ON COLUMN users.auth_provider IS 'Primary OAuth provider: twitch or youtube';
COMMENT ON CONSTRAINT users_at_least_one_oauth_id ON users IS 'Ensures user has at least Twitch ID or Google ID';
