-- All-Chat Migration 095: Rumble platform support (ADR-0061)
--
-- Registers 'rumble' in supported_platforms (the overlay_chat_sources FK
-- target) and seeds its ADR-0008 feature gate in one migration, matching
-- the pattern of migrations/039_discord_platform.sql + 061/089/090.
--
-- Rumble chat is read via Rumble's internal chat pop-up API
-- (web7.rumble.com/chat/api/chat/<id>/stream, SSE): no OAuth, so
-- requires_oauth is FALSE. Sources are the numeric chat pop-up id.
--
-- The feature gate description deliberately names ADR-0061, the decision
-- record for shipping this platform against an undocumented endpoint.
-- Seeded is_premium=TRUE = rollout cohort; flipped via the admin endpoint
-- without redeploy.
--
-- IDEMPOTENCY: every service runs the full migration set on each pod
-- restart (the runner does not track applied migrations), so this script
-- must be safe to re-execute — hence BEGIN/COMMIT and ON CONFLICT DO
-- NOTHING (the rows' mutable columns are owned by their admin toggles
-- thereafter and must not be reset by a re-run).

BEGIN;

INSERT INTO supported_platforms (platform, display_name, is_enabled, requires_oauth, config_schema)
VALUES ('rumble', 'Rumble', true, false, '{"api_type": "chat_popup_sse", "requires_channel_id": true}')
ON CONFLICT (platform) DO NOTHING;

INSERT INTO feature_gates (feature_key, is_premium, description)
VALUES (
    'platform_rumble',
    TRUE,
    'Rumble chat sources on overlays (ADR-0061: internal chat pop-up SSE endpoint)'
)
ON CONFLICT (feature_key) DO NOTHING;

COMMIT;
