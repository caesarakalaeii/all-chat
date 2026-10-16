-- Migration: 107_alert_profiles
-- Description: Per-overlay alert profiles for alerts overlays (issue #952,
--   ADR-0070): how one event type (subscription, bits, super_chat, ...) is
--   presented — a text template with {username}/{amount}/{count}/{message}
--   placeholders, optional sound and image URLs, duration, volume, on/off.
--
-- An overlay with no rows uses the built-in defaults in shared/alerts, so a
--   fresh overlay needs no backfill and no seed rows. Allowed event_type values
--   are validated in Go (shared/alerts.IsKnownEventType), not by a CHECK
--   constraint, so adding an event type is a code change, not a migration.
--
-- Also seeds the alerts_editor feature gate (ADR-0008): saving profiles is
--   premium; reading them and the test-alert button are free.
--
-- Idempotent throughout: the migration runner re-applies every migration on
--   each pod start (CREATE ... IF NOT EXISTS, ON CONFLICT DO NOTHING).

BEGIN;

CREATE TABLE IF NOT EXISTS alert_profiles (
    id            UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    overlay_id    UUID         NOT NULL REFERENCES overlays(id) ON DELETE CASCADE,
    event_type    VARCHAR(40)  NOT NULL,
    text_template TEXT         NOT NULL DEFAULT '',
    sound_url     TEXT         NOT NULL DEFAULT '',
    image_url     TEXT         NOT NULL DEFAULT '',
    duration_ms   INT          NOT NULL DEFAULT 5000,
    volume        INT          NOT NULL DEFAULT 100,
    enabled       BOOLEAN      NOT NULL DEFAULT TRUE,
    sort_after    INT          NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT alert_profiles_overlay_event_type_key UNIQUE (overlay_id, event_type)
);

COMMENT ON TABLE alert_profiles IS
    'Saved alert profiles of an alerts overlay, one per event type (ADR-0070). Missing rows fall back to the defaults in shared/alerts.';

INSERT INTO feature_gates (feature_key, is_premium, early_access, description)
VALUES (
    'alerts_editor',
    TRUE,
    FALSE,
    'Save custom alert profiles (text, sound, image, duration) on an alerts overlay (ADR-0070). Defaults and test alerts stay free'
)
ON CONFLICT (feature_key) DO NOTHING;

COMMIT;
