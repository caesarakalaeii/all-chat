-- 053: per-overlay 7TV emote set override.
--
-- Lets a streamer attach a 7TV emote set to an overlay independently of the
-- platform sources. Useful when no Twitch source exists (so the platform
-- connection lookup misses) or when the streamer wants emotes from a different
-- 7TV identity than the one linked to their Twitch/YouTube/Kick account.
--
-- Stores the resolved canonical emote-set ID. The overlay-manager handler
-- accepts user input as raw IDs, 7tv.app emote-set URLs, or profile URLs and
-- resolves to this canonical form on save.

ALTER TABLE overlay_configs
    ADD COLUMN IF NOT EXISTS seventv_emote_set_id VARCHAR(24);
