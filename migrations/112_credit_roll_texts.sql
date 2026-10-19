-- Migration: 112_credit_roll_texts
-- Description: Streamer-written text and per-element text sizes for the credit
--   roll. texts maps a credit-roll message key (heading, topGifters,
--   valueBitsMany, ...) to the streamer's own template, which replaces the
--   catalog copy in every UI language; a key that is absent keeps the default.
--   font_scales maps a roll element (heading, session, entry, ...) to a size in
--   percent of its default. Both default to empty, so every existing roll looks
--   exactly as before.
--
-- Idempotent: the migration runner re-applies every migration on each pod start.

ALTER TABLE credit_roll_configs
    ADD COLUMN IF NOT EXISTS texts JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN IF NOT EXISTS font_scales JSONB NOT NULL DEFAULT '{}'::jsonb;
