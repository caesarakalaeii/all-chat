-- Rollback: 085_moderation_platform_actor_id_comment
--
-- Restores 080's column wording and 082's table wording. Correct again once the Kick delegation
-- leg is rolled back, since Twitch is then the only platform writing platform_actor_id.

BEGIN;

COMMENT ON COLUMN moderation_actions.platform_actor_id IS
    'The platform id sent as moderator_id, so a row can be reconciled against the platform''s own moderator log.';

COMMENT ON TABLE moderation_actions IS
    'Audit log of every chat-moderation command issued via moderation-service (ADR-0017, extended by ADR-0048). Five identities are kept distinct: actor_user_id = the human who acted (owner or delegated moderator); actor_role = which; on_behalf_of_user_id = the overlay owner it was done for; credential_user_id = whose OAuth token actually performed the platform call, which is the machine-checkable proof a delegated action never fell back to the owner''s credential; platform_actor_id = the id sent as the platform''s moderator field. impersonated_by = the real admin when done under impersonation, else NULL.';

COMMIT;
