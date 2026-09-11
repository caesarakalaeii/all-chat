-- All-Chat Migration 092 (down): remove GoodGame platform support
--
-- Reverses 092_goodgame_support.sql. Safe to run if the rows are absent.
-- overlay_chat_sources rows referencing platform='goodgame' must be removed
-- first (or the FK on supported_platforms.platform will block the delete); in
-- practice the platform is added through the API which cleans up its sources.

BEGIN;

DELETE FROM feature_gates WHERE feature_key = 'platform_goodgame';
DELETE FROM supported_platforms WHERE platform = 'goodgame';

COMMIT;
