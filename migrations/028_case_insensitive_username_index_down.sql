-- Rollback Migration 028: Case-insensitive username index

-- Drop the case-insensitive index
DROP INDEX IF EXISTS idx_users_username_lower;

-- Restore the old case-sensitive index
CREATE INDEX idx_users_username ON users(username);
