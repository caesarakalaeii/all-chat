-- Migration: 100_overlay_type
-- Description: Add overlay_type column to overlays — 'chat' (every existing
--   overlay), or one of the three new ADR-0064 kinds: 'alerts' (event alert
--   box), 'goal' (goal bar), 'list' (e.g. recent follower list). Existing rows
--   keep behaving exactly as before via the DEFAULT 'chat', and the CHECK
--   constraint keeps out the kinds nothing renders yet.
--
-- Idempotent: the migration runner re-applies every migration on each pod
-- start, so both the column add and the constraint add are guarded
-- (ADD COLUMN IF NOT EXISTS / DO $$ name lookup).

ALTER TABLE overlays
    ADD COLUMN IF NOT EXISTS overlay_type VARCHAR(20) NOT NULL DEFAULT 'chat';

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'overlays_overlay_type_check'
          AND conrelid = 'overlays'::regclass
    ) THEN
        ALTER TABLE overlays
            ADD CONSTRAINT overlays_overlay_type_check
            CHECK (overlay_type IN ('chat', 'alerts', 'goal', 'list'));
    END IF;
END $$;
