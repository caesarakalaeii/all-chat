-- Migration: 108_alerts_overlay_gate
-- Description: Seeds the alerts_overlay feature gate (ADR-0008): creating an
--   alerts overlay (issue #953) is premium. overlay-manager enforces it on
--   POST /overlays and on clone; existing alerts overlays keep working.
--
-- Idempotent: the migration runner re-applies every migration on each pod
--   start (ON CONFLICT DO NOTHING keeps an admin's later flip).

BEGIN;

INSERT INTO feature_gates (feature_key, is_premium, early_access, description)
VALUES (
    'alerts_overlay',
    TRUE,
    FALSE,
    'Create alert overlays (alert box for subs, bits, raids, Super Chats, gifts). Existing alert overlays keep working'
)
ON CONFLICT (feature_key) DO NOTHING;

COMMIT;
