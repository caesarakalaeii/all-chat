-- All-Chat Impersonation Audit Log
-- Migration: 048
-- Description: Dedicated audit trail for admin impersonation events.
--              Required for DSGVO accountability (Art. 5(2)) — sensitive
--              actions must be traceable.

CREATE TABLE IF NOT EXISTS impersonation_audit_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    admin_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    admin_username VARCHAR(100) NOT NULL,
    target_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    target_username VARCHAR(100) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_impersonation_audit_admin ON impersonation_audit_log(admin_user_id);
CREATE INDEX idx_impersonation_audit_target ON impersonation_audit_log(target_user_id);
CREATE INDEX idx_impersonation_audit_created ON impersonation_audit_log(created_at);

COMMENT ON TABLE impersonation_audit_log IS
    'DSGVO Art.5(2) accountability: tracks every admin impersonation event.';
