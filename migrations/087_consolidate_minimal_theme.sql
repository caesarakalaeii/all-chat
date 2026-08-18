-- Migration: 087_consolidate_minimal_theme
-- Description: Point overlays on the retired `minimal-theme-fixed` id at the consolidated
--   `minimal-theme` (ADR-0053).
--
-- `minimal-theme-fixed` was a bugfix fork of `minimal-theme` that shipped ALONGSIDE the theme it
-- was meant to replace. Both stayed in the picker, both accumulated their own drift, and between
-- them they held the two largest theme populations on the platform. They are now one theme.
--
-- Theme CSS is resolved from the frontend bundle by this id at render time (not copied into the
-- row), which is what lets a theme fix reach every overlay on deploy — and also what makes a
-- dangling id fatal: an overlay whose theme_id matches no bundled theme renders with NO theme CSS
-- at all. So the id is rewritten here rather than left to rot.
--
-- The frontend additionally aliases the old id to the new one (bundled-themes.ts THEME_ALIASES).
-- That alias is deliberately kept AFTER this migration rather than replaced by it: it covers rows
-- written by a client that still holds the old id between deploy and this migration, and it makes
-- the rename safe to roll back.
--
-- Idempotent: the runner re-applies every migration on each pod start, and the WHERE clause
-- simply matches nothing on the second run.
--
-- Only the theme id changes. custom_css is untouched — a user's own overrides are their data, and
-- the two themes share the same class hooks, so overrides keep applying.

BEGIN;

UPDATE overlay_configs
SET    theme_id   = 'minimal-theme',
       updated_at = NOW()
WHERE  theme_id   = 'minimal-theme-fixed';

COMMIT;
