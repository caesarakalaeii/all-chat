-- Rollback: 081_moderation_actor_comments
--
-- Restores migration 060's original wording. That wording is factually wrong under role-based
-- authorization, so this rollback is only meaningful alongside a rollback of ADR-0048 itself.

BEGIN;

COMMENT ON COLUMN moderation_actions.actor_user_id IS
    'The overlay owner whose token performed the action.';

COMMENT ON COLUMN moderation_actions.impersonated_by IS
    'The real admin when the action was performed under impersonation, else NULL.';

COMMIT;
