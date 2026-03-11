-- Share requests and premium features support
-- Migration: 030
-- Description: Add share_requests table for overlay sharing and is_premium column for feature gating

-- Add is_premium column to users table
ALTER TABLE users ADD COLUMN IF NOT EXISTS is_premium BOOLEAN NOT NULL DEFAULT FALSE;

-- Create partial index for premium users (matches is_admin pattern from migration 009)
CREATE INDEX IF NOT EXISTS idx_users_is_premium ON users(is_premium) WHERE is_premium = TRUE;

-- Add comment
COMMENT ON COLUMN users.is_premium IS 'Whether the user has premium subscription for advanced features';

-- Create share_requests table
CREATE TABLE IF NOT EXISTS share_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sender_user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    sender_overlay_id UUID NOT NULL REFERENCES overlays(id) ON DELETE RESTRICT,
    recipient_user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    status VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'accepted', 'rejected', 'expired')),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    responded_at TIMESTAMP,
    expires_at TIMESTAMP NOT NULL DEFAULT (NOW() + INTERVAL '7 days'),
    CONSTRAINT no_self_share CHECK (sender_user_id != recipient_user_id)
);

-- Create indexes for efficient querying
-- Index for listing incoming requests filtered by status (most common query)
CREATE INDEX IF NOT EXISTS idx_share_requests_recipient ON share_requests(recipient_user_id, status);

-- Index for listing sent requests
CREATE INDEX IF NOT EXISTS idx_share_requests_sender ON share_requests(sender_user_id);

-- Partial index for expiry job (only pending requests need expiry checks)
CREATE INDEX IF NOT EXISTS idx_share_requests_expiry ON share_requests(status, expires_at) WHERE status = 'pending';

-- Add table and column comments
COMMENT ON TABLE share_requests IS 'Bidirectional overlay sharing requests between users';
COMMENT ON COLUMN share_requests.sender_user_id IS 'User who initiated the share request';
COMMENT ON COLUMN share_requests.sender_overlay_id IS 'Overlay being shared';
COMMENT ON COLUMN share_requests.recipient_user_id IS 'User receiving the share request';
COMMENT ON COLUMN share_requests.status IS 'Request status: pending (awaiting response), accepted (active share), rejected (declined), expired (timeout)';
COMMENT ON COLUMN share_requests.created_at IS 'When the share request was created';
COMMENT ON COLUMN share_requests.responded_at IS 'When the recipient responded (accepted/rejected), NULL for pending/expired';
COMMENT ON COLUMN share_requests.expires_at IS 'When pending requests automatically expire (default: 7 days from creation)';
