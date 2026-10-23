-- Migration: 115_chat_bots (down)
-- Reverses 115_chat_bots.sql. Custom bot credentials are deleted with their
-- owner's rows; the All-Chat Bot credentials and the gate seed go too. Safe to
-- run when the objects are absent.

BEGIN;

DELETE FROM feature_gates WHERE feature_key = 'custom_chat_bot';

DROP TABLE IF EXISTS chat_bot_settings;
DROP TABLE IF EXISTS chat_bot_accounts;

COMMIT;
