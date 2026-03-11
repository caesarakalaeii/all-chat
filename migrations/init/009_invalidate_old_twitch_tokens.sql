-- Migration: Invalidate old Twitch OAuth tokens to force re-authentication with new scopes
-- Date: 2026-01-27
-- Reason: EventSub webhook support now requires additional OAuth scopes:
--   - channel:read:subscriptions (for sub/gift/resub events)
--   - bits:read (for cheer events)
--   - moderator:read:followers (for follow events)
--
-- Old tokens don't have these scopes, so we expire them to force users to re-authenticate
-- with the updated scope list from auth-service.

-- Expire all Twitch tokens that were created before this migration
-- This forces users to re-authenticate and get tokens with the new scopes
UPDATE users
SET token_expires_at = NOW()
WHERE auth_provider = 'twitch'
  AND token_expires_at > NOW();  -- Only expire tokens that are currently valid

-- Log the number of tokens expired
-- (PostgreSQL doesn't have a built-in way to return this in a migration,
--  but the application logs will show users needing to re-auth)
