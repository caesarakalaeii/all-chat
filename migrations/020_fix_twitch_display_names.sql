-- Migration: Fix Twitch display names stored in overlay_chat_sources.channel_id
-- Date: 2026-01-27
-- Issue: Some overlay_chat_sources records have display_name instead of username
--        This causes API failures (invalid characters) and JOIN failures
--
-- Example: channel_id = "شوشو" (display name) should be "shahin200x" (username)
--
-- Twitch usernames are lowercase alphanumeric + underscore only
-- Display names can have Unicode, mixed case, special characters

-- Update channel_id to use username instead of display_name.
--
-- Collision-safe: skip a row when its corrected username already exists as a
-- twitch source on the same overlay. Without this guard the UPDATE violates the
-- unique constraint overlay_chat_sources_overlay_id_platform_channel_id_key when
-- an overlay has the SAME channel added both correctly (channel_id = username)
-- and via its display name (channel_id = LOWER(display_name)). The migration
-- runner re-runs every migration on each pod start, so a non-idempotent failure
-- here crash-loops the init container and blocks deploys/restarts for every
-- service. Skipping leaves the redundant display-name row in place (a harmless
-- dead source) rather than failing; cleaning up those duplicates is a separate
-- data task.
UPDATE overlay_chat_sources ocs
SET channel_id = u.username,
    updated_at = NOW()
FROM users u
WHERE ocs.platform = 'twitch'
  AND u.auth_provider = 'twitch'
  AND LOWER(u.display_name) = LOWER(ocs.channel_id)
  AND ocs.channel_id != u.username
  AND NOT EXISTS (
      SELECT 1 FROM overlay_chat_sources dup
      WHERE dup.overlay_id = ocs.overlay_id
        AND dup.platform = 'twitch'
        AND dup.channel_id = u.username
        AND dup.id != ocs.id
  );

-- Log the changes (PostgreSQL doesn't return affected rows in migrations,
-- but this comment shows what we expect):
-- Expected: 2 rows updated (user "شوشو" → "shahin200x")
