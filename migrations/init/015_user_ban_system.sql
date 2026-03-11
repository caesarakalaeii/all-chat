-- Migration 015: User Ban System
-- Description: Add user ban columns and banned platform IDs table for comprehensive ban system

-- Add ban columns to users table (simpler than separate table)
ALTER TABLE users ADD COLUMN IF NOT EXISTS is_banned BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE users ADD COLUMN IF NOT EXISTS banned_at TIMESTAMP NULL;
ALTER TABLE users ADD COLUMN IF NOT EXISTS banned_reason TEXT NULL;
ALTER TABLE users ADD COLUMN IF NOT EXISTS banned_by UUID NULL REFERENCES users(id);

-- Index for quickly finding banned users
CREATE INDEX IF NOT EXISTS idx_users_is_banned ON users(is_banned) WHERE is_banned = TRUE;

-- Banned platform IDs table (prevents multi-account abuse)
CREATE TABLE IF NOT EXISTS banned_platform_ids (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    platform VARCHAR(50) NOT NULL,  -- 'twitch', 'youtube', 'kick'
    platform_id VARCHAR(100) NOT NULL,
    banned_by UUID NOT NULL REFERENCES users(id),
    reason TEXT NOT NULL,
    banned_at TIMESTAMP NOT NULL DEFAULT NOW(),
    unbanned_at TIMESTAMP NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Partial unique index to prevent duplicate active bans (replaces UNIQUE constraint with WHERE)
CREATE UNIQUE INDEX IF NOT EXISTS idx_banned_platform_ids_unique_active
    ON banned_platform_ids(platform, platform_id) WHERE is_active = TRUE;

-- Index for platform ID ban lookups
CREATE INDEX IF NOT EXISTS idx_banned_platform_ids_lookup
    ON banned_platform_ids(platform, platform_id) WHERE is_active = TRUE;

-- Comments
COMMENT ON COLUMN users.is_banned IS 'Whether this user account is banned from logging in';
COMMENT ON COLUMN users.banned_reason IS 'Reason for ban (admin reference)';
COMMENT ON TABLE banned_platform_ids IS 'Prevents multi-account abuse by banning platform IDs';
