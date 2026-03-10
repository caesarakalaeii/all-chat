-- Migration 031: Add has_seen_acceptance column to track sender notification state
-- Part of Phase 15-03: Bidirectional add-source prompts

ALTER TABLE share_requests
ADD COLUMN has_seen_acceptance BOOLEAN NOT NULL DEFAULT FALSE;

COMMENT ON COLUMN share_requests.has_seen_acceptance IS
  'Tracks if sender has seen the acceptance notification (realtime or dashboard prompt)';
