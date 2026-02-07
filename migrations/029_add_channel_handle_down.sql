-- Remove channel_handle field
DROP INDEX IF EXISTS idx_overlay_chat_sources_channel_handle;
ALTER TABLE overlay_chat_sources DROP COLUMN channel_handle;
