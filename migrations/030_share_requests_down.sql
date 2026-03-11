-- Rollback share requests and premium features
-- Migration: 030 DOWN
-- Description: Remove share_requests table and is_premium column

-- Drop share_requests table and its indexes (indexes are dropped automatically with table)
DROP TABLE IF EXISTS share_requests;

-- Drop is_premium index
DROP INDEX IF EXISTS idx_users_is_premium;

-- Drop is_premium column from users table
ALTER TABLE users DROP COLUMN IF EXISTS is_premium;
