-- Migration: 124_credit_roll_restart_delay
-- Description: How long the credit roll holds its end before it starts over.
--   The roll's cycle is max(pass + restart_delay_seconds,
--   display_duration_seconds), so the default 0 keeps every existing roll
--   restarting exactly as before.
--
-- Idempotent: the migration runner re-applies every migration on each pod start.

ALTER TABLE credit_roll_configs
    ADD COLUMN IF NOT EXISTS restart_delay_seconds INTEGER NOT NULL DEFAULT 0;
