-- 071_engagement_native_mirror_down.sql
-- Reverses 071: drops the mirror tally columns. 071 no longer rescopes indexes
-- (069/070 create them per-overlay directly), so this down path deliberately does NOT
-- recreate any global (source, external_id) index — rebuilding a global unique over real
-- multi-overlay data would abort (the P0-1 landmine, ex-P3-1). The per-overlay indexes
-- are dropped with the polls/predictions tables in 069/070's down migrations.
ALTER TABLE prediction_outcomes DROP COLUMN IF EXISTS mirror_entrants;
ALTER TABLE prediction_outcomes DROP COLUMN IF EXISTS mirror_points;
ALTER TABLE poll_options DROP COLUMN IF EXISTS mirror_votes;
