-- Down migration: Remove the partial unique index on public overlays
DROP INDEX IF EXISTS idx_overlays_one_public_per_user;
