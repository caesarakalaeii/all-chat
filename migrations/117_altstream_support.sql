-- All-Chat Migration 117: Altstream platform support
-- Migration: 117
--
-- Registers the 'altstream' platform (ADR-0082) so overlay_chat_sources rows can
-- reference it (platform has a FOREIGN KEY to supported_platforms.platform).
-- Altstream needs no OAuth: the listener reads the public, documented SSE chat
-- API (altstream.online/documentation), and the channel is the lowercased
-- Altstream username.
--
-- Also seeds the platform_altstream feature gate (ADR-0008 rollout cohort).
-- Seeded is_premium=TRUE: a new listener reaches a premium cohort first; flip it
-- via the feature-gate admin endpoint to open the platform with no redeploy.
--
-- IDEMPOTENCY: every service runs the full migration set on each pod restart
-- (the runner does not track applied migrations), so this script must be safe
-- to re-execute, hence ON CONFLICT DO NOTHING (the row's is_premium is owned
-- by the admin toggle thereafter and must not be reset by a re-run).

BEGIN;

INSERT INTO supported_platforms (platform, display_name, is_enabled, requires_oauth, config_schema)
VALUES ('altstream', 'Altstream', true, false, '{"api_type": "public_sse", "requires_oauth": false}')
ON CONFLICT (platform) DO NOTHING;

INSERT INTO feature_gates (feature_key, is_premium, description)
VALUES (
    'platform_altstream',
    TRUE,
    'Altstream chat sources via the public SSE chat API (ADR-0082)'
)
ON CONFLICT (feature_key) DO NOTHING;

COMMIT;
