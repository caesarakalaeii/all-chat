-- All-Chat Migration 117 (down): remove the Altstream platform
--
-- Reverses 117_altstream_support.sql. Safe to run if the rows are absent.
--
-- With the gate row gone the feature-gate cache reports platform_altstream as
-- not-registered; with the supported_platforms row gone no new altstream
-- overlay_chat_sources can be created. Existing sources are left in place:
-- dropping data on a down migration is deliberately avoided.

BEGIN;

DELETE FROM feature_gates WHERE feature_key = 'platform_altstream';

DELETE FROM supported_platforms WHERE platform = 'altstream';

COMMIT;
