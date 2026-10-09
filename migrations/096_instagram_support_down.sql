-- Migration 096 (down): remove Instagram platform support
--
-- Reverses 096_instagram_support.sql. Safe to run if the rows/table are absent.
--
-- The table carries credentials, so the down migration DELETES it (not a
-- rename): a rolled-back deployment must not keep IG tokens it can no
-- longer use or govern.

BEGIN;

DELETE FROM feature_gates WHERE feature_key = 'platform_instagram';

DROP TABLE IF EXISTS instagram_oauth_tokens;

DELETE FROM supported_platforms WHERE platform = 'instagram';

COMMIT;
