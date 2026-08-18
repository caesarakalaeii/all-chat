-- Rollback: 087_consolidate_minimal_theme
--
-- Intentionally a no-op.
--
-- The forward migration merges two populations into one id. Nothing records which overlays were
-- on `minimal-theme-fixed` beforehand, so a rollback cannot tell them apart from the overlays that
-- were already on `minimal-theme` — reversing it would have to move BOTH groups onto the retired
-- id, which is strictly worse than leaving them on the consolidated one.
--
-- Rolling back is safe without any statement here: `minimal-theme` is a real bundled theme, so
-- these overlays keep rendering on an older frontend too. The reverse direction is covered instead
-- by THEME_ALIASES in bundled-themes.ts, which keeps the retired id resolving.

BEGIN;

-- No statements: see above.

COMMIT;
