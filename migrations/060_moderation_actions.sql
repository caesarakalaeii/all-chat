-- All-Chat Migration 060: Moderation action audit log
-- Migration: 060
--
-- The moderation-service (ADR-0017) performs All-Chat's first authenticated WRITE
-- actions (delete message / timeout / ban / unban) on a streamer's behalf using the
-- broadcaster's own OAuth token. Every command — allowed, denied, dry-run, or failed
-- at the platform — is recorded here for abuse forensics and accountability.
--
-- actor_user_id is the identity whose token acts (the overlay owner). When an admin
-- performs the action while impersonating that user, impersonated_by records the real
-- admin (product decision: impersonated moderation is ALLOWED but always attributed
-- to the admin). impersonated_by is NULL for normal, non-impersonated actions.
--
-- No foreign keys to users(id): the audit trail must survive account deletion for
-- forensic purposes.
--
-- IDEMPOTENCY: every service runs the full migration set on each pod restart (the
-- runner does not track applied migrations), so this script must be safe to
-- re-execute — hence CREATE TABLE / CREATE INDEX IF NOT EXISTS.

BEGIN;

CREATE TABLE IF NOT EXISTS moderation_actions (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    overlay_id        UUID         NOT NULL,
    actor_user_id     UUID         NOT NULL,
    impersonated_by   UUID,
    platform          VARCHAR(50)  NOT NULL,
    channel_id        VARCHAR(100) NOT NULL,
    action            VARCHAR(20)  NOT NULL,
    target_user_id    VARCHAR(100),
    target_message_id VARCHAR(200),
    reason            TEXT,
    outcome           VARCHAR(30)  NOT NULL,
    platform_status   TEXT,
    created_at        TIMESTAMP    NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_moderation_actions_overlay
    ON moderation_actions (overlay_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_moderation_actions_actor
    ON moderation_actions (actor_user_id, created_at DESC);

COMMENT ON TABLE moderation_actions IS
    'Audit log of every chat-moderation command issued via moderation-service (ADR-0017). actor_user_id = identity whose token acted; impersonated_by = admin when done under impersonation, else NULL.';

COMMIT;
