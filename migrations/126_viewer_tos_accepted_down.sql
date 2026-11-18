-- Rollback: 126_viewer_tos_accepted

ALTER TABLE viewers DROP COLUMN IF EXISTS tos_accepted_at;
ALTER TABLE viewers DROP COLUMN IF EXISTS tos_accepted;
