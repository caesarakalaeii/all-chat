-- All-Chat Migration 089: desktop control surfaces feature gate
--
-- Registers the 'desktop_control_surfaces' feature gate (ADR-0008 shape, ADR-0049 release
-- requirement 1) so device pairing for desktop control surfaces can be turned off, or turned
-- premium-only, without a redeploy. auth-service applies
-- shared/middleware.RequirePremium("desktop_control_surfaces") to
-- POST /api/v1/auth/me/devices/approve ONLY — the dashboard's Approve button. Gating the
-- pairing step rather than each action keeps enforcement in one place and leaves the existing
-- per-action gates (GateEngagement on starting a poll/prediction) exactly as they are.
--
-- SEEDED is_premium = FALSE, on purpose, and this is the one deliberate deviation worth
-- explaining. Three shipped documents — docs/guides/streamdeck.md,
-- streamdeck-plugin/README.md and streamcontroller-plugin/README.md — all state that both
-- plugins are free and that only *starting* a poll or prediction is premium. Seeding this gate
-- premium would silently make the good install flow premium and contradict all three. Seeding
-- it free satisfies the release requirement (the toggle exists and can be flipped from the
-- feature-gate admin endpoint without a deploy) while leaving the product as advertised.
--
-- The pasted-token path (personal access tokens, ADR-0051) is not gated at all and is not
-- affected by this row: it predates the gate and remains the credential for a headless box.
--
-- IDEMPOTENCY: the runner replays every migration on each pod restart and does not track
-- applied migrations, so this must be safe to re-run — hence ON CONFLICT DO NOTHING. The
-- row's is_premium value is owned by the admin toggle thereafter and must NOT be reset by a
-- re-run; that is precisely what DO NOTHING (rather than DO UPDATE) guarantees.

BEGIN;

INSERT INTO feature_gates (feature_key, is_premium, description)
VALUES (
    'desktop_control_surfaces',
    FALSE,
    'Pair a desktop control surface (Stream Deck / StreamController) with a device token — ADR-0049, issue #737'
)
ON CONFLICT (feature_key) DO NOTHING;

COMMIT;
