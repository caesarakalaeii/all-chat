-- Rollback migration: Remove public overlay flag
-- Created: 2025-12-20

-- Drop index
DROP INDEX IF EXISTS idx_overlays_user_public;

-- Drop column
ALTER TABLE overlays DROP COLUMN IF EXISTS is_public_for_viewers;
