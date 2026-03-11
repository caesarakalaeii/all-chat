-- Migration 031 down: Remove has_seen_acceptance column

ALTER TABLE share_requests
DROP COLUMN has_seen_acceptance;
