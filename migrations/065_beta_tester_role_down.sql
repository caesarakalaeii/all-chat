-- Migration 065 DOWN: remove the beta-tester role + early-access gate columns.
--
-- Dropping users.is_beta_tester removes a premium input; any user who was premium
-- ONLY by virtue of being a beta tester will revert to following their
-- subscription/override on the next shared/premium.Recompute. Dropping
-- feature_gates.early_access makes every gate non-early-access (RequireEarlyAccess
-- then allows all authenticated users for those keys). Both are reversible schema
-- changes; no grants to revoke (column-level, covered by the table grants).

BEGIN;

DROP INDEX IF EXISTS idx_users_is_beta_tester;
ALTER TABLE users DROP COLUMN IF EXISTS is_beta_tester;

ALTER TABLE feature_gates DROP COLUMN IF EXISTS early_access;

COMMIT;
