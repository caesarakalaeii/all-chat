-- All-Chat Migration 117: Altstream platform support
-- Migration: 117
--
-- Registers the 'altstream' platform (ADR-0082) so overlay_chat_sources rows can
-- reference it (platform has a FOREIGN KEY to supported_platforms.platform).
-- Altstream needs no OAuth: the listener reads the public, documented SSE chat
-- API (altstream.online/documentation), and the channel is the lowercased
-- Altstream username.
--
-- Also seeds the platform_altstream feature gate (ADR-0008) as early access
-- (early_access=TRUE, is_premium=FALSE, ADR-0020): beta testers and ambassadors
-- only, until an admin turns early_access off in /admin/features.
--
-- IDEMPOTENCY: every service runs the full migration set on each pod restart
-- (the runner does not track applied migrations), so this script must be safe
-- to re-execute, hence ON CONFLICT DO NOTHING (the row's flags are owned
-- by the admin toggle thereafter and must not be reset by a re-run).

BEGIN;

INSERT INTO supported_platforms (platform, display_name, is_enabled, requires_oauth, config_schema)
VALUES ('altstream', 'Altstream', true, false, '{"api_type": "public_sse", "requires_oauth": false}')
ON CONFLICT (platform) DO NOTHING;

INSERT INTO feature_gates (feature_key, is_premium, early_access, description)
VALUES (
    'platform_altstream',
    FALSE,
    TRUE,
    'Altstream chat sources via the public SSE chat API (ADR-0082)'
)
ON CONFLICT (feature_key) DO NOTHING;

COMMIT;
