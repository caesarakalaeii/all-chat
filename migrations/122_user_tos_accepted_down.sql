-- Rollback: 122_user_tos_accepted

ALTER TABLE users DROP COLUMN IF EXISTS tos_accepted_at;
ALTER TABLE users DROP COLUMN IF EXISTS tos_accepted;
