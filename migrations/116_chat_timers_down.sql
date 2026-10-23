-- Migration: 116_chat_timers (down)
-- Reverses 116_chat_timers.sql. Safe to run when the objects are absent.

BEGIN;

DROP TABLE IF EXISTS chat_timers;

COMMIT;
