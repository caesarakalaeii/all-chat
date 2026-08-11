-- Migration: 082_moderation_actions_table_comment
-- Description: Corrects the moderation_actions TABLE comment (ADR-0048). Follow-up to 081.
--
-- 081 corrected the per-column documentation, but the sentence that is actually wrong in the
-- database lives in migration 060's table comment: "actor_user_id = identity whose token acted".
-- Under delegation those are two different people — the moderator acts, and `credential_user_id`
-- records whose token performed the platform call. Anyone auditing "whose credential did this?"
-- off the table comment would read the wrong column and conclude a streamer did something a
-- volunteer did.
--
-- Corrected forward rather than by editing 060 or 081: the runner replays every migration on
-- every pod start, so a shipped file is history. Idempotent — COMMENT ON replaces.

BEGIN;

COMMENT ON TABLE moderation_actions IS
    'Audit log of every chat-moderation command issued via moderation-service (ADR-0017, extended by ADR-0048). Five identities are kept distinct: actor_user_id = the human who acted (owner or delegated moderator); actor_role = which; on_behalf_of_user_id = the overlay owner it was done for; credential_user_id = whose OAuth token actually performed the platform call, which is the machine-checkable proof a delegated action never fell back to the owner''s credential; platform_actor_id = the id sent as the platform''s moderator field. impersonated_by = the real admin when done under impersonation, else NULL.';

COMMIT;
