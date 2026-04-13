-- Migration: Fix public overlay default and backfill existing overlays
-- Created: 2026-04-01
-- Purpose: New overlays should be public to viewers by default. Existing active
--          overlays that were created before this fix need to be backfilled.

-- Change column default so new rows inserted at DB level are also public
ALTER TABLE overlays ALTER COLUMN is_public_for_viewers SET DEFAULT true;

-- Backfill: make all existing active overlays public for viewers
UPDATE overlays
SET is_public_for_viewers = true, updated_at = NOW()
WHERE is_public_for_viewers = false
  AND is_active = true;
