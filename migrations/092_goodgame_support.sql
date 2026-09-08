-- All-Chat Migration 092: GoodGame platform support
--
-- Registers goodgame (goodgame.ru, CIS streaming platform) in
-- supported_platforms so overlay_chat_sources rows can reference it (FK to
-- supported_platforms.platform), and seeds the platform feature gate
-- (ADR-0008): is_premium=TRUE is the rollout cohort; flip via the feature-gate
-- admin endpoint to graduate to all users — no redeploy.
--
-- No OAuth: GoodGame chat reads are guest-accessible on the public WebSocket
-- (wss://chat.goodgame.ru/chat/websocket); goodgame-listener resolves the
-- channel slug to the numeric chat id via GET /api/4/stream/<key>.
--
-- IDEMPOTENCY: the runner replays every migration on each pod restart, so all
-- statements are ON CONFLICT DO NOTHING (the feature gate's is_premium is
-- owned by the admin toggle thereafter and must not be reset by a re-run).

BEGIN;

INSERT INTO supported_platforms (platform, display_name, is_enabled, requires_oauth, config_schema)
VALUES ('goodgame', 'GoodGame', true, false, '{"api_type": "websocket", "requires_bot": false, "chat_endpoint": "wss://chat.goodgame.ru/chat/websocket"}')
ON CONFLICT (platform) DO NOTHING;

INSERT INTO feature_gates (feature_key, is_premium, description)
VALUES ('platform_goodgame', TRUE, 'GoodGame.ru chat source on overlays — ADR-0008 rollout gate')
ON CONFLICT (feature_key) DO NOTHING;

COMMIT;
