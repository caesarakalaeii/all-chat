-- Rollback: 077_user_onboarding

ALTER TABLE users DROP COLUMN IF EXISTS onboarding_completed_at;
