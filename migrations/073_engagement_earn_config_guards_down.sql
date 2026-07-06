-- 073_engagement_earn_config_guards_down.sql
-- Reverse 072: drop the earn-config CHECK bounds and restore the pre-072 enabled
-- default (TRUE). Existing row values are left as-is.

ALTER TABLE points_earn_config ALTER COLUMN enabled SET DEFAULT TRUE;

ALTER TABLE points_earn_config DROP CONSTRAINT IF EXISTS ck_earn_bits_multiplier;
ALTER TABLE points_earn_config DROP CONSTRAINT IF EXISTS ck_earn_usd_multiplier;
ALTER TABLE points_earn_config DROP CONSTRAINT IF EXISTS ck_earn_sub_high;
ALTER TABLE points_earn_config DROP CONSTRAINT IF EXISTS ck_earn_sub_medium;
ALTER TABLE points_earn_config DROP CONSTRAINT IF EXISTS ck_earn_sub_low;
ALTER TABLE points_earn_config DROP CONSTRAINT IF EXISTS ck_earn_gift_per_sub;
ALTER TABLE points_earn_config DROP CONSTRAINT IF EXISTS ck_earn_chat_per_minute;
ALTER TABLE points_earn_config DROP CONSTRAINT IF EXISTS ck_earn_watch_per_minute;
