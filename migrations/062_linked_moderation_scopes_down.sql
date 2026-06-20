-- Down migration for 062: drop the per-link moderation scope columns.
BEGIN;
ALTER TABLE kick_oauth_tokens DROP COLUMN IF EXISTS kick_user_id;
ALTER TABLE kick_oauth_tokens DROP COLUMN IF EXISTS granted_scopes;
ALTER TABLE youtube_oauth_tokens DROP COLUMN IF EXISTS granted_scopes;
COMMIT;
