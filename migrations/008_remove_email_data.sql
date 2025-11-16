-- Remove email storage and scopes no longer required
-- Migration: 008
-- Description: Drop email column from users and remove Twitch email OAuth scope

BEGIN;

ALTER TABLE users
    DROP COLUMN IF EXISTS email;

UPDATE supported_platforms
SET config_schema = jsonb_set(
        config_schema,
        '{oauth_scopes}',
        '["chat:read"]'::jsonb,
        true
    ),
    updated_at = NOW()
WHERE platform = 'twitch';

COMMIT;
