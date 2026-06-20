-- All-Chat Migration 062: Per-link moderation scopes (Kick + YouTube)
-- Migration: 062
--
-- Problem (ADR-0017, linked-account moderation): a streamer whose All-Chat login
-- is platform A but who ALSO linked platform B must be able to moderate B. The
-- moderation service resolves the broadcaster's own credential per platform, but
-- the per-link token tables lacked the columns moderation needs:
--   * kick_oauth_tokens  — no NUMERIC broadcaster id (the Kick moderation API keys
--                          on the numeric user id, not the slug) and no per-link
--                          granted_scopes (so the opt-in moderation:ban grant had
--                          nowhere to live for a non-Kick-login account).
--   * youtube_oauth_tokens — no granted_scopes (so the force-ssl opt-in grant for a
--                          linked YouTube channel could not be tracked).
-- twitch_oauth_tokens already carries twitch_user_id + granted_scopes (migration
-- 056), which is why linked Twitch moderation already worked; this brings Kick and
-- YouTube to parity.
--
-- granted_scopes mirrors twitch_oauth_tokens (TEXT[] NOT NULL DEFAULT '{}'); the
-- auth-service writes the opt-in moderation grant here on the re-consent callback,
-- and token-refresh-service / the moderation-service refresh leave it untouched.
--
-- IDEMPOTENCY: the runner re-executes every migration on each pod start, so every
-- statement here is ADD COLUMN IF NOT EXISTS. Guarded by
-- services/auth-service/repository/migrations_rerun_test.go.

BEGIN;

-- Kick: numeric broadcaster id (Kick moderation API target) + opt-in moderation scope.
ALTER TABLE kick_oauth_tokens ADD COLUMN IF NOT EXISTS kick_user_id VARCHAR(255);
ALTER TABLE kick_oauth_tokens ADD COLUMN IF NOT EXISTS granted_scopes TEXT[] NOT NULL DEFAULT '{}';

COMMENT ON COLUMN kick_oauth_tokens.kick_user_id IS
    'Numeric Kick user id of the channel owner (broadcaster == moderator for own-channel moderation). NULL for legacy listener-only rows that predate moderation support.';
COMMENT ON COLUMN kick_oauth_tokens.granted_scopes IS
    'OAuth scopes granted on this linked Kick credential. Carries the opt-in moderation:ban grant (ADR-0017) for non-Kick-login accounts; preserved across token refresh.';

-- YouTube: opt-in moderation scope (force-ssl) for a linked channel credential.
ALTER TABLE youtube_oauth_tokens ADD COLUMN IF NOT EXISTS granted_scopes TEXT[] NOT NULL DEFAULT '{}';

COMMENT ON COLUMN youtube_oauth_tokens.granted_scopes IS
    'OAuth scopes granted on this YouTube channel credential. Carries the opt-in youtube.force-ssl moderation grant (ADR-0017); preserved across token refresh.';

-- CNPG runs migrations as superuser; the app connects as allchat_user. Guarded so
-- local dev / test containers without the role still work (see migration 035/056).
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'allchat_user') THEN
        GRANT ALL ON kick_oauth_tokens TO allchat_user;
        GRANT ALL ON youtube_oauth_tokens TO allchat_user;
    END IF;
END $$;

COMMIT;
