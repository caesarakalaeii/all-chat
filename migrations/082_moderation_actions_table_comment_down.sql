-- Rollback: 082_moderation_actions_table_comment
--
-- Restores migration 060's wording. Correct again once ADR-0048 is rolled back, since dropping the
-- delegation columns makes the actor the overlay owner by construction.

BEGIN;

COMMENT ON TABLE moderation_actions IS
    'Audit log of every chat-moderation command issued via moderation-service (ADR-0017). actor_user_id = identity whose token acted; impersonated_by = admin when done under impersonation, else NULL.';

COMMIT;
