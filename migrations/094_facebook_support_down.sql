-- Migration 094 (down): remove Facebook platform support
--
-- Reverses 094_facebook_support.sql. Safe to run if the rows/table are absent.
--
-- The table carries credentials, so the down migration DELETES it (not a
-- rename): a rolled-back deployment must not keep page tokens it can no
-- longer use or govern.

BEGIN;

DELETE FROM feature_gates WHERE feature_key = 'platform_facebook';

DROP TABLE IF EXISTS facebook_oauth_tokens;

DELETE FROM supported_platforms WHERE platform = 'facebook';

COMMIT;
