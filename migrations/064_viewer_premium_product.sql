-- All-Chat Migration 064: Split streamer vs viewer premium (ADR-0019)
-- Migration: 064
--
-- Problem (ADR-0019): premium (ADR-0018) is user-only. premium_subscriptions and
-- patreon_oauth_tokens are keyed by users.id, and shared/premium derives only
-- users.is_premium. We want a cheaper, separately-priced VIEWER subscription that
-- grants viewers.is_premium (the cosmetic badge) to a "pure" viewer who has no
-- users account.
--
-- This migration generalizes the ADR-0018 schema to a POLYMORPHIC SUBJECT
-- (user | viewer) plus a tier-driven PRODUCT, reusing the same payment-service
-- pipeline, webhook, reconcile, and the Effective() rule:
--
--   - premium_subscriptions gains `product IN ('streamer','viewer')` + a nullable
--     `viewer_id`. A subscription is anchored to at most one subject.
--   - patreon_oauth_tokens becomes polymorphic: user_id is now nullable, a nullable
--     `viewer_id` is added, and EXACTLY ONE subject is set per connection. One
--     Patreon account still maps to one connection (UNIQUE(patreon_user_id) kept),
--     so one Patreon account grants premium to exactly one all-chat identity.
--   - viewers gains a tri-state `premium_admin_override`, mirroring
--     users.premium_admin_override, so shared/premium.RecomputeViewer can derive
--     viewers.is_premium the same clobber-free way users.is_premium is derived.
--
-- IDEMPOTENCY: the runner re-executes every migration on each pod start, so
-- everything here is IF NOT EXISTS / DO-block / ALTER ... IF EXISTS / partial
-- CREATE INDEX IF NOT EXISTS. Like 063 it writes NO entitlement VALUES (no
-- is_premium / override / product data writes beyond column DEFAULTs), so a re-run
-- can never clobber a live grant. Guarded by
-- services/auth-service/repository/migrations_rerun_test.go.

BEGIN;

-- 1. premium_subscriptions: add the product dimension and the viewer subject.
--    product DEFAULT 'streamer' keeps every existing/unlinked row a streamer sub.
ALTER TABLE premium_subscriptions
    ADD COLUMN IF NOT EXISTS product VARCHAR(16) NOT NULL DEFAULT 'streamer';

ALTER TABLE premium_subscriptions
    ADD COLUMN IF NOT EXISTS viewer_id UUID REFERENCES viewers(id) ON DELETE CASCADE;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'premium_subscriptions_product_chk'
    ) THEN
        ALTER TABLE premium_subscriptions
            ADD CONSTRAINT premium_subscriptions_product_chk
            CHECK (product IN ('streamer', 'viewer'));
    END IF;
END $$;

-- At most one subject: a row is anchored to a user XOR a viewer (or neither yet,
-- for a webhook that arrived before the patron connected).
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'premium_subscriptions_one_subject_chk'
    ) THEN
        ALTER TABLE premium_subscriptions
            ADD CONSTRAINT premium_subscriptions_one_subject_chk
            CHECK (NOT (user_id IS NOT NULL AND viewer_id IS NOT NULL));
    END IF;
END $$;

-- RecomputeViewer reads all of a viewer's viewer-product subscription rows.
CREATE INDEX IF NOT EXISTS idx_premium_subscriptions_viewer
    ON premium_subscriptions (viewer_id);

COMMENT ON COLUMN premium_subscriptions.product IS
    'Which product this subscription grants (ADR-0019): ''streamer'' -> users.is_premium, ''viewer'' -> viewers.is_premium. Determined by the connection subject + per-product PATREON_*_MIN_TIER_CENTS threshold.';
COMMENT ON COLUMN premium_subscriptions.viewer_id IS
    'Viewer subject for a viewer-product subscription (ADR-0019). Mutually exclusive with user_id.';

-- 2. patreon_oauth_tokens: polymorphic subject (user XOR viewer).
ALTER TABLE patreon_oauth_tokens ALTER COLUMN user_id DROP NOT NULL;

ALTER TABLE patreon_oauth_tokens
    ADD COLUMN IF NOT EXISTS viewer_id UUID REFERENCES viewers(id) ON DELETE CASCADE;

-- Replace the inline UNIQUE(user_id) (auto-named *_user_id_key in 063) with a
-- partial unique index, and add the matching one for viewers, so the upsert's
-- ON CONFLICT (user_id)/(viewer_id) WHERE ... NOT NULL has an arbiter index.
ALTER TABLE patreon_oauth_tokens DROP CONSTRAINT IF EXISTS patreon_oauth_tokens_user_id_key;
CREATE UNIQUE INDEX IF NOT EXISTS uq_patreon_oauth_tokens_user
    ON patreon_oauth_tokens (user_id) WHERE user_id IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_patreon_oauth_tokens_viewer
    ON patreon_oauth_tokens (viewer_id) WHERE viewer_id IS NOT NULL;

-- Exactly one subject per connection (a connection always belongs to someone).
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'patreon_oauth_tokens_one_subject_chk'
    ) THEN
        ALTER TABLE patreon_oauth_tokens
            ADD CONSTRAINT patreon_oauth_tokens_one_subject_chk
            CHECK ((user_id IS NOT NULL) <> (viewer_id IS NOT NULL));
    END IF;
END $$;

-- The reconcile job scans viewer connections too.
CREATE INDEX IF NOT EXISTS idx_patreon_oauth_tokens_viewer
    ON patreon_oauth_tokens (viewer_id);

COMMENT ON COLUMN patreon_oauth_tokens.viewer_id IS
    'Viewer subject for a viewer-scoped Patreon connection (ADR-0019). Exactly one of (user_id, viewer_id) is set.';

-- 3. viewers: tri-state admin override mirroring users.premium_admin_override.
--    NULL = follow subscription/inheritance, TRUE = force-grant, FALSE = force-deny.
ALTER TABLE viewers ADD COLUMN IF NOT EXISTS premium_admin_override BOOLEAN DEFAULT NULL;

COMMENT ON COLUMN viewers.premium_admin_override IS
    'Admin viewer-premium decision (ADR-0019): NULL=follow subscription+inheritance, TRUE=force-grant, FALSE=force-deny. viewers.is_premium is derived from this + an active viewer-product subscription + linked-streamer inheritance by shared/premium.RecomputeViewer.';

-- CNPG runs migrations as superuser; the app connects as allchat_user.
-- Guarded so local dev / test containers without the role still work (see 035/063).
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'allchat_user') THEN
        GRANT ALL ON premium_subscriptions TO allchat_user;
        GRANT ALL ON patreon_oauth_tokens TO allchat_user;
        GRANT ALL ON viewers TO allchat_user;
    END IF;
END $$;

COMMIT;
