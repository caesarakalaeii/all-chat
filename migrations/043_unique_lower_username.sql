-- Migration 043: Enforce case-insensitive uniqueness on username
--
-- Problem: The UNIQUE constraint on users.username is case-sensitive (from 001),
-- but GetByUsername uses LOWER(username) = LOWER($1). This allows two users to
-- register with "CrumbleSupreme" and "crumblesupreme" — both pass the UNIQUE
-- check but collide on lookup, returning non-deterministic results.
--
-- Additionally, users can create duplicate accounts by authenticating via
-- different platforms (e.g., Twitch as "CrumbleSupreme", YouTube as a separate
-- account) because getOrCreateUser only checks platform-specific IDs.
--
-- Solution: Replace the non-unique functional index from migration 028 with a
-- UNIQUE index on LOWER(username). Also drop the column-level UNIQUE constraint
-- since the new index covers it (and is stricter).

-- Drop the old non-unique functional index from migration 028
DROP INDEX IF EXISTS idx_users_username_lower;

-- Drop the column-level unique constraint from migration 001
-- (constraint name is auto-generated as users_username_key by PostgreSQL)
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_username_key;

-- Create a case-insensitive UNIQUE index
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_username_lower_unique ON users(LOWER(username));
