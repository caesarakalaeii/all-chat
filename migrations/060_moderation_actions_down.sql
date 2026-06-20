-- All-Chat Migration 060 rollback: drop the moderation action audit log.
BEGIN;
DROP TABLE IF EXISTS moderation_actions;
COMMIT;
