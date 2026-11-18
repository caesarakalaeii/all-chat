-- All-Chat Migration 121 (down): intentionally empty
--
-- 121 only fills NULL twitch_id / kick_id columns from linked credentials. The
-- backfilled values cannot be told apart from ids written at login or link time,
-- and an auth-service image from before 121 reads them without trouble (it
-- simply finds the linked account on login), so there is nothing to reverse.

SELECT 1;
