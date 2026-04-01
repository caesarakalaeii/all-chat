-- Rollback: Revert public overlay default change
-- Note: Does NOT revert the backfill — overlay visibility should be managed manually

ALTER TABLE overlays ALTER COLUMN is_public_for_viewers SET DEFAULT false;
