-- Durable all-time message counter. The homepage hero reads this through the
-- Redis key chat:stats:total; this table is the source of truth that survives
-- Redis flushes and lets a weekly job fold the expired daily buckets into a
-- permanent row instead of the counter growing only from live traffic.

CREATE TABLE IF NOT EXISTS landing_stats (
    key        VARCHAR(50) PRIMARY KEY,   -- 'all_time_messages'
    value      BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO landing_stats (key, value) VALUES ('all_time_messages', 0)
ON CONFLICT (key) DO NOTHING;
