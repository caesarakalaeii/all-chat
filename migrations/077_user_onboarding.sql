-- Migration: 077_user_onboarding
-- Description: Add users.onboarding_completed_at for the first-run setup guide.
--
-- NULL means the user has not finished (or dismissed) the first-run onboarding
-- and the frontend shows the setup guide; a timestamp means finished/dismissed.
-- Restarting onboarding from Settings sets the column back to NULL.
--
-- The migration runner (scripts/run-migrations.sh) re-runs EVERY migration on
-- every pod start, so the one-time backfill is guarded by the column-existence
-- check: it only runs in the same transaction that first adds the column.
-- Re-runs are a complete no-op and never touch user state (see
-- services/auth-service/repository/migrations_rerun_test.go).

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'users' AND column_name = 'onboarding_completed_at'
    ) THEN
        ALTER TABLE users ADD COLUMN onboarding_completed_at TIMESTAMP NULL;

        -- Backfill: users who already own an overlay have de-facto onboarded.
        -- Existing zero-overlay users (the signed-up-then-dropped-out cohort
        -- this feature targets) keep NULL and get the setup guide on their
        -- next visit.
        UPDATE users u
        SET onboarding_completed_at = NOW()
        WHERE EXISTS (SELECT 1 FROM overlays o WHERE o.user_id = u.id);
    END IF;
END $$;

COMMENT ON COLUMN users.onboarding_completed_at IS
    'When the user finished or dismissed the first-run setup guide; NULL = guide is shown';
