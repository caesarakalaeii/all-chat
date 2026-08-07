-- Rollback: 080_delegated_moderators
--
-- Drops the delegation grant tables, the delegated-moderator credential store, the audit
-- attribution columns and the rollout gate. Grants and stored credentials are destroyed, so
-- moderators must be re-invited and must re-consent after a roll-forward.
--
-- The base `moderation` gate (migration 061) and moderation_actions itself are left alone:
-- owner moderation predates this migration and must keep working.

BEGIN;

DROP TABLE IF EXISTS overlay_moderator_platforms;
DROP TABLE IF EXISTS overlay_moderators;

DROP TRIGGER IF EXISTS update_mod_oauth_credentials_updated_at ON mod_oauth_credentials;
DROP TABLE IF EXISTS mod_oauth_credentials;

DROP INDEX IF EXISTS idx_moderation_actions_on_behalf;
ALTER TABLE moderation_actions DROP COLUMN IF EXISTS grant_id;
ALTER TABLE moderation_actions DROP COLUMN IF EXISTS platform_actor_id;
ALTER TABLE moderation_actions DROP COLUMN IF EXISTS credential_user_id;
ALTER TABLE moderation_actions DROP COLUMN IF EXISTS on_behalf_of_user_id;
ALTER TABLE moderation_actions DROP COLUMN IF EXISTS actor_role;

DELETE FROM feature_gates WHERE feature_key = 'delegated_moderation';

COMMIT;
