-- All-Chat Migration 063: Patreon-based premium entitlements
-- Migration: 063
--
-- Problem (ADR-0018): premium (users.is_premium) can only be granted manually by
-- an admin (POST /api/v1/admin/premium/users/:id, share-service). There is no
-- self-serve monetization. We want users to unlock premium by backing all-chat's
-- own Patreon campaign.
--
-- This migration makes users.is_premium a DERIVED column with two independent
-- inputs, so the new payment-service can be a second writer without clobbering
-- admin grants:
--
--   is_premium = (premium_admin_override IS TRUE)
--                OR (premium_admin_override IS NULL
--                    AND <user has an 'active' premium_subscriptions row>)
--
-- It is recomputed by shared/premium.RecomputePremium after any change to either
-- input. The admin endpoint now writes premium_admin_override (tri-state) instead
-- of the raw boolean; payment-service writes premium_subscriptions. Existing
-- readers (shared/middleware/premium.go, moderation-service) are unchanged — they
-- keep reading users.is_premium.
--
-- patreon_oauth_tokens stores the per-user Patreon credentials (encrypted at rest
-- with the shared multi-key cipher, encryption_version=1, like the other
-- *_oauth_tokens tables) so the reconcile job can re-query membership and refresh
-- tokens.
--
-- IDEMPOTENCY: the runner re-executes every migration on each pod start, so
-- everything here is IF NOT EXISTS / DO-block / CREATE OR REPLACE. Crucially this
-- migration NEVER writes is_premium or premium_admin_override VALUES, so a re-run
-- can never clobber a live grant. Guarded by
-- services/auth-service/repository/migrations_rerun_test.go.

BEGIN;

-- 1. Tri-state admin override. NULL = no admin opinion (follow subscription),
--    TRUE = force-grant (comp/staff/partner), FALSE = force-deny (reserved).
--    Replaces the raw admin write to users.is_premium.
ALTER TABLE users ADD COLUMN IF NOT EXISTS premium_admin_override BOOLEAN DEFAULT NULL;

COMMENT ON COLUMN users.premium_admin_override IS
    'Admin premium decision (ADR-0018): NULL=follow subscription, TRUE=force-grant, FALSE=force-deny. Written by share-service admin endpoint. users.is_premium is derived from this + premium_subscriptions by shared/premium.RecomputePremium.';

-- 2. Per-user Patreon OAuth credentials, encrypted at rest (encryption_version=1).
--    One connection per all-chat user, and a Patreon account maps to at most one
--    all-chat user.
CREATE TABLE IF NOT EXISTS patreon_oauth_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    patreon_user_id VARCHAR(64) NOT NULL,
    access_token TEXT NOT NULL,
    refresh_token TEXT NOT NULL,
    token_expires_at TIMESTAMP NOT NULL,
    granted_scopes TEXT[] NOT NULL DEFAULT '{}',
    encryption_version INT NOT NULL DEFAULT 1,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(user_id),
    UNIQUE(patreon_user_id)
);

-- Webhooks resolve the all-chat user by Patreon user id.
CREATE INDEX IF NOT EXISTS idx_patreon_oauth_tokens_patreon_user
    ON patreon_oauth_tokens (patreon_user_id);

-- The reconcile job scans by expiry to refresh tokens nearing expiry.
CREATE INDEX IF NOT EXISTS idx_patreon_oauth_tokens_expiry
    ON patreon_oauth_tokens (token_expires_at);

-- 3. Subscription state = source of truth for the subscription half of is_premium.
--    user_id is nullable: a webhook can arrive before the user links their Patreon
--    (the reconcile job / OAuth callback fills it in later).
CREATE TABLE IF NOT EXISTS premium_subscriptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    provider VARCHAR(20) NOT NULL DEFAULT 'patreon',
    provider_user_id VARCHAR(64) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'none',
    tier_id VARCHAR(64),
    cents INTEGER NOT NULL DEFAULT 0,
    current_period_end TIMESTAMP,
    raw JSONB,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- UPSERT target / idempotency key for webhook + reconcile + OAuth callback.
CREATE UNIQUE INDEX IF NOT EXISTS uq_premium_subscriptions_provider_user
    ON premium_subscriptions (provider, provider_user_id);

-- RecomputePremium reads all of a user's subscription rows.
CREATE INDEX IF NOT EXISTS idx_premium_subscriptions_user
    ON premium_subscriptions (user_id);

-- status CHECK via DO-block (ADD CONSTRAINT is not IF-NOT-EXISTS-able; the
-- DO-block keeps it re-runnable per the runner's audited patterns).
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'premium_subscriptions_status_chk'
    ) THEN
        ALTER TABLE premium_subscriptions
            ADD CONSTRAINT premium_subscriptions_status_chk
            CHECK (status IN ('none', 'active', 'declined', 'former', 'expired'));
    END IF;
END $$;

-- updated_at trigger (reuse the shared function from migration 044).
DROP TRIGGER IF EXISTS update_premium_subscriptions_updated_at ON premium_subscriptions;
CREATE TRIGGER update_premium_subscriptions_updated_at
    BEFORE UPDATE ON premium_subscriptions
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- CNPG runs migrations as superuser; the app connects as allchat_user.
-- Guarded so local dev / test containers without the role still work (see 035).
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'allchat_user') THEN
        GRANT ALL ON patreon_oauth_tokens TO allchat_user;
        GRANT ALL ON premium_subscriptions TO allchat_user;
    END IF;
END $$;

-- Extend the DSGVO cleanup (047/056) to Patreon credentials: tokens expired 7+
-- days (i.e. the reconcile job gave up refreshing them) serve no purpose. Copies
-- the current (056) body verbatim and adds the patreon_oauth_tokens delete.
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

    -- Patreon premium credentials
    DELETE FROM patreon_oauth_tokens
    WHERE token_expires_at < NOW() - INTERVAL '7 days';

    -- Viewer sessions with expired tokens (not refreshed for 7+ days)
    DELETE FROM viewer_sessions
    WHERE token_expires_at < NOW() - INTERVAL '7 days';
END;
$$ LANGUAGE plpgsql;

COMMENT ON TABLE patreon_oauth_tokens IS
    'Per-user Patreon OAuth credentials (encrypted, encryption_version=1) for membership re-query and token refresh by payment-service (ADR-0018).';

COMMENT ON TABLE premium_subscriptions IS
    'Patreon premium subscription state — source of truth for the subscription half of users.is_premium (ADR-0018). Upserted by payment-service via webhook/reconcile/OAuth callback; one row per (provider, provider_user_id). A row grants premium iff status=''active''.';

COMMIT;
