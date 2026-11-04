-- Migration: 119_account_erasure
-- Description: Foreign-key delete rules that let account deletion (DELETE /auth/me)
--   and overlay deletion succeed (ADR-0088).
--
-- Before this migration, DELETE FROM users failed with a foreign-key violation
--   (the endpoint answered 500) for:
--   - anyone who ever sent or received a share request, and any overlay that was
--     ever shared: share_requests.sender_user_id / recipient_user_id /
--     sender_overlay_id were ON DELETE RESTRICT (migration 030), and share rows are
--     never deleted, only moved to a terminal status;
--   - any admin who ever banned a user or a platform id: users.banned_by and
--     banned_platform_ids.banned_by had no ON DELETE rule (migration 015).
--
-- Share requests are erased with either party or the shared overlay (CASCADE): a
--   share means nothing once one side is gone. Bans outlive the admin who issued
--   them (SET NULL; banned_platform_ids.banned_by becomes nullable for that).
--
-- A recipient subscribes to a shared overlay through an overlay_chat_sources row
--   (platform 'shared_overlay', channel_id = the sender overlay id as text) that has
--   no foreign key, so the trigger below removes those rows when the sender overlay
--   is deleted, instead of leaving them pointing at nothing.
--
-- Idempotent and cheap to re-run: scripts/run-migrations.sh applies every migration
--   on every pod start. Each constraint is replaced only when its delete rule is
--   wrong (matched by column, not by name, so a differently named legacy constraint
--   is replaced too); the nullability change and the trigger are guarded the same
--   way, so a re-run takes no table locks.

BEGIN;

DO $$
DECLARE
    fk  record;
    con record;
BEGIN
    FOR fk IN
        SELECT * FROM (VALUES
            ('share_requests',      'share_requests_sender_user_id_fkey',    'sender_user_id',    'users',    'CASCADE',  'c'),
            ('share_requests',      'share_requests_recipient_user_id_fkey', 'recipient_user_id', 'users',    'CASCADE',  'c'),
            ('share_requests',      'share_requests_sender_overlay_id_fkey', 'sender_overlay_id', 'overlays', 'CASCADE',  'c'),
            ('users',               'users_banned_by_fkey',                  'banned_by',         'users',    'SET NULL', 'n'),
            ('banned_platform_ids', 'banned_platform_ids_banned_by_fkey',    'banned_by',         'users',    'SET NULL', 'n')
        ) AS t(tbl, name, col, ref, action, code)
    LOOP
        FOR con IN
            SELECT c.conname
            FROM pg_constraint c
            JOIN pg_attribute a ON a.attrelid = c.conrelid AND a.attnum = c.conkey[1]
            WHERE c.contype = 'f'
              AND c.conrelid = fk.tbl::regclass
              AND c.confrelid = fk.ref::regclass
              AND cardinality(c.conkey) = 1
              AND a.attname = fk.col
              AND c.confdeltype <> fk.code
        LOOP
            EXECUTE format('ALTER TABLE %I DROP CONSTRAINT %I', fk.tbl, con.conname);
        END LOOP;

        IF NOT EXISTS (
            SELECT 1
            FROM pg_constraint c
            JOIN pg_attribute a ON a.attrelid = c.conrelid AND a.attnum = c.conkey[1]
            WHERE c.contype = 'f'
              AND c.conrelid = fk.tbl::regclass
              AND c.confrelid = fk.ref::regclass
              AND cardinality(c.conkey) = 1
              AND a.attname = fk.col
        ) THEN
            EXECUTE format('ALTER TABLE %I ADD CONSTRAINT %I FOREIGN KEY (%I) REFERENCES %I(id) ON DELETE %s',
                           fk.tbl, fk.name, fk.col, fk.ref, fk.action);
        END IF;
    END LOOP;

    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = current_schema() AND table_name = 'banned_platform_ids'
          AND column_name = 'banned_by' AND is_nullable = 'NO'
    ) THEN
        ALTER TABLE banned_platform_ids ALTER COLUMN banned_by DROP NOT NULL;
    END IF;
END $$;

CREATE OR REPLACE FUNCTION drop_shared_overlay_sources() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    -- lower(channel_id) matches idx_overlay_chat_sources_platform_lower_channel;
    -- uuid::text is already lower case.
    DELETE FROM overlay_chat_sources
    WHERE platform = 'shared_overlay' AND lower(channel_id) = OLD.id::text;
    RETURN OLD;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_trigger
        WHERE tgname = 'overlays_drop_shared_overlay_sources' AND tgrelid = 'overlays'::regclass
    ) THEN
        CREATE TRIGGER overlays_drop_shared_overlay_sources
            AFTER DELETE ON overlays
            FOR EACH ROW EXECUTE FUNCTION drop_shared_overlay_sources();
    END IF;
END $$;

COMMENT ON COLUMN banned_platform_ids.banned_by IS
    'Admin who issued the ban; NULL once that admin deleted their account (the ban stays in force).';

COMMIT;
