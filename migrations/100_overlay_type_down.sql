-- Migration: 100_overlay_type (down)
-- Description: Remove overlay_type from overlays. The CHECK constraint is not
--   dropped by name because it is dropped with the column it belongs to.

ALTER TABLE overlays
    DROP COLUMN IF EXISTS overlay_type;
