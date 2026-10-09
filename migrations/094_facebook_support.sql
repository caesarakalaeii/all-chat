-- Migration 094: Facebook platform support (ADR-0060)
--
-- Three things, in dependency order:
--
-- 1. supported_platforms row — the overlay_chat_sources.platform FK refuses an
--    unknown value, so 'facebook' must exist before any source can be created.
--
-- 2. facebook_oauth_tokens — the streamer's Page credential. The token chain
--    (ADR-0060): authorization code -> short-lived user token -> long-lived
--    user token (~60 days) -> Page access token. A Page token obtained from a
--    long-lived user token does NOT expire (Meta's Access Tokens doc), so
--    unlike youtube/kick/twitch there is NO refresh_token column and NO expiry:
--    the row is replaced wholesale on re-consent and dies only when the
--    streamer revokes the app or changes their password. token-refresh-service
--    deliberately has nothing to do for Facebook.
--
-- 3. feature_gates seed platform_facebook (ADR-0008): seeded is_premium=TRUE =
--    rollout cohort, flipped via the admin feature-gates endpoint without a
--    redeploy. Enforcement (overlay-manager gate cache + source-add check) is
--    wired centrally after merge.
--
-- IDEMPOTENCY: the migration runner replays every migration on each pod
-- restart, so every statement is safe to re-execute (CREATE IF NOT EXISTS,
-- ON CONFLICT DO NOTHING).

BEGIN;

INSERT INTO supported_platforms (platform, display_name, is_enabled, requires_oauth, config_schema)
VALUES ('facebook', 'Facebook', true, true,
        '{"api_type": "graph_api_polling", "graph_version": "v26.0", "requires_page_token": true}'::jsonb)
ON CONFLICT (platform) DO NOTHING;

CREATE TABLE IF NOT EXISTS facebook_oauth_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    page_id VARCHAR(255) NOT NULL,              -- Facebook Page ID (the source's channel_id)
    page_name VARCHAR(255),                     -- display only; refreshed on re-consent
    access_token TEXT NOT NULL,                 -- encrypted Page access token (non-expiring, ADR-0060)
    granted_scopes TEXT[] NOT NULL DEFAULT '{}',-- permissions granted at consent (pages_read_engagement, pages_manage_engagement)
    encryption_version INT NOT NULL DEFAULT 1,  -- AES-GCM multi-key chain (D-04); 1 = encrypted writes
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, page_id)
);

CREATE INDEX IF NOT EXISTS idx_facebook_oauth_user_id ON facebook_oauth_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_facebook_oauth_page_id ON facebook_oauth_tokens(page_id);

-- The Page token is per (user, page): one Facebook account must not be able to
-- claim another user's Page (mirrors uq_discord_identities_discord_user_id's
-- reasoning, weaker only because Facebook page ids are not globally secret).
CREATE UNIQUE INDEX IF NOT EXISTS uq_facebook_oauth_user_page
    ON facebook_oauth_tokens (user_id, page_id);

COMMENT ON TABLE facebook_oauth_tokens IS
    'Facebook Page credentials per user (ADR-0060). The page token from a '
    'long-lived user token does not expire, so there is no refresh_token or '
    'expiry: replaced on re-consent, invalidated only by revocation or a '
    'password change. Consumed by facebook-listener (comments polling) and '
    'moderation-service (delete/hide comment, ban/unban page user).';

COMMENT ON COLUMN facebook_oauth_tokens.access_token IS
    'Page access token, encrypted with the shared multi-key chain. Read '
    'through the per-service decryptors; never logged.';

INSERT INTO feature_gates (feature_key, is_premium, description)
VALUES (
    'platform_facebook',
    TRUE,
    'Facebook Live comment ingest + moderation (ADR-0060): Graph API polling of the streamer''s Page live video, and the moderation write path (delete/hide comment, ban/unban page user) via pages_manage_engagement'
)
ON CONFLICT (feature_key) DO NOTHING;

COMMIT;
