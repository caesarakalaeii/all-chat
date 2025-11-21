-- Add admin role support
-- Migration: 009
-- Description: Add is_admin column to users table for role-based access control

-- Add is_admin column to users table
ALTER TABLE users ADD COLUMN IF NOT EXISTS is_admin BOOLEAN NOT NULL DEFAULT FALSE;

-- Create index for faster admin checks
CREATE INDEX IF NOT EXISTS idx_users_is_admin ON users(is_admin) WHERE is_admin = TRUE;

-- Add comment
COMMENT ON COLUMN users.is_admin IS 'Whether the user has admin privileges';

-- Note: To make a user an admin, run:
-- UPDATE users SET is_admin = TRUE WHERE id = '<user_id>';
-- or
-- UPDATE users SET is_admin = TRUE WHERE username = '<username>';
