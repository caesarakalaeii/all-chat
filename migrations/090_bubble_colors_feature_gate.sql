-- All-Chat Migration 090: Bubble colors feature gate
-- Migration: 090
--
-- Registers the 'bubble_colors' feature gate (ADR-0008) for differently-coloured
-- chat bubbles: a fill per platform, and/or a palette cycled down the feed.
--
-- Seeded is_premium=FALSE — this ships OPEN, unlike every other gate in this
-- table. Each of those sits on a real marginal cost or risk (tts = audio delivery
-- bandwidth, moderation = YouTube quota and platform write access, engagement =
-- send quota, sharing = abuse prevention). Bubble colours are pure client-side
-- CSS: nothing is served, metered or written on anyone's behalf, and telling
-- Twitch from YouTube at a glance serves the multistream promise the product
-- exists for. The gate is registered anyway so it can be turned into an upsell
-- from the feature-gate admin endpoint without a redeploy, which is what
-- CLAUDE.md "Shipping a Feature" asks for.
--
-- overlay-manager resolves this key on GET /overlays/:id/config (as
-- `bubble_colors_locked`) so the editor locks the controls the moment it is
-- flipped, and re-checks it on PUT, carrying over the stored values instead of
-- accepting new ones. Settings saved while the gate was open keep rendering: the
-- public overlay-config read path is deliberately not gated, so a flip stops new
-- configuration rather than retroactively restyling live overlays.
--
-- IDEMPOTENCY: every service runs the full migration set on each pod restart (the
-- runner does not track applied migrations), so this script must be safe to
-- re-execute — hence ON CONFLICT DO NOTHING (the row's is_premium is owned by the
-- admin toggle thereafter and must not be reset by a re-run).

BEGIN;

INSERT INTO feature_gates (feature_key, is_premium, description)
VALUES (
    'bubble_colors',
    FALSE,
    'Differently-coloured chat bubbles — a fill per platform and/or a palette cycled down the feed'
)
ON CONFLICT (feature_key) DO NOTHING;

COMMIT;
