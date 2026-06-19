-- All-Chat Migration 061: Moderation feature gate
-- Migration: 061
--
-- Registers the 'moderation' feature gate (ADR-0008) so the cross-platform chat
-- moderation write-path (ADR-0017) can roll out to a small cohort first. Seeded
-- is_premium=TRUE: only premium users may moderate initially. Flip to FALSE via the
-- feature-gate admin endpoint to graduate moderation to all users — no redeploy.
--
-- The moderation-service applies shared/middleware.RequirePremium("moderation") to
-- its write endpoints and surfaces the same decision on the capabilities endpoint so
-- the dashboard hides controls for users outside the cohort.
--
-- IDEMPOTENCY: every service runs the full migration set on each pod restart (the
-- runner does not track applied migrations), so this script must be safe to
-- re-execute — hence ON CONFLICT DO NOTHING (the row's is_premium is owned by the
-- admin toggle thereafter and must not be reset by a re-run).

BEGIN;

INSERT INTO feature_gates (feature_key, is_premium, description)
VALUES ('moderation', TRUE, 'Cross-platform chat moderation write-path (delete/timeout/ban) — ADR-0017')
ON CONFLICT (feature_key) DO NOTHING;

COMMIT;
