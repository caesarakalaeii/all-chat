-- Migration: 080_delegated_moderators
-- Description: Foundation for delegated overlay moderators (ADR-0048). Lets a streamer grant
--   other All-Chat users the moderation write-path on their overlay, where the moderator acts
--   with THEIR OWN platform credential rather than the owner's.
--
-- Idempotent throughout: the migration runner re-applies every migration on each pod start, so
-- a non-idempotent statement would crash-loop fresh pods.
--
-- Deliberately NOT here: any INSERT of a grant row. A grant is user intent, and a migration
-- that seeded one would resurrect a revoked grant on every pod restart.

BEGIN;

-- ---------------------------------------------------------------------------------------------
-- The grant itself.
-- ---------------------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS overlay_moderators (
    id                        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    overlay_id                UUID        NOT NULL REFERENCES overlays(id) ON DELETE CASCADE,
    -- NULL until the invitee accepts: an invite is created before we know who redeems it.
    moderator_user_id         UUID                 REFERENCES users(id)    ON DELETE CASCADE,
    -- No FK, mirroring moderation_actions (migration 060): the audit trail of who delegated
    -- must survive that account being deleted.
    granted_by                UUID        NOT NULL,
    status                    VARCHAR(16) NOT NULL DEFAULT 'pending'
                              CHECK (status IN ('pending', 'active', 'suspended', 'revoked')),
    -- Per-action grant. Ban/unban are opt-in; delete+timeout are the sane default.
    actions                   TEXT[]      NOT NULL DEFAULT '{delete,timeout}',
    -- SHA-256 of a single-use invite secret. The secret itself is shown once and never stored.
    invite_token_hash         BYTEA,
    invite_expires_at         TIMESTAMP,
    -- Display only: what the streamer typed or picked, so the list is readable before acceptance.
    invitee_label             VARCHAR(120),
    -- Optional pre-binding from the "pick from your Twitch mods" flow, so acceptance can say
    -- "this invite is for @sarah, but you are signed in as @bob".
    expected_platform         VARCHAR(20),
    expected_platform_user_id VARCHAR(100),
    -- Denormalised at accept time so the owner's activity view can name the moderator without
    -- joining users, and still name them after the account is gone.
    moderator_display_name    VARCHAR(120),
    created_at                TIMESTAMP   NOT NULL DEFAULT NOW(),
    accepted_at               TIMESTAMP,
    revoked_at                TIMESTAMP,
    revoked_by                UUID,
    suspended_at              TIMESTAMP,
    -- Drives the 90-day dormancy suspension: grants outlive relationships otherwise.
    last_action_at            TIMESTAMP
);

-- One live grant per (overlay, moderator). Revoked rows are kept for history, so the index is
-- partial rather than a plain UNIQUE.
CREATE UNIQUE INDEX IF NOT EXISTS uq_overlay_moderators_live
    ON overlay_moderators (overlay_id, moderator_user_id)
    WHERE moderator_user_id IS NOT NULL AND revoked_at IS NULL;

-- Serves "which overlays do I moderate", which an accepted moderator has no other way to find:
-- GET /api/v1/overlays is owner-filtered and there is no shared-with-me listing.
CREATE INDEX IF NOT EXISTS idx_overlay_moderators_by_mod
    ON overlay_moderators (moderator_user_id)
    WHERE status = 'active';

CREATE INDEX IF NOT EXISTS idx_overlay_moderators_by_overlay
    ON overlay_moderators (overlay_id, status);

CREATE UNIQUE INDEX IF NOT EXISTS uq_overlay_moderators_invite
    ON overlay_moderators (invite_token_hash)
    WHERE invite_token_hash IS NOT NULL;

COMMENT ON TABLE overlay_moderators IS
    'Delegated moderation grants (ADR-0048). Lets moderator_user_id use the moderation write-path on overlay_id with THEIR OWN platform credentials — never the owner''s. Premium is keyed on the overlay OWNER, never the moderator. Read live on every action; never cached, so revocation takes effect within one request.';
COMMENT ON COLUMN overlay_moderators.actions IS
    'Subset of delete/timeout/ban/unban this moderator may perform. Enforced server-side in authorize(), not just hidden in the UI.';
COMMENT ON COLUMN overlay_moderators.last_action_at IS
    'Last successful action. Drives dormancy suspension (90 days idle) rather than hard expiry, which would cut off a working moderator mid-stream.';

-- ---------------------------------------------------------------------------------------------
-- Per-platform enablement for a grant, plus DISPLAY-ONLY readiness.
-- ---------------------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS overlay_moderator_platforms (
    grant_id       UUID        NOT NULL REFERENCES overlay_moderators(id) ON DELETE CASCADE,
    platform       VARCHAR(20) NOT NULL,
    -- Absent row == disabled. Fail closed: a platform is never implicitly delegated, which is
    -- what keeps Discord off until the owner explicitly opts in.
    enabled        BOOLEAN     NOT NULL DEFAULT FALSE,
    verification   VARCHAR(24) NOT NULL DEFAULT 'unverified'
                   CHECK (verification IN ('unverified', 'verified', 'not_a_moderator',
                                           'needs_consent', 'needs_discord_link', 'unavailable')),
    verified_at    TIMESTAMP,
    last_denied_at TIMESTAMP,
    PRIMARY KEY (grant_id, platform)
);

COMMENT ON TABLE overlay_moderator_platforms IS
    'Per-platform legs of a delegation grant (ADR-0048). enabled is authorization; verification is TELEMETRY ONLY and must never be read as an ALLOW or a DENY — the platform''s own answer at action time is the authority. Caching a denial here would make All-Chat the stale authority the design exists to avoid, and one transient 403 would lock out a legitimate moderator.';
COMMENT ON COLUMN overlay_moderator_platforms.verification IS
    'Last known platform moderator status, for the owner-facing panel and remediation copy. Never consulted by authorize().';

-- ---------------------------------------------------------------------------------------------
-- The delegated moderator's OWN platform credential.
-- ---------------------------------------------------------------------------------------------
-- A dedicated table, keyed on the MODERATOR's identity, is not fastidiousness. The existing
-- per-channel credential tables are selected from by channel with NO user scoping:
--   twitch-eventsub-listener/channels/manager.go  LOWER(twitch_login) = LOWER(channel_id)
--   kick-listener/channels/manager.go             kick_oauth_tokens WHERE channel_id = $1
-- A moderator-scoped credential written into either becomes a candidate INGEST credential and
-- can sort first, silently breaking chat on a real channel. Hence: never store a delegated
-- credential in a row whose key is, or contains, a channel identifier.
CREATE TABLE IF NOT EXISTS mod_oauth_credentials (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id            UUID         NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    platform           VARCHAR(20)  NOT NULL,
    -- The id the platform re-checks: Twitch numeric user id, Kick numeric user id, YouTube UC…
    -- channel id. This is what we send as moderator_id.
    platform_user_id   VARCHAR(100) NOT NULL,
    platform_login     VARCHAR(200),
    access_token       TEXT         NOT NULL,
    refresh_token      TEXT,
    token_type         VARCHAR(20)  NOT NULL DEFAULT 'bearer',
    token_expires_at   TIMESTAMP,
    -- Kick rotates refresh tokens on a sliding window; Twitch/Google do not expire theirs.
    refresh_expires_at TIMESTAMP,
    granted_scopes     TEXT[]       NOT NULL DEFAULT '{}',
    encryption_version INT          NOT NULL DEFAULT 0,
    created_at         TIMESTAMP    NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMP    NOT NULL DEFAULT NOW(),
    -- Exactly one credential per (moderator, platform) — one refresh owner, so nothing races
    -- token-refresh-service. Re-consenting with a different account replaces the row, and the
    -- capabilities payload echoes which account is acting so it is never a mystery.
    UNIQUE (user_id, platform)
);

CREATE INDEX IF NOT EXISTS idx_mod_oauth_credentials_refresh
    ON mod_oauth_credentials (token_expires_at)
    WHERE refresh_token IS NOT NULL;

DROP TRIGGER IF EXISTS update_mod_oauth_credentials_updated_at ON mod_oauth_credentials;
CREATE TRIGGER update_mod_oauth_credentials_updated_at
    BEFORE UPDATE ON mod_oauth_credentials
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE mod_oauth_credentials IS
    'A delegated moderator''s OWN platform credential (ADR-0048), used to act on channels they moderate. Keyed on the moderator, NEVER on a channel: the listeners select credentials by channel with no user scoping, so a row keyed to someone else''s channel would become a candidate ingest credential. Scopes are minimised to the delegated actions and must never include chat-read or chat-send.';

-- ---------------------------------------------------------------------------------------------
-- Audit attribution. Five identities must stay distinguishable forever.
-- ---------------------------------------------------------------------------------------------
-- No backfill: actor_role IS NULL unambiguously means "legacy row, the actor was the owner".
-- An UPDATE here would also be re-run on every pod start.
ALTER TABLE moderation_actions ADD COLUMN IF NOT EXISTS actor_role           VARCHAR(24);
ALTER TABLE moderation_actions ADD COLUMN IF NOT EXISTS on_behalf_of_user_id UUID;
ALTER TABLE moderation_actions ADD COLUMN IF NOT EXISTS credential_user_id   UUID;
ALTER TABLE moderation_actions ADD COLUMN IF NOT EXISTS platform_actor_id    VARCHAR(100);
-- No FK: the action history must outlive the grant it was performed under.
ALTER TABLE moderation_actions ADD COLUMN IF NOT EXISTS grant_id             UUID;

CREATE INDEX IF NOT EXISTS idx_moderation_actions_on_behalf
    ON moderation_actions (on_behalf_of_user_id, created_at DESC)
    WHERE on_behalf_of_user_id IS NOT NULL;

COMMENT ON COLUMN moderation_actions.actor_role IS
    'owner | moderator | admin_impersonation. NULL = legacy row (pre-ADR-0048), where the actor was always the owner.';
COMMENT ON COLUMN moderation_actions.on_behalf_of_user_id IS
    'The overlay owner the action was performed for. Equals actor_user_id when the owner acted themselves.';
COMMENT ON COLUMN moderation_actions.credential_user_id IS
    'Whose OAuth token actually performed the platform call — the machine-checkable proof that a delegated action never fell back to the owner''s credential. NULL for Discord, where the shared bot acts.';
COMMENT ON COLUMN moderation_actions.platform_actor_id IS
    'The platform id sent as moderator_id, so a row can be reconciled against the platform''s own moderator log.';

-- ---------------------------------------------------------------------------------------------
-- Rollout control (ADR-0008). Seeded premium-only, and separate from the base `moderation`
-- gate so delegation can be rolled back without disabling owner moderation.
-- ---------------------------------------------------------------------------------------------
INSERT INTO feature_gates (feature_key, is_premium, description)
VALUES ('delegated_moderation', TRUE,
        'Delegated overlay moderators (ADR-0048) — keyed on the overlay OWNER''s entitlement, so a premium streamer''s moderators moderate for free')
ON CONFLICT (feature_key) DO NOTHING;

COMMIT;
