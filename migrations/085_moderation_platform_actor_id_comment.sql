-- Migration: 085_moderation_platform_actor_id_comment
-- Description: Widens the platform_actor_id documentation for platforms with no moderator field
--              (ADR-0048, Kick leg). Follow-up to 080/081/082.
--
-- 080 documented the column as "the platform id sent as moderator_id". That is exactly right for
-- Twitch, and wrong for Kick: Kick's moderation endpoints carry no moderator field at all — the
-- acting identity is implied by the bearer token, and `broadcaster_user_id` is the only id in the
-- request. The value recorded there for a Kick action is therefore the Kick account that acted,
-- not a field that was transmitted.
--
-- The distinction matters to anyone reconciling an All-Chat row against a platform's own moderator
-- log: read literally, the old comment says a Kick row's platform_actor_id should appear in the
-- request we sent, and it never does. What it IS good for is the same thing on both platforms —
-- identifying which platform account the platform saw acting.
--
-- Corrected forward rather than by editing 080/081: the runner replays every migration on every
-- pod start, so a shipped file is history. Idempotent — COMMENT ON replaces.

BEGIN;

COMMENT ON COLUMN moderation_actions.platform_actor_id IS
    'The platform account that performed the action, so a row can be reconciled against the platform''s own moderator log. On Twitch this is the id sent as moderator_id. On Kick nothing is sent: its endpoints carry no moderator field (the actor is implied by the token), so this is the acting account''s numeric Kick id as recorded at consent. NULL for Discord, where the shared bot acts, and for dry runs.';

COMMENT ON TABLE moderation_actions IS
    'Audit log of every chat-moderation command issued via moderation-service (ADR-0017, extended by ADR-0048). Five identities are kept distinct: actor_user_id = the human who acted (owner or delegated moderator); actor_role = which; on_behalf_of_user_id = the overlay owner it was done for; credential_user_id = whose OAuth token actually performed the platform call, which is the machine-checkable proof a delegated action never fell back to the owner''s credential; platform_actor_id = the platform account that acted (sent as moderator_id where the platform has such a field). impersonated_by = the real admin when done under impersonation, else NULL.';

COMMIT;
