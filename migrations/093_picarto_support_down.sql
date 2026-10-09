-- All-Chat Migration 093 (down): remove the Picarto platform
--
-- Reverses 093_picarto_support.sql. Safe to run if the rows are absent.
--
-- With the gate row gone the feature-gate cache reports platform_picarto as
-- not-registered; with the supported_platforms row gone no new picarto
-- overlay_chat_sources can be created. Existing sources are left in place —
-- dropping data on a down migration is deliberately avoided.

BEGIN;

DELETE FROM feature_gates WHERE feature_key = 'platform_picarto';

DELETE FROM supported_platforms WHERE platform = 'picarto';

COMMIT;
