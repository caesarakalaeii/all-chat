-- Migration: Enforce at most one public overlay per user at the database level
-- Created: 2026-04-08
-- Purpose: The "only one is_public_for_viewers=true overlay per user" invariant was
--          previously enforced only in application code (UnsetAllPublicForUser).
--          A partial unique index makes the constraint atomic and prevents drift from
--          direct SQL, migration backfills, or future application bugs.

-- First ensure there are no violations left over from prior bugs / migration 046.
-- Keep the most-recently-updated public overlay per user and unset the rest.
WITH ranked AS (
    SELECT id,
           ROW_NUMBER() OVER (PARTITION BY user_id ORDER BY updated_at DESC) AS rn
    FROM overlays
    WHERE is_public_for_viewers = true
)
UPDATE overlays
SET is_public_for_viewers = false, updated_at = NOW()
WHERE id IN (
    SELECT id FROM ranked WHERE rn > 1
);

-- Create partial unique index: at most one row per user_id may have is_public_for_viewers = true.
-- A partial index on a boolean column achieves a per-user uniqueness constraint efficiently.
CREATE UNIQUE INDEX IF NOT EXISTS idx_overlays_one_public_per_user
    ON overlays (user_id)
    WHERE is_public_for_viewers = true;
