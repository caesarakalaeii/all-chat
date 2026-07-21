-- Migration 078 DOWN: remove the ambassador role + public showcase (ADR-0041).
--
-- Dropping users.is_ambassador removes a premium input; any user who was premium
-- ONLY by virtue of being an ambassador reverts to following their
-- subscription/override on the next shared/premium.Recompute (and loses the
-- early-access capability). Dropping ambassador_showcase removes the public
-- "Featured Ambassadors" card metadata (tagline / sort_order / consent). Both are
-- reversible schema changes; no grants to revoke (column/table-level, covered by
-- the table grants).

BEGIN;

DROP TABLE IF EXISTS ambassador_showcase;

DROP INDEX IF EXISTS idx_users_is_ambassador;
ALTER TABLE users DROP COLUMN IF EXISTS is_ambassador;

COMMIT;
