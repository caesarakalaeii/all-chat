-- Migration: 017_event_settings_down
-- Description: Rollback overlay_event_settings table

-- Drop trigger
DROP TRIGGER IF EXISTS trigger_create_overlay_event_settings ON overlays;

-- Drop function
DROP FUNCTION IF EXISTS create_overlay_event_settings();

-- Drop index
DROP INDEX IF EXISTS idx_overlay_event_settings_overlay_id;

-- Drop table
DROP TABLE IF EXISTS overlay_event_settings;
