-- All-Chat Migration 050: Add encryption metadata for Kick OAuth tokens
-- Migration: 050
-- Phase 14 (Secret Rotation Infrastructure) - D-16 schema half.
--
-- Existing rows have encryption_version=0 (plaintext); new writes from Plan 14-05
-- onward set encryption_version=1 with the versioned [kid||nonce||ct||tag] format
-- from shared/encryption.MultiKeyEncryptor.
--
-- Read paths (kick-listener/channels/manager.go, overlay-manager/handlers/sources.go)
-- must gate decryption on encryption_version >= 1 (Plan 14-05).
-- Sweeper (Plan 14-06 key-rotator) uses this column to drive re-encryption sweeps.

BEGIN;

ALTER TABLE kick_oauth_tokens
    ADD COLUMN IF NOT EXISTS encryption_version SMALLINT NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_kick_oauth_tokens_enc_version
    ON kick_oauth_tokens(encryption_version);

COMMENT ON COLUMN kick_oauth_tokens.encryption_version IS
    '0 = legacy plaintext access_token/refresh_token (pre-Phase-14); 1+ = versioned ciphertext per shared/encryption.MultiKeyEncryptor wire format [kid(1B)||nonce(12B)||ct||tag(16B)]';

COMMIT;
