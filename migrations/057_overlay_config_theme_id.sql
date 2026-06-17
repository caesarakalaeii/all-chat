-- All-Chat Migration 057: Reference applied theme by id instead of copying its CSS
-- Migration: 057
--
-- Until now, applying a marketplace theme copied the theme's full CSS into
-- overlay_configs.custom_css. That froze a private snapshot per overlay, so an
-- upstream theme fix never reached existing overlays without a data migration
-- (see the emote max-height cap bug). Themes now ship bundled in the frontend
-- build; an overlay only needs to record WHICH theme it uses, and the renderer
-- resolves the (always current) CSS from the bundle at load time.
--
-- theme_id is the bundled theme's id (e.g. 'modern-dark-theme'); NULL means no
-- bundled theme (legacy overlays whose custom_css still holds a frozen copy,
-- until the Phase 3 backfill, or overlays using only raw custom CSS).
-- visual_settings keeps holding the user's per-overlay customization diff;
-- custom_css is reduced to genuinely custom raw overrides.
--
-- IDEMPOTENCY: the runner re-executes every migration on each pod start, so
-- this is ADD COLUMN IF NOT EXISTS and safe to re-run.

BEGIN;

ALTER TABLE overlay_configs
    ADD COLUMN IF NOT EXISTS theme_id TEXT;

COMMENT ON COLUMN overlay_configs.theme_id IS
    'Bundled marketplace theme id applied to this overlay (e.g. ''modern-dark-theme''). The renderer resolves theme CSS fresh from the frontend bundle so theme fixes propagate on deploy. NULL = no bundled theme (raw custom_css only / legacy frozen copy pending backfill).';

COMMIT;
