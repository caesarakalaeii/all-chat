-- Down migration for 050_kick_token_encryption.sql
--
-- WARNING: Dropping the encryption_version column does NOT decrypt rows. Any
-- ciphertext stored in access_token/refresh_token (encryption_version >= 1)
-- will become unrecoverable because the decryption path (kick-listener,
-- overlay-manager) reads encryption_version to decide whether to decrypt.
--
-- Run this down migration ONLY if Phase 14 is being fully reverted AND no
-- kick_oauth_tokens rows have been encrypted (i.e., all rows still have
-- encryption_version=0 / plaintext tokens). Verify with:
--   SELECT COUNT(*) FROM kick_oauth_tokens WHERE encryption_version >= 1;
-- If result > 0, do NOT run this migration — plaintext access will be
-- permanently lost for those rows.
--
-- This migration is gated by the Phase 14 deployment runbook (Plan 14-07).

BEGIN;

DROP INDEX IF EXISTS idx_kick_oauth_tokens_enc_version;
ALTER TABLE kick_oauth_tokens DROP COLUMN IF EXISTS encryption_version;

COMMIT;
