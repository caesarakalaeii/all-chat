-- All-Chat Migration 064 DOWN: revert streamer/viewer premium split (ADR-0019)

BEGIN;

-- premium_subscriptions: drop the product dimension + viewer subject.
ALTER TABLE premium_subscriptions DROP CONSTRAINT IF EXISTS premium_subscriptions_one_subject_chk;
ALTER TABLE premium_subscriptions DROP CONSTRAINT IF EXISTS premium_subscriptions_product_chk;
DROP INDEX IF EXISTS idx_premium_subscriptions_viewer;
DELETE FROM premium_subscriptions WHERE viewer_id IS NOT NULL;
ALTER TABLE premium_subscriptions DROP COLUMN IF EXISTS viewer_id;
ALTER TABLE premium_subscriptions DROP COLUMN IF EXISTS product;

-- patreon_oauth_tokens: revert to the user-only (063) shape.
ALTER TABLE patreon_oauth_tokens DROP CONSTRAINT IF EXISTS patreon_oauth_tokens_one_subject_chk;
DROP INDEX IF EXISTS uq_patreon_oauth_tokens_user;
DROP INDEX IF EXISTS uq_patreon_oauth_tokens_viewer;
DROP INDEX IF EXISTS idx_patreon_oauth_tokens_viewer;
DELETE FROM patreon_oauth_tokens WHERE viewer_id IS NOT NULL;
ALTER TABLE patreon_oauth_tokens DROP COLUMN IF EXISTS viewer_id;
ALTER TABLE patreon_oauth_tokens ALTER COLUMN user_id SET NOT NULL;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'patreon_oauth_tokens_user_id_key'
    ) THEN
        ALTER TABLE patreon_oauth_tokens ADD CONSTRAINT patreon_oauth_tokens_user_id_key UNIQUE (user_id);
    END IF;
END $$;

-- viewers: drop the tri-state override.
ALTER TABLE viewers DROP COLUMN IF EXISTS premium_admin_override;

COMMIT;
