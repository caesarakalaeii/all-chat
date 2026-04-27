-- All-Chat Migration 051: Add encryption metadata for TikTok OAuth tokens
-- Migration: 051
-- Phase 14 (Secret Rotation Infrastructure) - D-16 schema half.
--
-- IMPORTANT: The tiktok-listener service is Node.js (services/tiktok-listener/),
-- not Go. The encryption code-side change for D-17 (encrypt-on-write,
-- decrypt-on-read) is deferred to a follow-up phase that adds a Node.js
-- equivalent of shared/encryption.MultiKeyEncryptor (or routes token I/O
-- through a Go gateway). This migration ships the schema column NOW so the
-- sweeper (services/auth-service/cmd/key-rotator) and any future writer can
-- distinguish v0 plaintext from v1+ ciphertext rows.
--
-- NOTE FOR PLAN 14-06 SWEEPER: tiktok_oauth_tokens scan MUST SKIP rows with
-- encryption_version=0 for now — the Node.js listener has not migrated to
-- versioned encryption. Only sweep encryption_version >= 1 rows for
-- re-encryption to the current kid (forward-compatible, no Node.js changes
-- required in Phase 14).
--
-- No Go service currently reads tiktok_oauth_tokens at runtime (verified in
-- Phase 14 research §8). The column is added defensively for schema completeness
-- and sweeper forward-compatibility.

BEGIN;

ALTER TABLE tiktok_oauth_tokens
    ADD COLUMN IF NOT EXISTS encryption_version SMALLINT NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_tiktok_oauth_tokens_enc_version
    ON tiktok_oauth_tokens(encryption_version);

COMMENT ON COLUMN tiktok_oauth_tokens.encryption_version IS
    '0 = plaintext (current state — Node.js tiktok-listener has not migrated to versioned encryption; Phase 14 sweeper skips these rows); 1+ = versioned ciphertext per shared/encryption.MultiKeyEncryptor wire format [kid(1B)||nonce(12B)||ct||tag(16B)]. See Phase 14 plan 14-03.';

COMMIT;
