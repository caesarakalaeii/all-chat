-- 075_engagement_poll_vote_seq_down.sql
-- Reverses 075: drops the poll-vote ordering column.
ALTER TABLE poll_votes DROP COLUMN IF EXISTS seq;
