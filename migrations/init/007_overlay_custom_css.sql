-- Add custom CSS storage for overlays
ALTER TABLE overlay_configs
    ADD COLUMN IF NOT EXISTS custom_css TEXT DEFAULT '';

UPDATE overlay_configs
SET custom_css = ''
WHERE custom_css IS NULL;

ALTER TABLE overlay_configs
    ALTER COLUMN custom_css SET NOT NULL;
