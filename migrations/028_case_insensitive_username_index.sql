-- Migration 028: Add case-insensitive username index
--
-- Problem: Username lookups are case-sensitive but platform usernames
-- (Twitch, YouTube, Kick) are case-insensitive. This causes STREAMER_NOT_FOUND
-- errors when the URL casing doesn't match the stored username.
--
-- Solution: Add a functional index on LOWER(username) for efficient
-- case-insensitive lookups.

-- Drop the old case-sensitive index
DROP INDEX IF EXISTS idx_users_username;

-- Create new case-insensitive index
CREATE INDEX idx_users_username_lower ON users(LOWER(username));

-- Note: The query in GetByUsername has been updated to use:
-- WHERE LOWER(username) = LOWER($1)
