-- All-Chat Migration 127: Twitch ad break notices feature gate
-- Migration: 127
--
-- Registers the 'twitch_ad_breaks' feature gate (ADR-0008) for the monitor's
-- Twitch ad break notice: a row in Activity & Events when an ad break starts,
-- whether Ads Manager scheduled it or someone ran it by hand.
--
-- Seeded is_premium=FALSE, like bubble_colors (migration 090): the feature ships
-- OPEN. It reads one EventSub subscription per channel and spends no quota, send
-- budget or bandwidth on anyone's behalf. The gate is registered anyway so it can
-- be narrowed from the feature-gate admin endpoint without a redeploy, which is
-- what CLAUDE.md "Shipping a Feature" asks for.
--
-- auth-service enforces it on the opt-in consent
-- (GET /api/v1/auth/twitch/moderation/:overlay_id?actions=ads). A flip therefore
-- stops new grants; channels that already granted channel:read:ads keep their
-- notices.
--
-- IDEMPOTENCY: every service runs the full migration set on each pod restart (the
-- runner does not track applied migrations), so this script must be safe to
-- re-execute — hence ON CONFLICT DO NOTHING (the row's flags are owned by the
-- admin toggle thereafter and must not be reset by a re-run).

BEGIN;

INSERT INTO feature_gates (feature_key, is_premium, description)
VALUES (
    'twitch_ad_breaks',
    FALSE,
    'Twitch ad break notices in the monitor''s Activity & Events panel'
)
ON CONFLICT (feature_key) DO NOTHING;

COMMIT;
