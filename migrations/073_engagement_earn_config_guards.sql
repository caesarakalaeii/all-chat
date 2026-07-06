-- 073_engagement_earn_config_guards.sql
-- Description: Harden points_earn_config (issue #523, PR #524 round-5 M2/U3).
--   (1) CHECK constraints bound every numeric earn value to [0, cap] so an owner
--       cannot store a negative (which silently mints nothing) or a value that
--       overflows int64 in the earn engine. Bounds match the handler-side
--       validation (maxMultiplier=100000, maxEarnPoints=1000000).
--   (2) Flip the `enabled` column DEFAULT to FALSE so points earning is opt-in
--       (U3). Existing rows are deliberately NOT backfilled — an owner who already
--       enabled it stays on.
-- Idempotent + re-runnable every deploy (the runner replays all up-migrations):
-- guarded ADD CONSTRAINT via pg_constraint checks, and an ALTER DEFAULT that is a
-- no-op once applied. Only source='allchat' economics use this table.

-- Normalize any pre-existing out-of-range rows first so ADD CONSTRAINT can't fail
-- validation on data written before these bounds existed.
UPDATE points_earn_config SET bits_multiplier  = LEAST(GREATEST(bits_multiplier, 0), 100000);
UPDATE points_earn_config SET usd_multiplier   = LEAST(GREATEST(usd_multiplier, 0), 100000);
UPDATE points_earn_config SET sub_high         = LEAST(GREATEST(sub_high, 0), 1000000);
UPDATE points_earn_config SET sub_medium       = LEAST(GREATEST(sub_medium, 0), 1000000);
UPDATE points_earn_config SET sub_low          = LEAST(GREATEST(sub_low, 0), 1000000);
UPDATE points_earn_config SET gift_per_sub     = LEAST(GREATEST(gift_per_sub, 0), 1000000);
UPDATE points_earn_config SET chat_per_minute  = LEAST(GREATEST(chat_per_minute, 0), 1000000);
UPDATE points_earn_config SET watch_per_minute = LEAST(GREATEST(watch_per_minute, 0), 1000000);

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'ck_earn_bits_multiplier') THEN
        ALTER TABLE points_earn_config ADD CONSTRAINT ck_earn_bits_multiplier
            CHECK (bits_multiplier >= 0 AND bits_multiplier <= 100000);
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'ck_earn_usd_multiplier') THEN
        ALTER TABLE points_earn_config ADD CONSTRAINT ck_earn_usd_multiplier
            CHECK (usd_multiplier >= 0 AND usd_multiplier <= 100000);
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'ck_earn_sub_high') THEN
        ALTER TABLE points_earn_config ADD CONSTRAINT ck_earn_sub_high
            CHECK (sub_high >= 0 AND sub_high <= 1000000);
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'ck_earn_sub_medium') THEN
        ALTER TABLE points_earn_config ADD CONSTRAINT ck_earn_sub_medium
            CHECK (sub_medium >= 0 AND sub_medium <= 1000000);
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'ck_earn_sub_low') THEN
        ALTER TABLE points_earn_config ADD CONSTRAINT ck_earn_sub_low
            CHECK (sub_low >= 0 AND sub_low <= 1000000);
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'ck_earn_gift_per_sub') THEN
        ALTER TABLE points_earn_config ADD CONSTRAINT ck_earn_gift_per_sub
            CHECK (gift_per_sub >= 0 AND gift_per_sub <= 1000000);
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'ck_earn_chat_per_minute') THEN
        ALTER TABLE points_earn_config ADD CONSTRAINT ck_earn_chat_per_minute
            CHECK (chat_per_minute >= 0 AND chat_per_minute <= 1000000);
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'ck_earn_watch_per_minute') THEN
        ALTER TABLE points_earn_config ADD CONSTRAINT ck_earn_watch_per_minute
            CHECK (watch_per_minute >= 0 AND watch_per_minute <= 1000000);
    END IF;
END $$;

-- U3: points earning is opt-in. Change only the column DEFAULT for future lazy
-- inserts; do NOT backfill existing rows.
ALTER TABLE points_earn_config ALTER COLUMN enabled SET DEFAULT FALSE;
