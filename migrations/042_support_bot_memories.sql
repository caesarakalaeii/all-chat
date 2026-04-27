-- Postgres has no CREATE TYPE IF NOT EXISTS — guard via pg_type.
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'memory_type') THEN
        CREATE TYPE memory_type AS ENUM ('error_pattern', 'correction', 'codebase_insight');
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS bot_memories (
  id           SERIAL PRIMARY KEY,
  type         memory_type NOT NULL,
  tags         TEXT[]      NOT NULL DEFAULT '{}',
  content      TEXT        NOT NULL,
  access_count INTEGER     NOT NULL DEFAULT 0,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_bot_memories_tags ON bot_memories USING GIN (tags);
CREATE INDEX IF NOT EXISTS idx_bot_memories_updated_at ON bot_memories (updated_at DESC);
