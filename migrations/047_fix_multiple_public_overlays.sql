-- Migration: Fix multiple public overlays per user
-- Created: 2026-04-01
-- Purpose: Migration 046 backfilled ALL active overlays to is_public_for_viewers=true,
--          but only one overlay per user should be public. Keep the most recently
--          updated overlay as public and unset the rest.

WITH ranked AS (
    SELECT id, user_id,
           ROW_NUMBER() OVER (PARTITION BY user_id ORDER BY updated_at DESC) AS rn
    FROM overlays
    WHERE is_public_for_viewers = true
)
UPDATE overlays
SET is_public_for_viewers = false, updated_at = NOW()
WHERE id IN (
    SELECT id FROM ranked WHERE rn > 1
);
