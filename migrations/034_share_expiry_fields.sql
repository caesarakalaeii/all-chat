-- Migration 034: Add expiry fields to share_requests for active share lifecycle
-- expiry_option: which expiry type the recipient chose at acceptance time
-- share_expires_at: the computed deadline for custom time-based expiry (NULL for this_stream/unlimited)

ALTER TABLE share_requests
  ADD COLUMN IF NOT EXISTS expiry_option VARCHAR(20) DEFAULT 'unlimited',
  ADD COLUMN IF NOT EXISTS share_expires_at TIMESTAMP NULL;

-- Index for the 5-minute expiry job to efficiently find expired accepted shares
CREATE INDEX IF NOT EXISTS idx_share_requests_share_expires
  ON share_requests(share_expires_at, status)
  WHERE status = 'accepted' AND share_expires_at IS NOT NULL;
