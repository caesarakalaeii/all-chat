-- Add visual_settings JSONB column to overlay_configs
-- Stores structured CSS property values for the visual-customizer cascade layer

ALTER TABLE overlay_configs
  ADD COLUMN IF NOT EXISTS visual_settings JSONB NOT NULL DEFAULT '{}'::jsonb;
