-- Migration 044: Feature Gate Infrastructure
--
-- Creates the feature_gates table as the source of truth for capability-level
-- premium feature flags. is_premium=false means feature is free for all users.
-- In-memory caches per service are refreshed via Redis Pub/Sub + 60s TTL fallback.
--
-- ADR-0008: Feature Gate Infrastructure

CREATE TABLE IF NOT EXISTS feature_gates (
    feature_key VARCHAR(100) PRIMARY KEY,
    is_premium  BOOLEAN NOT NULL DEFAULT TRUE,
    description TEXT NOT NULL DEFAULT '',
    updated_at  TIMESTAMP NOT NULL DEFAULT NOW()
);

DROP TRIGGER IF EXISTS update_feature_gates_updated_at ON feature_gates;
CREATE TRIGGER update_feature_gates_updated_at
    BEFORE UPDATE ON feature_gates
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

INSERT INTO feature_gates (feature_key, is_premium, description)
VALUES ('sharing', TRUE, 'Overlay share requests — allows users to create and accept chat overlay shares')
ON CONFLICT (feature_key) DO NOTHING;

COMMENT ON TABLE feature_gates IS 'Capability-level premium feature flags. is_premium=false means feature is free for all users.';
COMMENT ON COLUMN feature_gates.feature_key IS 'Unique feature identifier, used as the gate key in middleware calls';
COMMENT ON COLUMN feature_gates.is_premium IS 'When true, user must have is_premium=true to access. When false, all authenticated users may access.';
