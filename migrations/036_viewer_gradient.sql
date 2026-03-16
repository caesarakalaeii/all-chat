-- 036_viewer_gradient.sql
-- Phase 29: Add gradient support and is_premium flag to viewers

ALTER TABLE viewers
    ADD COLUMN IF NOT EXISTS is_premium BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE viewer_cosmetics
    ADD COLUMN IF NOT EXISTS name_gradient JSONB;
