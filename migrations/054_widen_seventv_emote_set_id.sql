-- 054: widen seventv_emote_set_id from VARCHAR(24) to TEXT.
--
-- Migration 053 sized the column for legacy 24-char MongoDB ObjectIDs, but
-- 7TV's current scheme uses 26-char Crockford-base32 ULIDs (e.g.
-- 01K0BT1KXDYA24WQJD80CRZC75). Any user attaching a current 7TV emote set
-- (every set created post-migration to the new backend) hits a Postgres
-- "value too long for type character varying(24)" 500 on save. The resolver
-- code already accepts both formats; the column just has to fit them.
--
-- TEXT is used instead of VARCHAR(26) so any future ID-scheme tweak by 7TV
-- doesn't require another migration; the column stays small in practice
-- because real values are 24 or 26 chars.

ALTER TABLE overlay_configs
    ALTER COLUMN seventv_emote_set_id TYPE TEXT;
