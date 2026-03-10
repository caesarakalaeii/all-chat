-- Migration 033: Add 'revoked' status to share_requests CHECK constraint
ALTER TABLE share_requests
    DROP CONSTRAINT IF EXISTS share_requests_status_check;

ALTER TABLE share_requests
    ADD CONSTRAINT share_requests_status_check
    CHECK (status IN ('pending', 'accepted', 'rejected', 'expired', 'revoked'));
