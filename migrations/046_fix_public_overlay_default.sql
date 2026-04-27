-- Migration: Fix public overlay default and backfill existing overlays
-- Created: 2026-04-01
-- Purpose: New overlays should be public to viewers by default. Existing active
--          overlays that were created before this fix need to be backfilled.

-- Change column default so new rows inserted at DB level are also public
ALTER TABLE overlays ALTER COLUMN is_public_for_viewers SET DEFAULT true;

-- Backfill: make all existing active overlays public for viewers.
-- This was a one-time backfill before migration 048 added a unique constraint
-- (idx_overlays_one_public_per_user). After 048 lands, blindly re-running this
-- UPDATE turns multiple active overlays per user public again, violating the
-- unique constraint. Guarded so it only runs when the constraint isn't yet
-- in place — safe under ON_ERROR_STOP=1 on every subsequent pod startup.
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_indexes
        WHERE schemaname = 'public'
          AND indexname = 'idx_overlays_one_public_per_user'
    ) THEN
        UPDATE overlays
        SET is_public_for_viewers = true, updated_at = NOW()
        WHERE is_public_for_viewers = false
          AND is_active = true;
    END IF;
END $$;
