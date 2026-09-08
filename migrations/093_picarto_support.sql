-- All-Chat Migration 093: Picarto platform support
-- Migration: 093
--
-- Registers the 'picarto' platform (ADR-0059) so overlay_chat_sources rows can
-- reference it (platform has a FOREIGN KEY to supported_platforms.platform).
-- Picarto needs no OAuth: the listener reads the public pop-out chat feed with
-- an anonymous viewer token.
--
-- Also seeds the platform_picarto feature gate (ADR-0008 rollout cohort;
-- ADR-0059 records the accepted unofficial-endpoint risk). Seeded
-- is_premium=TRUE: access to a channel that rides an undocumented endpoint is
-- held back until live verification confirms the feed is stable.
--
-- IDEMPOTENCY: every service runs the full migration set on each pod restart
-- (the runner does not track applied migrations), so this script must be safe
-- to re-execute — hence ON CONFLICT DO NOTHING (the row's is_premium is owned
-- by the admin toggle thereafter and must not be reset by a re-run).

BEGIN;

INSERT INTO supported_platforms (platform, display_name, is_enabled, requires_oauth, config_schema)
VALUES ('picarto', 'Picarto', true, false, '{"api_type": "popout_websocket", "requires_oauth": false}')
ON CONFLICT (platform) DO NOTHING;

INSERT INTO feature_gates (feature_key, is_premium, description)
VALUES (
    'platform_picarto',
    TRUE,
    'Picarto chat sources via the unofficial pop-out WebSocket feed (ADR-0059)'
)
ON CONFLICT (feature_key) DO NOTHING;

COMMIT;
