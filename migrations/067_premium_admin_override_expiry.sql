-- All-Chat Migration 067: Time-limited admin premium overrides (ADR-0027)
-- Migration: 067
--
-- Problem (ADR-0027): the admin premium override (users.premium_admin_override /
-- viewers.premium_admin_override, ADR-0018/0019) is a permanent-until-revoked
-- tri-state. Admins want to grant premium for a LIMITED time (e.g. a 7-day comp)
-- to a streamer (users.is_premium) and/or a viewer (viewers.is_premium).
--
-- This migration adds an optional expiry to each override:
--
--   - users.premium_admin_override_expires_at
--   - viewers.premium_admin_override_expires_at
--
-- Semantics (implemented in shared/premium.Recompute / RecomputeViewer): when the
-- expiry is set AND already past (<= NOW()), the override is treated as absent and
-- premium falls through to the subscription half — exactly as if the admin had
-- never set it. NULL expiry = permanent (today's behaviour, unchanged). Only the
-- admin-override input gains time; the subscription half stays time-free and keeps
-- honoring Patreon's own grace window (ADR-0018).
--
-- The materialized is_premium is refreshed on any recompute AND by the
-- payment-service expiry sweep (single replica), which clears due overrides so a
-- grant that lapses with no other write still converges to not-premium.
--
-- IDEMPOTENCY: the runner re-executes every migration on each pod start, so both
-- adds are ADD COLUMN IF NOT EXISTS with no DEFAULT value written — no entitlement
-- VALUES are set, so a re-run can never shorten or clobber a live grant. Guarded by
-- services/auth-service/repository/migrations_rerun_test.go.

BEGIN;

-- 1. Streamer (users) override expiry.
ALTER TABLE users ADD COLUMN IF NOT EXISTS premium_admin_override_expires_at TIMESTAMP NULL;

CREATE INDEX IF NOT EXISTS idx_users_premium_override_expiry
    ON users(premium_admin_override_expires_at)
    WHERE premium_admin_override_expires_at IS NOT NULL;

COMMENT ON COLUMN users.premium_admin_override_expires_at IS
    'Time-limited admin premium (ADR-0027): when set, premium_admin_override is ignored once past this instant (premium falls through to the subscription). NULL = permanent. Written by share-service admin endpoint; cleared by the payment-service expiry sweep. Only affects the override half of shared/premium.Recompute — the subscription half stays time-free.';

-- 2. Viewer (viewers) override expiry, mirroring users.
ALTER TABLE viewers ADD COLUMN IF NOT EXISTS premium_admin_override_expires_at TIMESTAMP NULL;

CREATE INDEX IF NOT EXISTS idx_viewers_premium_override_expiry
    ON viewers(premium_admin_override_expires_at)
    WHERE premium_admin_override_expires_at IS NOT NULL;

COMMENT ON COLUMN viewers.premium_admin_override_expires_at IS
    'Time-limited admin premium (ADR-0027): when set, premium_admin_override is ignored once past this instant (premium falls through to the viewer subscription / linked-streamer inheritance). NULL = permanent. Written by auth-service admin endpoint; cleared by the payment-service expiry sweep.';

COMMIT;
