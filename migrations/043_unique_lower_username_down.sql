-- Rollback Migration 043: Restore original username constraints

-- Drop the case-insensitive unique index
DROP INDEX IF EXISTS idx_users_username_lower_unique;

-- Restore the column-level unique constraint
ALTER TABLE users ADD CONSTRAINT users_username_key UNIQUE (username);

-- Restore the non-unique functional index from migration 028
CREATE INDEX idx_users_username_lower ON users(LOWER(username));
