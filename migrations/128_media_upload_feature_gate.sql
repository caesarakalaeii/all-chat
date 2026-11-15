-- All-Chat Migration 128: media upload feature gate
-- Migration: 128
--
-- Registers the 'media_upload' feature gate (ADR-0008) for uploading your own
-- images, GIFs and sounds (media-service POST /api/v1/media/presign, ADR-0088).
--
-- Seeded is_premium=TRUE. Alerts overlays, the alerts editor and chat commands
-- are free for everyone (their gates were opened in /admin/features on
-- 2026-11-15). Before that, uploads were premium only because the editor that
-- offers them was; this gate keeps them premium on their own. Uploads take
-- object storage, so they stay a premium perk. Pasting a URL, the built-in
-- presets and KLIPY GIF search need no upload and stay free. Listing and
-- deleting your own uploads are never gated.
--
-- IDEMPOTENCY: every service runs the full migration set on each pod restart (the
-- runner does not track applied migrations), so this script must be safe to
-- re-execute, hence ON CONFLICT DO NOTHING (the row's flags are owned by the
-- admin toggle thereafter and must not be reset by a re-run).

BEGIN;

INSERT INTO feature_gates (feature_key, is_premium, description)
VALUES (
    'media_upload',
    TRUE,
    'Uploading your own images, GIFs and sounds for alerts and chat command alerts'
)
ON CONFLICT (feature_key) DO NOTHING;

COMMIT;
