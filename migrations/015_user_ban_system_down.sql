-- Migration 015 Down: Rollback User Ban System

-- Drop banned platform IDs table
DROP TABLE IF EXISTS banned_platform_ids CASCADE;

-- Drop indexes
DROP INDEX IF EXISTS idx_users_is_banned;

-- Remove ban columns from users table
ALTER TABLE users DROP COLUMN IF EXISTS is_banned;
ALTER TABLE users DROP COLUMN IF EXISTS banned_at;
ALTER TABLE users DROP COLUMN IF EXISTS banned_reason;
ALTER TABLE users DROP COLUMN IF EXISTS banned_by;
