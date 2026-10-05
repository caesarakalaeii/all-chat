-- Migration: 101_alert_events
-- Description: Event log for the alert-processor (issue #951, ADR-0064). One
--   row per normalized platform event delivered to one alert-capable overlay
--   (overlay_type IN ('alerts','goal','list')). A single raw stream message
--   routed to N overlays produces N rows.
--
-- id is NOT generated here: the alert-processor derives a deterministic UUIDv5
--   from (platform, channel, message id, overlay id) and supplies it, so a
--   re-delivered stream entry (at-least-once consumer group semantics) inserts
--   the same id again and ON CONFLICT DO NOTHING absorbs the duplicate. This is
--   what makes the at-least-once pipeline safe to re-run a half-finished batch.
--
-- overlay_id has no FK to overlays on purpose: this is an append-only history,
--   and a deleted overlay's past alerts must remain queryable. user_id is the
--   platform-native id (TEXT, not users.id UUID) because most event senders
--   never resolve to an All-Chat account.
--
-- Idempotent throughout (CREATE ... IF NOT EXISTS): scripts/run-migrations.sh
--   re-applies every migration on each pod start, so a non-idempotent
--   statement would crash-loop fresh pods.

BEGIN;

CREATE TABLE IF NOT EXISTS alert_events (
    id          UUID         PRIMARY KEY,
    overlay_id  UUID         NOT NULL,
    platform    VARCHAR(32)  NOT NULL,
    event_type  VARCHAR(64)  NOT NULL,
    user_id     TEXT         NOT NULL DEFAULT '',
    user_name   TEXT         NOT NULL DEFAULT '',
    event_data  JSONB        NOT NULL DEFAULT '{}'::jsonb,
    occurred_at TIMESTAMPTZ  NOT NULL,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_alert_events_overlay_occurred
    ON alert_events (overlay_id, occurred_at DESC);

COMMENT ON TABLE alert_events IS
    'Normalized platform events delivered to alert-capable overlays by the alert-processor (issue #951).';
COMMENT ON COLUMN alert_events.id IS
    'Deterministic UUIDv5 of (platform, channel, message id, overlay id): stable across at-least-once re-deliveries so ON CONFLICT DO NOTHING dedupes.';
COMMENT ON COLUMN alert_events.overlay_id IS
    'Target overlay. Deliberately not a foreign key: alert history must survive overlay deletion.';
COMMENT ON COLUMN alert_events.user_id IS
    'Platform-native sender id; not users.id — most event senders have no All-Chat account.';
COMMENT ON COLUMN alert_events.event_data IS
    'Normalized EventInfo JSON (tier, value, duration, event-specific metadata) as produced by the message-processor normalizers.';

COMMIT;