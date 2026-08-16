-- Migration: 086_api_tokens
-- Description: Personal access tokens (PATs) for non-browser desktop clients — the Stream Deck
--   and StreamController plugins. Those clients cannot hold a cookie/JWT session, so they present
--   a long-lived token as `Authorization: Bearer allchat_pat_…`, resolved by shared/middleware.
--
-- The token is NEVER stored. Only a SHA-256 digest lands here, in BYTEA, deliberately mirroring
-- `overlay_moderators.invite_token_hash BYTEA` from migration 080: same hash, same column type,
-- same "shown exactly once, then unrecoverable" contract. A digest cannot be replayed by whoever
-- reads a database dump, and there is no code path that can leak a plaintext token from this
-- table because the plaintext is not in it.
--
-- Idempotent throughout: the migration runner re-applies every migration on each pod start, so
-- a non-idempotent statement would crash-loop fresh pods.
--
-- Deliberately NOT here: any INSERT of a token row. A token is user intent, and a migration that
-- seeded one would resurrect a revoked token — with a plaintext secret in the repo — on every
-- pod restart.
--
-- Authentication only, never authorization: a PAT identifies a user exactly as a session JWT
-- does, and remains subject to every downstream check (premium feature gates, ownership). Scopes
-- narrow what a token may do; they never widen it.

BEGIN;

CREATE TABLE IF NOT EXISTS api_tokens (
    id           UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID         NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    -- User-supplied label so a streamer can tell "Stream Deck (studio PC)" from "laptop".
    name         VARCHAR(120) NOT NULL,
    -- SHA-256 of the plaintext token, exactly as invite_token_hash in migration 080. UNIQUE
    -- both because a digest collision would be an authentication ambiguity and because the
    -- lookup on every authenticated request goes through this index.
    token_hash   BYTEA        NOT NULL UNIQUE,
    -- Least privilege, e.g. {chat:write} or {engagement:write}. Empty means "no write scope",
    -- which the resolver treats as a token that can authenticate but pass no scope check.
    scopes       TEXT[]       NOT NULL DEFAULT '{}',
    created_at   TIMESTAMP    NOT NULL DEFAULT NOW(),
    -- Best-effort telemetry so the management UI can show "last used 3 days ago" and a user can
    -- recognise a token they no longer need. Written on the auth path, never read by it.
    last_used_at TIMESTAMP,
    -- NULL = never expires. Desktop plugins are configured once and left alone, so a mandatory
    -- expiry would silently break a live stream; expiry is opt-in and enforced at resolve time.
    expires_at   TIMESTAMP,
    -- Set once, never cleared: revocation is permanent, and the row is kept so the management
    -- list can still show what was revoked and when.
    revoked_at   TIMESTAMP
);

-- Serves the management list ("my tokens, newest first").
CREATE INDEX IF NOT EXISTS idx_api_tokens_user
    ON api_tokens (user_id, created_at DESC);

-- Serves the hot path: resolve a live token by digest. Partial, so revoked rows do not bloat it.
CREATE INDEX IF NOT EXISTS idx_api_tokens_live
    ON api_tokens (token_hash)
    WHERE revoked_at IS NULL;

COMMENT ON TABLE api_tokens IS
    'Personal access tokens for non-browser clients (Stream Deck / StreamController plugins). Presented as `Authorization: Bearer allchat_pat_<secret>` and resolved by shared/middleware instead of the JWT path. AUTHENTICATION ONLY: a resolved token populates the same request identity a session JWT would and stays subject to every premium gate and ownership check — it is never an authorization bypass.';
COMMENT ON COLUMN api_tokens.token_hash IS
    'SHA-256 of the plaintext token (same convention as overlay_moderators.invite_token_hash, migration 080). The plaintext is returned exactly once by the create endpoint and is never stored, logged or retrievable afterwards.';
COMMENT ON COLUMN api_tokens.scopes IS
    'Capability strings (e.g. chat:write, engagement:write) enforced server-side IN ADDITION to the existing premium gates, never instead of them.';
COMMENT ON COLUMN api_tokens.last_used_at IS
    'Last successful authentication with this token. Telemetry for the management UI; never consulted when deciding whether the token is valid.';
COMMENT ON COLUMN api_tokens.expires_at IS
    'Optional hard expiry. NULL means the token lives until revoked — desktop plugins are set up once, so a forced expiry would break a stream mid-flight.';
COMMENT ON COLUMN api_tokens.revoked_at IS
    'Revocation timestamp. Read live on every request, so revoking takes effect within one request; rows are retained for the management list rather than deleted.';

COMMIT;
