-- Rollback for 054: narrow seventv_emote_set_id back to VARCHAR(24).
--
-- WARNING: this will fail if any row contains a value longer than 24 chars
-- (e.g. a 26-char ULID). Manually clear or truncate such rows before
-- applying this rollback.

ALTER TABLE overlay_configs
    ALTER COLUMN seventv_emote_set_id TYPE VARCHAR(24);
