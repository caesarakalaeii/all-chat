-- All-Chat Migration 091: Owncast platform support (ADR-0058)
-- Migration: 091
--
-- 1. supported_platforms row: 'owncast' is a self-hosted streaming server
--    (github.com/owncast/owncast). Its chat is read-only and unauthenticated
--    by design (POST /api/chat/register -> token -> /ws), and its "channel"
--    is an instance URL, not a channel name — the FK to supported_platforms
--    just registers the platform.
-- 2. Feature gate seed (ADR-0008): platform_owncast, is_premium=TRUE, flipped
--    via the admin endpoint without a redeploy. Enforcement is wired
--    centrally by the coordinator after merge; this migration only seeds.
--
-- IDEMPOTENCY: every service runs the full migration set on each pod restart
-- (the runner does not track applied migrations), so this script must be safe
-- to re-execute — ON CONFLICT DO NOTHING; a re-run never resets is_premium,
-- which the admin toggle owns thereafter.

BEGIN;

INSERT INTO supported_platforms (platform, display_name, is_enabled, requires_oauth, config_schema)
VALUES (
    'owncast',
    'Owncast',
    TRUE,
    FALSE,
    '{"transport": "websocket", "register_endpoint": "/api/chat/register", "interface": "/ws"}'::jsonb
)
ON CONFLICT (platform) DO NOTHING;

INSERT INTO feature_gates (feature_key, is_premium, description)
VALUES (
    'platform_owncast',
    TRUE,
    'Attach a self-hosted Owncast instance (by URL) as a chat source (ADR-0058)'
)
ON CONFLICT (feature_key) DO NOTHING;

COMMIT;
