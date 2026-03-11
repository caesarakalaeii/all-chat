-- All-Chat Migration 006: Add encryption metadata for YouTube tokens
BEGIN;

ALTER TABLE youtube_oauth_tokens
    ADD COLUMN IF NOT EXISTS encryption_version SMALLINT NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_youtube_oauth_tokens_encryption_version
    ON youtube_oauth_tokens(encryption_version);

COMMIT;
