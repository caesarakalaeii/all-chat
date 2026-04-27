-- Down migration for 051_tiktok_token_encryption.sql
--
-- WARNING: Dropping the encryption_version column does NOT decrypt rows. Any
-- ciphertext stored in access_token/refresh_token (encryption_version >= 1)
-- will become unrecoverable because the decryption path reads
-- encryption_version to decide whether to decrypt.
--
-- For TikTok specifically: as of Phase 14, all tiktok_oauth_tokens rows
-- should have encryption_version=0 (plaintext) because the Node.js
-- tiktok-listener does not perform encryption. This down migration is
-- therefore safe to run in Phase 14 rollback scenarios.
--
-- Verify before running:
--   SELECT COUNT(*) FROM tiktok_oauth_tokens WHERE encryption_version >= 1;
-- If result > 0, a future Node.js encryption implementation has been deployed
-- — do NOT drop the column without a coordinated rollback of that code too.
--
-- This migration is gated by the Phase 14 deployment runbook (Plan 14-07).

BEGIN;

DROP INDEX IF EXISTS idx_tiktok_oauth_tokens_enc_version;
ALTER TABLE tiktok_oauth_tokens DROP COLUMN IF EXISTS encryption_version;

COMMIT;
