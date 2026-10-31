-- Migration: 116_chat_timers
-- Description: Streamer-defined chat timers (ADR-0081). A timer posts a rendered
--   message on the owner's own channels when its trigger is due: an interval
--   elapsed since it last fired, and/or enough chat lines since then (both set =
--   every X minutes, but only if at least Y lines arrived). engagement-service
--   owns the table; tokens and sends live in auth-service.
--
-- Idempotent throughout: the migration runner re-applies every up-migration on
--   each pod start (CREATE ... IF NOT EXISTS, no data backfill needed).

BEGIN;

CREATE TABLE IF NOT EXISTS chat_timers (
    id                UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id           UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name              VARCHAR(50) NOT NULL CHECK (btrim(name) <> ''),
    message           TEXT        NOT NULL CHECK (btrim(message) <> '' AND length(message) <= 2000),
    interval_minutes  INT         NULL CHECK (interval_minutes BETWEEN 5 AND 1440),
    min_messages      INT         NULL CHECK (min_messages BETWEEN 1 AND 10000),
    enabled           BOOLEAN     NOT NULL DEFAULT TRUE,
    last_fired_at     TIMESTAMPTZ NULL,           -- also set to now() on create/enable (start of the first period)
    lines_at_last_fire BIGINT     NOT NULL DEFAULT 0,
    fire_count        BIGINT      NOT NULL DEFAULT 0,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chat_timers_has_trigger CHECK (interval_minutes IS NOT NULL OR min_messages IS NOT NULL)
);

CREATE UNIQUE INDEX IF NOT EXISTS chat_timers_user_name
    ON chat_timers (user_id, LOWER(name));

COMMENT ON TABLE chat_timers IS
    'Streamer-defined chat timers (ADR-0081), owned by engagement-service. Send goes through auth-service /internal/chat/bot-send.';

COMMIT;
