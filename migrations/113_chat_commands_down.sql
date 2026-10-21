-- Migration: 113_chat_commands (down)
-- Reverses 113_chat_commands.sql. Safe to run when the objects are absent.

BEGIN;

DROP TABLE IF EXISTS chat_commands;
DELETE FROM feature_gates WHERE feature_key = 'chat_commands';

COMMIT;
