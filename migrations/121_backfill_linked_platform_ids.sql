-- All-Chat Migration 121: record linked Twitch and Kick identities on users
-- Migration: 121
--
-- Linking a second platform (a Kick account connecting Twitch, a Twitch account
-- connecting Kick) set the platform id only on the in-memory user: Update never
-- wrote twitch_id / kick_id, so the link was lost. A later login with the linked
-- platform found no account by id, tried to create a new one and failed on
-- idx_users_username_lower_unique ("Failed to process user"). auth-service now
-- records the id when the link happens; this backfills accounts linked before.
--
-- The source is the per-link credential table, written only by the add-source
-- consent of that platform account itself, so the id is one the account owner
-- proved they control. Rows are skipped when the id is ambiguous or already
-- owned: another account holds it (a separate login with that platform), or the
-- same platform account is linked to more than one All-Chat account. YouTube is
-- not backfilled: youtube_oauth_tokens holds channel ids, not the Google account
-- id that google_id stores.
--
-- IDEMPOTENCY: every service runs the full migration set on each pod restart
-- (the runner does not track applied migrations). Only NULL columns are filled,
-- so a re-run changes nothing.

BEGIN;

UPDATE users u
SET twitch_id = t.twitch_user_id, updated_at = NOW()
FROM (
    SELECT DISTINCT ON (user_id) user_id, twitch_user_id
    FROM twitch_oauth_tokens
    ORDER BY user_id, updated_at DESC
) t
WHERE u.id = t.user_id
  AND u.twitch_id IS NULL
  AND NOT EXISTS (SELECT 1 FROM users o WHERE o.twitch_id = t.twitch_user_id)
  AND NOT EXISTS (
      SELECT 1 FROM twitch_oauth_tokens t2
      WHERE t2.twitch_user_id = t.twitch_user_id AND t2.user_id <> t.user_id
  );

UPDATE users u
SET kick_id = k.kick_user_id, updated_at = NOW()
FROM (
    SELECT DISTINCT ON (user_id) user_id, kick_user_id
    FROM kick_oauth_tokens
    WHERE kick_user_id IS NOT NULL
    ORDER BY user_id, updated_at DESC
) k
WHERE u.id = k.user_id
  AND u.kick_id IS NULL
  AND NOT EXISTS (SELECT 1 FROM users o WHERE o.kick_id = k.kick_user_id)
  AND NOT EXISTS (
      SELECT 1 FROM kick_oauth_tokens k2
      WHERE k2.kick_user_id = k.kick_user_id AND k2.user_id <> k.user_id
  );

COMMIT;
