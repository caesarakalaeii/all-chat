-- Migration: 081_moderation_actor_comments
-- Description: Corrects moderation_actions.actor_user_id's documented meaning (ADR-0048).
--
-- Migration 060 defined that column when moderation was owner-only, and its comment asserts the
-- actor IS the overlay owner. Role-based authorization made that false: a delegated moderator is
-- the actor, and the owner they acted for lives in on_behalf_of_user_id (migration 080). A comment
-- that contradicts the data is worse than no comment — anyone auditing "who moderated this
-- channel?" would read the wrong column and conclude a streamer did something a volunteer did.
--
-- Comments only. Migration 060 itself is left untouched: the runner replays every migration on
-- every pod start, so a shipped file is history and gets corrected forward, not edited.
--
-- Idempotent by construction: COMMENT ON replaces whatever was there.

BEGIN;

COMMENT ON COLUMN moderation_actions.actor_user_id IS
    'The human who performed the action: the overlay owner when they moderate their own overlay, the delegated moderator when one acts (ADR-0048). NOT necessarily the overlay owner (see on_behalf_of_user_id) and NOT necessarily whose credential ran the platform call (see credential_user_id).';

COMMENT ON COLUMN moderation_actions.impersonated_by IS
    'The real admin when the action was performed under impersonation, else NULL. Deliberately NOT overloaded to express delegation: an admin impersonating a streamer and a moderator acting for one are different events and must stay distinguishable.';

COMMIT;
