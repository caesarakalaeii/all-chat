-- All-Chat Migration 057 DOWN: drop the overlay theme reference column

BEGIN;

ALTER TABLE overlay_configs
    DROP COLUMN IF EXISTS theme_id;

COMMIT;
