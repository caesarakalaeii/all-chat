-- Migration: 099_media_objects (down)
-- Description: Remove the alert-media registry table. Objects in MinIO are
--   NOT removed here — they are unreachable through the service once the
--   registry is gone and can be dropped with the bucket itself (issue #948).

BEGIN;

DROP INDEX IF EXISTS idx_media_objects_user_id;
DROP TABLE IF EXISTS media_objects;

COMMIT;
