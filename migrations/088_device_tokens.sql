-- Migration: 088_device_tokens
-- Description: Device tokens and pending device-link requests for desktop control surfaces
--   (ADR-0049, steps 2-3). A device token is what a *paired* Stream Deck / StreamController
--   plugin authenticates with: `Authorization: Bearer allchat_dev_…`, resolved by
--   shared/middleware/devicetoken.go behind the same APITokenResolver seam migration 086's
--   personal access tokens use.
--
-- This is deliberately NOT a replacement for api_tokens (migration 086). ADR-0049's loopback
-- flow cannot reach a headless capture box, a second PC or a self-hoster's CLI, which is
-- exactly what a pasted PAT is for. Both credential types coexist, on the same resolver seam,
-- distinguished by their bearer prefix.
--
-- Three things a device token has that a PAT structurally cannot (ADR-0049, "Scope of a
-- device token"), and each is a column below:
--
--   1. PER-OVERLAY BINDING. device_tokens.overlay_id is set at pairing time and is NOT NULL,
--      so a compromised control surface cannot drive a different overlay. A PAT is
--      user-scoped, which ADR-0051 records as a residual risk.
--   2. NOTHING IS TYPED OR PASTED. The secret travels from the exchange endpoint to the
--      plugin over the loopback redirect and never appears on screen, so it cannot be read
--      aloud, screenshotted or leaked on camera. The dashboard never renders it.
--   3. SLIDING EXPIRY. expires_at is NOT NULL (unlike api_tokens.expires_at) and is pushed
--      forward on use, so an abandoned pairing lapses on its own instead of living until
--      somebody notices.
--
-- The token is NEVER stored. Only a SHA-256 digest lands here, in BYTEA, exactly as
-- api_tokens.token_hash (086) and overlay_moderators.invite_token_hash (080): same hash, same
-- column type, same "returned exactly once, then unrecoverable" contract. A digest cannot be
-- replayed by whoever reads a database dump, and no code path can leak a plaintext from this
-- table because the plaintext is not in it. The same applies to device_link_requests.
-- user_code_hash: the pairing code a streamer types is hashed at rest for the same reason.
--
-- Idempotent throughout: scripts/run-migrations.sh re-applies every migration on each pod
-- start and tracks nothing, so a non-idempotent statement would crash-loop fresh pods.
--
-- Deliberately NOT here: any INSERT of credential material. A device token is user intent, and
-- a migration that seeded one would resurrect a revoked token — with a plaintext secret in the
-- repo — on every pod restart.
--
-- Authentication only, never authorization: a device token identifies a user exactly as a
-- session JWT does, and remains subject to every downstream check (premium feature gates,
-- ownership, the advanced-controls consent grant). Scopes and the overlay binding narrow what
-- a token may do; they never widen it.

BEGIN;

-- ---------------------------------------------------------------------------
-- device_tokens: the credential itself.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS device_tokens (
    id           UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID         NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    -- The overlay this device may drive. NOT NULL because the binding is the point: it is
    -- decided on the approve screen and can never be widened afterwards (re-pair instead).
    overlay_id   UUID         NOT NULL REFERENCES overlays(id) ON DELETE CASCADE,
    -- Self-reported device name from the plugin, shown in the paired-devices list so a
    -- streamer can tell "Stream Deck (studio PC)" from "laptop". Self-reported means
    -- untrusted for anything but display; the dashboard labels it as such.
    name         VARCHAR(120) NOT NULL,
    -- SHA-256 of the plaintext token, exactly as api_tokens.token_hash. UNIQUE both because a
    -- digest collision would be an authentication ambiguity and because the lookup on every
    -- authenticated request goes through this index.
    token_hash   BYTEA        NOT NULL UNIQUE,
    -- Least privilege (ADR-0012), e.g. {chat:write,engagement:write}. Empty means "no write
    -- scope": the token can authenticate but passes no scope check.
    scopes       TEXT[]       NOT NULL DEFAULT '{}',
    created_at   TIMESTAMP    NOT NULL DEFAULT NOW(),
    -- Best-effort telemetry for the paired-devices list ("last used 3 days ago"). Written on
    -- the auth path under a one-write-per-minute throttle, never read by it.
    last_used_at TIMESTAMP,
    -- NOT NULL, unlike api_tokens.expires_at. Default lifetime is 90 days, slid forward to
    -- NOW() + 90 days on use, so a device in daily service never expires while an abandoned
    -- pairing lapses by itself. The default here is a safety net; the issuing code sets it.
    expires_at   TIMESTAMP    NOT NULL DEFAULT (NOW() + INTERVAL '90 days'),
    -- Set once, never cleared: revocation is permanent, and the row is kept so the list can
    -- still show what was revoked and when.
    revoked_at   TIMESTAMP
);

-- Serves the paired-devices list ("my devices, newest first").
CREATE INDEX IF NOT EXISTS idx_device_tokens_user
    ON device_tokens (user_id, created_at DESC);

-- Serves the hot path: resolve a live token by digest. Partial, so revoked rows do not bloat it.
CREATE INDEX IF NOT EXISTS idx_device_tokens_live
    ON device_tokens (token_hash)
    WHERE revoked_at IS NULL;

COMMENT ON TABLE device_tokens IS
    'Paired-device credentials for desktop control surfaces (ADR-0049). Presented as `Authorization: Bearer allchat_dev_<secret>` and resolved by shared/middleware/devicetoken.go behind the same resolver seam as api_tokens (ADR-0051). AUTHENTICATION ONLY: a resolved device token populates the same request identity a session JWT would and stays subject to every premium gate and ownership check. Distinct from api_tokens in three ways: bound to one overlay, never typed or pasted by a human, and with a mandatory sliding expiry.';
COMMENT ON COLUMN device_tokens.overlay_id IS
    'The one overlay this device may drive, fixed at pairing time. Enforced by middleware.RequireDeviceTokenOverlay on overlay-keyed routes. NOTE the honest limit: routes with no overlay dimension (POST /api/v1/auth/chat/send fans out to the account''s connected platforms) are narrowed by scopes only, not by this column.';
COMMENT ON COLUMN device_tokens.token_hash IS
    'SHA-256 of the plaintext token (same convention as api_tokens.token_hash, migration 086). The plaintext is returned exactly once, by the device-link exchange endpoint, to the plugin over the loopback redirect — and is never rendered in the dashboard, stored, logged or retrievable afterwards.';
COMMENT ON COLUMN device_tokens.scopes IS
    'Capability strings (e.g. chat:write, engagement:write) enforced server-side IN ADDITION to the existing premium gates, never instead of them.';
COMMENT ON COLUMN device_tokens.expires_at IS
    'Mandatory expiry, default 90 days, slid forward to NOW() + 90 days on use under a one-write-per-minute throttle. NOT NULL on purpose: unlike a PAT, an abandoned device pairing must lapse without anyone noticing it.';
COMMENT ON COLUMN device_tokens.revoked_at IS
    'Revocation timestamp. Read live on every request, so revoking takes effect within one request; rows are retained for the paired-devices list rather than deleted.';

-- ---------------------------------------------------------------------------
-- device_link_requests: the pending-link row, serving BOTH delivery paths.
--
-- One table and one state machine for the loopback flow (RFC 8252) and the typed-code
-- fallback (RFC 8628), because they differ only in how the streamer's approval reaches the
-- plugin. Two tables would mean two state machines to keep correct, and ADR-0049 already
-- notes the fallback is the path that will rot.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS device_link_requests (
    id               UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    -- 'loopback' (the primary path: browser redirects to 127.0.0.1) or 'code' (the fallback:
    -- the streamer types an 8-character code shown by the plugin). CHECK-constrained rather
    -- than an enum type so the migration stays trivially re-runnable.
    flow             VARCHAR(16)  NOT NULL,
    -- SHA-256 of the user code, NULL for the loopback flow which shows no code. Hashed for the
    -- same reason every other secret here is: a database dump must not let someone approve a
    -- pending link.
    user_code_hash   BYTEA,
    -- PKCE (RFC 7636). The challenge is stored, the verifier never is: the plugin keeps the
    -- verifier and presents it at exchange, which is what replaces a client secret in a public
    -- client. S256 only — `plain` is rejected at the endpoint, not merely discouraged.
    pkce_challenge   TEXT         NOT NULL,
    pkce_method      VARCHAR(8)   NOT NULL DEFAULT 'S256',
    -- The validated loopback redirect (http://127.0.0.1:<port>/<fixed path>), NULL for the
    -- code flow. Validated by services/auth-service/handlers/loopback_redirect.go before it is
    -- ever written here, so a row can only carry a redirect that already passed the rule.
    redirect_uri     TEXT,
    -- Self-reported by the plugin and shown on the approve screen, clearly labelled as such.
    device_name      VARCHAR(120) NOT NULL,
    requested_scopes TEXT[]       NOT NULL DEFAULT '{}',
    -- NULL until approved: an unapproved request belongs to nobody, which is what makes the
    -- approve step meaningful. Both are set in one UPDATE by the approve handler.
    user_id          UUID         REFERENCES users(id) ON DELETE CASCADE,
    overlay_id       UUID         REFERENCES overlays(id) ON DELETE CASCADE,
    -- What the streamer actually granted, which may be narrower than requested_scopes.
    granted_scopes   TEXT[]       NOT NULL DEFAULT '{}',
    -- The brute-force bound for the typed code, checked and incremented in SQL so two
    -- concurrent guesses cannot both see the same count. At 5 the row is dead.
    attempts         INTEGER      NOT NULL DEFAULT 0,
    -- Short TTL: 10 minutes for the code flow, and the one-time authorization code minted at
    -- approval gets its own <= 5 minute window (see auth_code_expires_at).
    expires_at       TIMESTAMP    NOT NULL,
    approved_at      TIMESTAMP,
    -- SHA-256 of the one-time authorization code handed to the loopback redirect. Set at
    -- approval, cleared on first exchange attempt whether or not that attempt succeeded.
    auth_code_hash   BYTEA,
    auth_code_expires_at TIMESTAMP,
    -- Set when the request has produced (or failed to produce) a token. A consumed row is
    -- terminal: a second exchange of the same code is a 400 and revokes the minted token,
    -- because a replay means the code leaked.
    consumed_at      TIMESTAMP,
    -- The token this request minted, so a replayed exchange knows what to revoke.
    device_token_id  UUID         REFERENCES device_tokens(id) ON DELETE SET NULL,
    created_at       TIMESTAMP    NOT NULL DEFAULT NOW(),
    CONSTRAINT device_link_requests_flow_check
        CHECK (flow IN ('loopback', 'code')),
    CONSTRAINT device_link_requests_pkce_method_check
        CHECK (pkce_method = 'S256')
);

-- Serves the fallback lookup: "find the pending request for this typed code".
CREATE INDEX IF NOT EXISTS idx_device_link_requests_user_code
    ON device_link_requests (user_code_hash)
    WHERE consumed_at IS NULL;

-- Serves the expiry sweep, which deletes rows that were never completed.
CREATE INDEX IF NOT EXISTS idx_device_link_requests_expiry
    ON device_link_requests (expires_at)
    WHERE consumed_at IS NULL;

-- Serves the approve screen's "which of my pending requests" and the audit view.
CREATE INDEX IF NOT EXISTS idx_device_link_requests_user
    ON device_link_requests (user_id, created_at DESC);

COMMENT ON TABLE device_link_requests IS
    'Pending device-link requests (ADR-0049). One table serves both delivery paths — the loopback redirect (RFC 8252, flow=loopback) and the typed pairing code (RFC 8628, flow=code) — because they mint the identical credential and differ only in how approval reaches the plugin. Rows are short-lived and terminal once consumed_at is set.';
COMMENT ON COLUMN device_link_requests.user_code_hash IS
    'SHA-256 of the 8-character pairing code, NULL for the loopback flow. Hashed at rest so a database dump cannot be used to approve a pending link.';
COMMENT ON COLUMN device_link_requests.pkce_challenge IS
    'PKCE code challenge (RFC 7636). Only the challenge is stored; the verifier stays in the plugin and is presented at exchange, which is what lets a public client authenticate without a secret.';
COMMENT ON COLUMN device_link_requests.attempts IS
    'Failed pairing-code attempts, checked and incremented in SQL. Five is the ceiling, and it is the actual brute-force bound for the fallback path (the gateway rate limit is defence in depth, not the bound).';
COMMENT ON COLUMN device_link_requests.auth_code_hash IS
    'SHA-256 of the one-time authorization code delivered via the loopback redirect. Cleared on the first exchange attempt whether or not it succeeded, so the code is single-use by construction.';
COMMENT ON COLUMN device_link_requests.consumed_at IS
    'Terminal marker. A second exchange against a consumed row is a 400 AND revokes the token it minted: a replay means the code leaked.';

COMMIT;
