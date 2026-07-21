-- All-Chat Migration 078: Ambassador role + public showcase (ADR-0041)
-- Migration: 078
--
-- Problem (ADR-0041): we want to recognise a small set of streamers as
-- "ambassadors". An ambassador is an admin-granted recognition role that (a)
-- grants all premium features, (b) grants the beta-tester / early-access
-- capability, and (c) opts the streamer into a public "Featured Ambassadors"
-- showcase on the marketing homepage. Premium is the derived column
-- users.is_premium (ADR-0018) and early-access is gated by
-- feature_gates.early_access + shared/middleware.RequireEarlyAccess (ADR-0020).
--
-- This migration adds:
--
--   1. users.is_ambassador — an independent, admin-granted flag, modelled on
--      is_beta_tester (ADR-0020). It is folded into the is_premium derivation
--      (an ambassador is premium) by shared/premium.Recompute, and folded into
--      the early-access check (is_beta_tester OR is_ambassador) by
--      shared/middleware.RequireEarlyAccess. Granting it force-grants premium the
--      same clobber-free way premium_admin_override does.
--
--   2. ambassador_showcase — a separate presentation table (NOT columns on users)
--      holding the marketing-card metadata: an admin-curated tagline + sort_order
--      and the streamer's own featured_consent opt-in. Kept off the users row so
--      the entitlement column (is_ambassador) stays a single boolean and the
--      ~9 users SELECT/scan sites in auth-service only gain one field. The public
--      homepage endpoint joins this to users; a streamer never appears publicly
--      until featured_consent = TRUE (opt-in, per ADR-0041).
--
-- GRANTS ARE ADMIN ACTIONS, NOT A DATA MIGRATION: ambassadors are assigned via
-- the admin API (share-service POST /api/v1/admin/ambassadors/users/:id). This
-- migration deliberately writes NO is_ambassador VALUES and NO showcase rows — a
-- blanket UPDATE/INSERT would be re-applied by the re-running migration runner on
-- every pod start (the 009-incident class of bug).
--
-- IDEMPOTENCY: the runner re-executes every migration on each pod start, so every
-- statement is IF NOT EXISTS / ON CONFLICT DO NOTHING with a DEFAULT only — no
-- entitlement or consent VALUES are written, so a re-run can never clobber a live
-- grant or a streamer's consent choice. Guarded by
-- services/auth-service/repository/migrations_rerun_test.go.

BEGIN;

-- 1. Ambassador role: admin-granted, independent of subscription/override.
--    Folded into is_premium by shared/premium.Recompute (an ambassador is premium)
--    and into the early-access check by shared/middleware.RequireEarlyAccess.
ALTER TABLE users ADD COLUMN IF NOT EXISTS is_ambassador BOOLEAN NOT NULL DEFAULT FALSE;

CREATE INDEX IF NOT EXISTS idx_users_is_ambassador ON users(is_ambassador) WHERE is_ambassador = TRUE;

COMMENT ON COLUMN users.is_ambassador IS
    'Ambassador role (ADR-0041): admin-granted recognition role. Grants all premium features (folded into is_premium by shared/premium.Recompute) PLUS early-access (folded into shared/middleware.RequireEarlyAccess as is_beta_tester OR is_ambassador). Public showcase metadata lives in ambassador_showcase. Written by share-service POST /api/v1/admin/ambassadors/users/:id. Grants are admin actions, never a data migration.';

-- 2. Ambassador showcase: presentation metadata for the public homepage card.
--    tagline + sort_order are admin-curated; featured_consent is the streamer's own
--    opt-in (default FALSE) — an ambassador is NOT shown publicly until they enable it.
CREATE TABLE IF NOT EXISTS ambassador_showcase (
    user_id          UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    tagline          TEXT,
    sort_order       INTEGER NOT NULL DEFAULT 0,
    featured_consent BOOLEAN NOT NULL DEFAULT FALSE,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Public homepage query filters on is_ambassador (users) AND featured_consent
-- (this table) and orders by sort_order; index the ordering/consent read path.
CREATE INDEX IF NOT EXISTS idx_ambassador_showcase_featured
    ON ambassador_showcase(featured_consent, sort_order) WHERE featured_consent = TRUE;

COMMENT ON TABLE ambassador_showcase IS
    'Public "Featured Ambassadors" homepage card metadata (ADR-0041). tagline + sort_order are admin-curated; featured_consent is the streamer opt-in (default FALSE). Joined to users WHERE users.is_ambassador AND ambassador_showcase.featured_consent by the public GET /api/v1/ambassadors endpoint.';

COMMIT;
