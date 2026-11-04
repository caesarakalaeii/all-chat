-- Migration: 119_account_erasure (down)
-- Reverses 119_account_erasure.sql: restores the RESTRICT / NO ACTION delete rules
-- of migrations 015 and 030 and drops the shared-source cleanup trigger. Safe to run
-- when 119 was never applied.
--
-- Restoring NOT NULL on banned_platform_ids.banned_by fails while a ban has lost its
-- admin (banned_by IS NULL); such rows are left as they are and the column stays
-- nullable rather than deleting bans on a down migration.

BEGIN;

DROP TRIGGER IF EXISTS overlays_drop_shared_overlay_sources ON overlays;
DROP FUNCTION IF EXISTS drop_shared_overlay_sources();

ALTER TABLE share_requests
    DROP CONSTRAINT IF EXISTS share_requests_sender_user_id_fkey,
    DROP CONSTRAINT IF EXISTS share_requests_recipient_user_id_fkey,
    DROP CONSTRAINT IF EXISTS share_requests_sender_overlay_id_fkey,
    ADD CONSTRAINT share_requests_sender_user_id_fkey
        FOREIGN KEY (sender_user_id) REFERENCES users(id) ON DELETE RESTRICT,
    ADD CONSTRAINT share_requests_recipient_user_id_fkey
        FOREIGN KEY (recipient_user_id) REFERENCES users(id) ON DELETE RESTRICT,
    ADD CONSTRAINT share_requests_sender_overlay_id_fkey
        FOREIGN KEY (sender_overlay_id) REFERENCES overlays(id) ON DELETE RESTRICT;

ALTER TABLE users
    DROP CONSTRAINT IF EXISTS users_banned_by_fkey,
    ADD CONSTRAINT users_banned_by_fkey FOREIGN KEY (banned_by) REFERENCES users(id);

ALTER TABLE banned_platform_ids
    DROP CONSTRAINT IF EXISTS banned_platform_ids_banned_by_fkey,
    ADD CONSTRAINT banned_platform_ids_banned_by_fkey FOREIGN KEY (banned_by) REFERENCES users(id);

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM banned_platform_ids WHERE banned_by IS NULL) THEN
        ALTER TABLE banned_platform_ids ALTER COLUMN banned_by SET NOT NULL;
    END IF;
END $$;

COMMIT;
