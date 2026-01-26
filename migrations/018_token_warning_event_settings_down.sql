-- Rollback: 018_token_warning_event_settings
-- Description: Remove token expiration warning event toggle

-- Remove enable_token_warnings column
ALTER TABLE overlay_event_settings
DROP COLUMN IF EXISTS enable_token_warnings;
