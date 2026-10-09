-- Migration: 099_media_objects
-- Description: Registry of user-uploaded alert media for the event-alerts
--   platform (issue #949). The bytes live in MinIO (bucket allchat-media,
--   single-node instance from issue #948); this table is only the index.
--
-- One row per issued presigned upload. The row is written at PRESIGN time,
--   before the client performs the PUT — single-node MinIO has no S3 event
--   notification wired up, so upload completion cannot be confirmed by the
--   service. Rows whose object never arrived are harmless orphans: GET /media
--   lists this registry, never the bucket, and an orphan holds no bytes.
--
-- object_key is {user_id}/{uuid}/{filename} (randomized middle segment per
--   ADR-0064's key scheme): the uuid makes keys unguessable so the
--   public-read bucket can serve media directly, and the user_id prefix
--   scopes ownership checks on delete. UNIQUE because a presign attempt
--   never reuses a key; a collision would mean a UUID collision.
--
-- Idempotent throughout (CREATE ... IF NOT EXISTS): scripts/run-migrations.sh
--   re-applies every migration on each pod start, so a non-idempotent
--   statement would crash-loop fresh pods.

BEGIN;

CREATE TABLE IF NOT EXISTS media_objects (
    id           UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID         NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    object_key   TEXT         NOT NULL UNIQUE,
    filename     TEXT         NOT NULL,
    content_type TEXT         NOT NULL,
    size_bytes   BIGINT       NOT NULL,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_media_objects_user_id
    ON media_objects (user_id);

COMMENT ON TABLE media_objects IS
    'Index of user-uploaded alert media in MinIO; rows are created at presign time, so an object that was never uploaded is an expected orphan.';
COMMENT ON COLUMN media_objects.object_key IS
    'MinIO object key: {user_id}/{uuid}/{filename}. The uuid segment makes public-read URLs unguessable.';
COMMENT ON COLUMN media_objects.size_bytes IS
    'Client-declared size from the presign request, enforced against the 10 MiB per-object limit at presign time.';

COMMIT;
