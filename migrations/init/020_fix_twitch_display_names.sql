-- Migration: Fix Twitch display names stored in overlay_chat_sources.channel_id
-- Date: 2026-01-27
-- Issue: Some overlay_chat_sources records have display_name instead of username
--        This causes API failures (invalid characters) and JOIN failures
--
-- Example: channel_id = "شوشو" (display name) should be "shahin200x" (username)
--
-- Twitch usernames are lowercase alphanumeric + underscore only
-- Display names can have Unicode, mixed case, special characters

-- Update channel_id to use username instead of display_name
UPDATE overlay_chat_sources ocs
SET channel_id = u.username,
    updated_at = NOW()
FROM users u
WHERE ocs.platform = 'twitch'
  AND u.auth_provider = 'twitch'
  AND LOWER(u.display_name) = LOWER(ocs.channel_id)
  AND ocs.channel_id != u.username;

-- Log the changes (PostgreSQL doesn't return affected rows in migrations,
-- but this comment shows what we expect):
-- Expected: 2 rows updated (user "شوشو" → "shahin200x")
