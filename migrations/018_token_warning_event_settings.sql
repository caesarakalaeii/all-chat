-- Migration: 018_token_warning_event_settings
-- Description: Add token expiration warning event toggle

-- Add enable_token_warnings column to overlay_event_settings
ALTER TABLE overlay_event_settings
ADD COLUMN IF NOT EXISTS enable_token_warnings BOOLEAN DEFAULT TRUE;

-- Add comment
COMMENT ON COLUMN overlay_event_settings.enable_token_warnings
IS 'Display OAuth token expiration warnings on overlay (requires token-refresh-service)';

-- Backfill existing overlays with default value
UPDATE overlay_event_settings
SET enable_token_warnings = TRUE
WHERE enable_token_warnings IS NULL;
