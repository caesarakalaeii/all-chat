-- All-Chat Migration 065: Beta-Testers role + early-access feature gates (ADR-0020)
-- Migration: 065
--
-- Problem (ADR-0020): we want to thank the ~5 users who held premium BEFORE paid
-- monetization (ADR-0018) by giving them a "beta tester" status that grants all
-- premium features PLUS early-access ones. Premium today is the derived column
-- users.is_premium (ADR-0018) and feature access is gated by feature_gates +
-- shared/middleware. There is no notion of an early-access feature and no role
-- above premium.
--
-- This migration adds the two columns that make beta-testers a first-class,
-- admin-granted status:
--
--   - users.is_beta_tester: an independent, admin-granted flag. It is folded into
--     the is_premium derivation (a beta tester is premium) by
--     shared/premium.Recompute, so granting it force-grants premium the same
--     clobber-free way premium_admin_override does, while ALSO unlocking
--     early-access features that plain premium does not.
--
--   - feature_gates.early_access: a per-gate flag orthogonal to is_premium. A gate
--     with early_access = TRUE is reachable only by beta testers
--     (shared/middleware.RequireEarlyAccess); flipping it FALSE "graduates" the
--     feature. is_premium and early_access compose: a beta tester passes both.
--
-- GRANDFATHERING IS MANUAL, NOT A DATA MIGRATION (product decision 2026-06-20):
-- with only ~5 users to move, an admin grants each one via the new
-- "Grant Beta Tester" button (share-service POST /api/v1/admin/beta-tester/
-- users/:id). This migration deliberately writes NO is_beta_tester VALUES — a
-- blanket `UPDATE ... WHERE is_premium = TRUE` would be re-applied by the
-- re-running migration runner on every pod start (the 009-incident class of bug).
--
-- IDEMPOTENCY: the runner re-executes every migration on each pod start, so both
-- adds are ADD COLUMN IF NOT EXISTS with a DEFAULT only — no entitlement VALUES are
-- written, so a re-run can never clobber a live grant. Guarded by
-- services/auth-service/repository/migrations_rerun_test.go.

BEGIN;

-- 1. Beta-tester status: admin-granted, independent of subscription/override.
--    Folded into is_premium by shared/premium.Recompute (a beta tester is premium)
--    and read by shared/middleware.RequireEarlyAccess for early-access gates.
ALTER TABLE users ADD COLUMN IF NOT EXISTS is_beta_tester BOOLEAN NOT NULL DEFAULT FALSE;

CREATE INDEX IF NOT EXISTS idx_users_is_beta_tester ON users(is_beta_tester) WHERE is_beta_tester = TRUE;

COMMENT ON COLUMN users.is_beta_tester IS
    'Beta-tester role (ADR-0020): admin-granted thank-you for pre-monetization premium users. Grants all premium features (folded into is_premium by shared/premium.Recompute) PLUS early-access gates (shared/middleware.RequireEarlyAccess). Written by share-service POST /api/v1/admin/beta-tester/users/:id. Grandfathering is manual, never a data migration.';

-- 2. Early-access feature gates: a dimension orthogonal to is_premium.
--    TRUE => beta-testers only; FALSE => graduated (defer to is_premium / free).
ALTER TABLE feature_gates ADD COLUMN IF NOT EXISTS early_access BOOLEAN NOT NULL DEFAULT FALSE;

COMMENT ON COLUMN feature_gates.early_access IS
    'Early-access dimension (ADR-0020), orthogonal to is_premium. TRUE => reachable only by beta-testers (shared/middleware.RequireEarlyAccess); FALSE => graduated. is_premium and early_access compose: a beta tester passes both.';

COMMIT;
