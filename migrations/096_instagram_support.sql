-- Migration 096: Instagram platform support (ADR-0062)
--
-- Three things, in dependency order:
--
-- 1. supported_platforms row — the overlay_chat_sources.platform FK refuses an
--    unknown value, so 'instagram' must exist before any source can be created.
--
-- 2. instagram_oauth_tokens — the streamer's Instagram credential, resolved
--    through the Facebook Page linked to their IG professional account. The
--    token chain mirrors Facebook's (ADR-0060): authorization code ->
--    short-lived user token -> long-lived user token (~60 days) -> Page access
--    token via /me/accounts. A Page token from a long-lived user token does
--    NOT expire (Meta's Access Tokens doc), so unlike youtube/kick/twitch
--    there is NO refresh_token column — but unlike facebook_oauth_tokens the
--    expires_at column IS kept, NULL by default (the non-expiring
--    Page-derived token) and set when a 60-day long-lived user token is
--    stored instead; an expired row makes instagram-listener deactivate the
--    source so the streamer re-auths. The row is replaced wholesale on
--    re-consent and dies only when the streamer revokes the app or changes
--    their password. token-refresh-service deliberately has nothing to do for
--    Instagram.
--
-- 3. feature_gates seed platform_instagram (ADR-0008): seeded is_premium=TRUE
--    = rollout cohort, flipped via the admin feature-gates endpoint without a
--    redeploy. Enforcement (overlay-manager gate cache + source-add check) is
--    wired centrally after merge.
--
-- IDEMPOTENCY: the migration runner replays every migration on each pod
-- restart, so every statement is safe to re-execute (CREATE IF NOT EXISTS,
-- ON CONFLICT DO NOTHING).

BEGIN;

INSERT INTO supported_platforms (platform, display_name, is_enabled, requires_oauth, config_schema)
VALUES ('instagram', 'Instagram', true, true,
        '{"api_type": "graph_api_polling", "graph_version": "v26.0", "requires_ig_user_token": true}'::jsonb)
ON CONFLICT (platform) DO NOTHING;

CREATE TABLE IF NOT EXISTS instagram_oauth_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    ig_user_id VARCHAR(255) NOT NULL,           -- Instagram professional account ID (the source's channel_id)
    ig_username VARCHAR(255),                   -- display only; refreshed on re-consent
    access_token TEXT NOT NULL,                 -- encrypted token (Page-derived: non-expiring, expires_at NULL)
    granted_scopes TEXT[] NOT NULL DEFAULT '{}',-- permissions granted at consent (instagram_basic, instagram_manage_comments, pages_show_list)
    encryption_version INT NOT NULL DEFAULT 1,  -- AES-GCM multi-key chain (D-04); 1 = encrypted writes
    expires_at TIMESTAMPTZ,                     -- NULL = non-expiring Page-derived token; set for a 60d long-lived user token
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, ig_user_id)
);

CREATE INDEX IF NOT EXISTS idx_instagram_oauth_user_id ON instagram_oauth_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_instagram_oauth_ig_user_id ON instagram_oauth_tokens(ig_user_id);

-- The token is per (user, ig user): one Facebook account must not be able to
-- claim another user's IG account (mirrors the facebook_oauth_tokens reasoning).
CREATE UNIQUE INDEX IF NOT EXISTS uq_instagram_oauth_user_ig_user
    ON instagram_oauth_tokens (user_id, ig_user_id);

COMMENT ON TABLE instagram_oauth_tokens IS
    'Instagram professional-account credentials per user (ADR-0062). The '
    'stored token is the Page token for the Page linked to the IG account: '
    'non-expiring (expires_at NULL) like Facebook''s, but expires_at is set '
    'when a 60-day long-lived user token is stored instead — an expired row '
    'makes instagram-listener deactivate the source for re-auth. Replaced on '
    're-consent, invalidated only by revocation or a password change. '
    'Consumed by instagram-listener (live_comments polling); ingest is '
    'read-only, there is no moderation write path for Instagram.';

COMMENT ON COLUMN instagram_oauth_tokens.access_token IS
    'Instagram/Page access token, encrypted with the shared multi-key chain. '
    'Read through the per-service decryptors; never logged.';

COMMENT ON COLUMN instagram_oauth_tokens.expires_at IS
    'NULL for a non-expiring Page-derived token; set to the ~60-day horizon '
    'when a long-lived user token is stored. instagram_oauth_tokens rows past '
    'this point are treated as absent by the listener.';

INSERT INTO feature_gates (feature_key, is_premium, description)
VALUES (
    'platform_instagram',
    TRUE,
    'Instagram Live comment ingest (ADR-0062): Graph API polling of the streamer''s IG professional account''s live broadcast via live_comments. Read-only: no moderation write path for Instagram'
)
ON CONFLICT (feature_key) DO NOTHING;

COMMIT;
