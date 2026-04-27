-- All-Chat Overlay TTS Configurations
-- Migration: 049
-- Description: Per-overlay ElevenLabs API key (AES-GCM encrypted via shared/encryption)
--              + per-overlay tts_signing_secret for the OBS-URL tts_token JWT scheme.
--              Also registers the 'tts' feature_gate (is_premium=true, admin-toggleable).
-- ADR: 0008 (feature gates), Phase 13 D-03, D-06, D-08

CREATE TABLE IF NOT EXISTS overlay_tts_configs (
    id                 UUID      PRIMARY KEY DEFAULT gen_random_uuid(),
    overlay_id         UUID      NOT NULL UNIQUE REFERENCES overlays(id) ON DELETE CASCADE,
    encrypted_api_key  BYTEA     NOT NULL,
    voice_id           TEXT      NOT NULL,
    tts_signing_secret BYTEA     NOT NULL,
    created_at         TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_overlay_tts_configs_overlay ON overlay_tts_configs(overlay_id);

DROP TRIGGER IF EXISTS update_overlay_tts_configs_updated_at ON overlay_tts_configs;
CREATE TRIGGER update_overlay_tts_configs_updated_at
    BEFORE UPDATE ON overlay_tts_configs
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE overlay_tts_configs IS
    'Phase 13: per-overlay ElevenLabs credentials + JWT signing secret. Encrypted at rest via shared/encryption AES-GCM with 12-byte random nonce prefix.';
COMMENT ON COLUMN overlay_tts_configs.encrypted_api_key IS
    'base64(AES-GCM(nonce || ciphertext || authTag)) of the user-supplied ElevenLabs API key. Master key from env TOKEN_ENCRYPTION_KEY. NEVER logged.';
COMMENT ON COLUMN overlay_tts_configs.tts_signing_secret IS
    'Per-overlay 32 random bytes used as HS256 signing secret for the tts_token JWT (D-08). Rotating this invalidates all prior JWTs (D-10).';

INSERT INTO feature_gates (feature_key, is_premium, description)
VALUES ('tts', TRUE, 'Text-to-speech for chat messages')
ON CONFLICT (feature_key) DO NOTHING;
