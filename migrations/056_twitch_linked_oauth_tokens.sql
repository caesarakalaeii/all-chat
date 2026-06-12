-- All-Chat Migration 056: Per-link Twitch OAuth credentials
-- Migration: 056
--
-- Problem (ADR-0016): the IRC↔EventSub partition can only see Twitch chat
-- grants stored on the users row, and that row belongs to the login provider.
-- A streamer who signed up via YouTube/Kick and completes the Twitch
-- add-source consent had NOWHERE to persist the grant: the predicate
-- (LOWER(users.username) = channel AND auth_provider = 'twitch') can never
-- match their account, so their channel was stuck on the IRC listener forever
-- and the "re-add your Twitch source" migration instruction silently did
-- nothing for them.
--
-- This table stores the Twitch credentials obtained when ANY account links
-- Twitch via the add-source flow, keyed by the Twitch login (= channel name).
-- The partition predicate (overlay-manager chat_via_eventsub, eventsub
-- listener SyncChannels) now also matches here, and token-refresh-service
-- keeps the rows fresh (token_type 'twitch_link').
--
-- Tokens are encrypted at rest with the shared multi-key cipher, like
-- youtube_oauth_tokens / kick_oauth_tokens (encryption_version=1).
--
-- IDEMPOTENCY: the runner re-executes every migration on each pod start, so
-- everything here is IF NOT EXISTS / CREATE OR REPLACE. Guarded by
-- services/auth-service/repository/migrations_rerun_test.go.

BEGIN;

CREATE TABLE IF NOT EXISTS twitch_oauth_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    twitch_user_id VARCHAR(50) NOT NULL,
    twitch_login VARCHAR(100) NOT NULL,
    access_token TEXT NOT NULL,
    refresh_token TEXT NOT NULL,
    token_expires_at TIMESTAMP NOT NULL,
    granted_scopes TEXT[] NOT NULL DEFAULT '{}',
    encryption_version INT NOT NULL DEFAULT 1,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, twitch_login)
);

-- The partition predicate and the EventSub listener look channels up by login,
-- case-insensitively, on every sync (30s cadence).
CREATE INDEX IF NOT EXISTS idx_twitch_oauth_tokens_login
    ON twitch_oauth_tokens (LOWER(twitch_login));

-- token-refresh-service scans by expiry.
CREATE INDEX IF NOT EXISTS idx_twitch_oauth_tokens_expiry
    ON twitch_oauth_tokens (token_expires_at);

-- CNPG runs migrations as superuser; the app connects as allchat_user.
-- Guarded so local dev / test containers without the role still work (see 035).
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'allchat_user') THEN
        GRANT ALL ON twitch_oauth_tokens TO allchat_user;
    END IF;
END $$;

-- Extend the DSGVO cleanup (047) to the new table: credentials expired 7+ days
-- (i.e. token-refresh gave up on them) serve no purpose. Falling out of this
-- table just returns the channel to the IRC listener.
CREATE OR REPLACE FUNCTION cleanup_expired_oauth_tokens() RETURNS void AS $$
BEGIN
    -- YouTube tokens
    DELETE FROM youtube_oauth_tokens
    WHERE expiry IS NOT NULL AND expiry < NOW() - INTERVAL '7 days';

    -- TikTok tokens
    DELETE FROM tiktok_oauth_tokens
    WHERE expiry IS NOT NULL AND expiry < NOW() - INTERVAL '7 days';

    -- Kick tokens
    DELETE FROM kick_oauth_tokens
    WHERE expiry IS NOT NULL AND expiry < NOW() - INTERVAL '7 days';

    -- Linked Twitch credentials
    DELETE FROM twitch_oauth_tokens
    WHERE token_expires_at < NOW() - INTERVAL '7 days';

    -- Viewer sessions with expired tokens (not refreshed for 7+ days)
    DELETE FROM viewer_sessions
    WHERE token_expires_at < NOW() - INTERVAL '7 days';
END;
$$ LANGUAGE plpgsql;

COMMENT ON TABLE twitch_oauth_tokens IS
    'Twitch OAuth credentials obtained via the add-source link flow, keyed by Twitch login. Lets non-Twitch-login accounts (YouTube/Kick signups) grant the EventSub chat scopes for their channel. Read by overlay-manager (chat_via_eventsub) and twitch-eventsub-listener (SyncChannels); refreshed by token-refresh-service.';

COMMIT;
