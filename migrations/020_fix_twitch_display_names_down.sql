-- Rollback migration: Revert username back to display_name
-- Note: This is a lossy operation - we won't know which records were originally wrong
-- Only run this if the migration causes issues and you need to rollback

-- This rollback only works if the users table still has the same data
UPDATE overlay_chat_sources ocs
SET channel_id = u.display_name,
    updated_at = NOW()
FROM users u
WHERE ocs.platform = 'twitch'
  AND u.auth_provider = 'twitch'
  AND ocs.channel_id = u.username
  AND u.display_name != u.username;

-- Note: This will revert shahin200x → شوشو
-- Only use if migration 020 needs to be rolled back
