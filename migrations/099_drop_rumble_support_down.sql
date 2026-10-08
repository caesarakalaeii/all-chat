-- All-Chat Migration 099 Down: no-op
--
-- Rumble support was withdrawn on purpose. Re-seeding supported_platforms or
-- the platform_rumble gate would advertise a platform that has no listener
-- and no normalizer, and the deleted overlay_chat_sources rows cannot be
-- restored from here. Nothing to roll back.

SELECT 1;
