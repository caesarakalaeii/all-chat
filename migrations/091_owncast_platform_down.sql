-- All-Chat Migration 091 (down): remove the Owncast platform (ADR-0058)
--
-- Reverses 091_owncast_platform.sql. Safe to run if the rows are absent.
--
-- The feature gate deletion follows the same convention as 090's down script:
-- with the row gone the gate cache reports the key as not-premium, so the
-- feature opens rather than hard-fails. overlay_chat_sources rows that
-- reference platform='owncast' would break the FK if left behind, so the
-- transaction deletes them before the platform row.

BEGIN;

DELETE FROM overlay_chat_sources WHERE platform = 'owncast';
DELETE FROM feature_gates WHERE feature_key = 'platform_owncast';
DELETE FROM supported_platforms WHERE platform = 'owncast';

COMMIT;
