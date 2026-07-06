-- 068_engagement_points.sql
-- Description: Viewer points economy (issue #523). Per-overlay (per channel-scope)
-- virtual currency: a materialized balance plus an append-only ledger of record.
-- points_transactions.dedup_key UNIQUE is the double-award / idempotency guard
-- (earn events, wagers, payouts, refunds all insert ON CONFLICT (dedup_key) DO NOTHING).
-- Reuses the existing viewers/viewer_platform_identities identity (035).

CREATE TABLE IF NOT EXISTS viewer_points (
    viewer_id  UUID NOT NULL REFERENCES viewers(id) ON DELETE CASCADE,
    overlay_id UUID NOT NULL REFERENCES overlays(id) ON DELETE CASCADE,
    balance    BIGINT NOT NULL DEFAULT 0 CHECK (balance >= 0),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    PRIMARY KEY (viewer_id, overlay_id)
);

CREATE TABLE IF NOT EXISTS points_transactions (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    viewer_id  UUID NOT NULL REFERENCES viewers(id) ON DELETE CASCADE,
    overlay_id UUID NOT NULL REFERENCES overlays(id) ON DELETE CASCADE,
    delta      BIGINT NOT NULL,                       -- signed: +earn / -wager / +payout / +refund
    reason     VARCHAR(32) NOT NULL,                  -- earn_bits|earn_sub|earn_gift|earn_chat|earn_watch|wager|payout|refund|adjust
    ref_type   VARCHAR(16),                           -- event|prediction|poll|heartbeat|chat
    ref_id     UUID,
    dedup_key  TEXT NOT NULL UNIQUE,                  -- idempotency: the double-award guard
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_points_txn_viewer_overlay ON points_transactions(viewer_id, overlay_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_points_txn_ref ON points_transactions(ref_type, ref_id);

-- Per-overlay earning configuration. One row per overlay; engagement-service
-- lazily inserts defaults on first read. points_name keeps the currency label
-- configurable (issue #523 lists Sparks/Waves/Flow/Hype/Coins as candidates).
CREATE TABLE IF NOT EXISTS points_earn_config (
    overlay_id       UUID PRIMARY KEY REFERENCES overlays(id) ON DELETE CASCADE,
    points_name      VARCHAR(32) NOT NULL DEFAULT 'Points',
    bits_multiplier  NUMERIC(10,4) NOT NULL DEFAULT 1,      -- points per bit
    usd_multiplier   NUMERIC(10,4) NOT NULL DEFAULT 100,    -- points per USD (super chat / donation)
    sub_high         BIGINT NOT NULL DEFAULT 500,           -- tier 3 / high
    sub_medium       BIGINT NOT NULL DEFAULT 300,           -- tier 2 / medium
    sub_low          BIGINT NOT NULL DEFAULT 150,           -- tier 1 / low / default
    gift_per_sub     BIGINT NOT NULL DEFAULT 150,           -- points to gifter per gifted sub
    chat_per_minute  BIGINT NOT NULL DEFAULT 5,             -- capped once per active minute
    watch_per_minute BIGINT NOT NULL DEFAULT 2,             -- per heartbeat minute-bucket
    enabled          BOOLEAN NOT NULL DEFAULT FALSE,        -- opt-in: no economy accrues until the streamer turns it on
    created_at       TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMP NOT NULL DEFAULT NOW()
);
