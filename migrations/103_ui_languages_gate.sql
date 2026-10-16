-- All-Chat Migration 103: UI languages feature gate
-- Migration: 103
--
-- Registers the 'ui_languages' feature gate (ADR-0008) for choosing a
-- non-English UI language in Settings (ADR-0066). German is the first.
--
-- Seeded early_access=TRUE, is_premium=FALSE. The German catalog starts as a
-- machine translation, so beta testers and ambassadors get it first and can
-- report or fix wording through the /translate tool (ADR-0063). Flipping
-- early_access to FALSE from the feature-gate admin endpoint opens it to every
-- signed-in user with no deploy. is_premium stays FALSE: a language is not an
-- upsell, and the dimension exists only so it could be made one without a
-- redeploy, which is what CLAUDE.md "Shipping a Feature" asks for.
--
-- share-service enforces both dimensions on GET /api/v1/ui-languages, which the
-- Settings language card probes. The chosen language is stored in the browser,
-- not here: the gate decides who may pick one, and a later flip does not
-- switch anybody's UI back.
--
-- IDEMPOTENCY: every service runs the full migration set on each pod restart (the
-- runner does not track applied migrations), so this script must be safe to
-- re-execute — hence ON CONFLICT DO NOTHING (the row is owned by the admin toggle
-- thereafter and must not be reset by a re-run).

BEGIN;

INSERT INTO feature_gates (feature_key, is_premium, early_access, description)
VALUES (
    'ui_languages',
    FALSE,
    TRUE,
    'Choose a non-English UI language in Settings (ADR-0066). Beta testers first while the catalogs are machine-translated'
)
ON CONFLICT (feature_key) DO NOTHING;

COMMIT;
