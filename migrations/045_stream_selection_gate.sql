-- Feature gate for YouTube stream selection strategy (premium feature)
-- Allows premium users to choose how the innertube listener selects among
-- multiple concurrent live streams (e.g. most viewers, title match).
INSERT INTO feature_gates (feature_key, is_premium, description)
VALUES ('stream_selection', TRUE, 'YouTube stream selection strategy — choose which stream to monitor when a channel has multiple concurrent live streams')
ON CONFLICT (feature_key) DO NOTHING;
