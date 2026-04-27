-- All-Chat Overlay TTS Configurations — DOWN
-- Migration: 049 (rollback)
-- Reverses migration 049_overlay_tts_configs.sql.

DROP TRIGGER IF EXISTS update_overlay_tts_configs_updated_at ON overlay_tts_configs;
DROP INDEX IF EXISTS idx_overlay_tts_configs_overlay;
DROP TABLE IF EXISTS overlay_tts_configs;
DELETE FROM feature_gates WHERE feature_key = 'tts';
