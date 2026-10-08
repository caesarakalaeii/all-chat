-- All-Chat Migration 099: Drop Rumble platform support
--
-- Rumble support (ADR-0061, migration 095) is withdrawn and its listener is
-- removed. Migration 095 is deleted from the tree, so fresh databases never
-- get the rows; this migration removes them from databases that already ran
-- 095.
--
-- overlay_chat_sources.platform references supported_platforms(platform), so
-- the sources go first. Deleting them is intended: with no listener and no
-- normalizer, a Rumble source can never deliver a message again.
--
-- IDEMPOTENCY: the runner re-executes every migration on each pod restart;
-- plain DELETEs are no-ops once the rows are gone.

BEGIN;

DELETE FROM overlay_chat_sources WHERE platform = 'rumble';
DELETE FROM feature_gates WHERE feature_key = 'platform_rumble';
DELETE FROM supported_platforms WHERE platform = 'rumble';

COMMIT;
