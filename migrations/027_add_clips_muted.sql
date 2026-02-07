-- Migration: 027_add_clips_muted
-- Description: Add clips_muted column to credit_roll_configs to allow users to control audio

-- Add clips_muted column (default true to enable autoplay in browsers)
ALTER TABLE credit_roll_configs
ADD COLUMN IF NOT EXISTS clips_muted BOOLEAN NOT NULL DEFAULT true;

-- Update existing configs to default to muted (browser autoplay requirement)
UPDATE credit_roll_configs
SET clips_muted = true
WHERE clips_muted IS NULL;
