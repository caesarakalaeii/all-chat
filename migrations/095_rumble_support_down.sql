-- All-Chat Migration 095 Down: Remove Rumble platform support (ADR-0061)
--
-- Removes the feature gate seed and the supported_platforms row. Existing
-- overlay_chat_sources rows referencing 'rumble' would violate the FK, so
-- they are removed first — this is a destructive down, only used to roll
-- the platform out entirely.

BEGIN;

DELETE FROM overlay_chat_sources WHERE platform = 'rumble';
DELETE FROM feature_gates WHERE feature_key = 'platform_rumble';
DELETE FROM supported_platforms WHERE platform = 'rumble';

COMMIT;
