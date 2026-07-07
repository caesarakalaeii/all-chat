-- All-Chat Migration 076 (down): remove the engagement feature gate
--
-- Reverses 076_engagement_feature_gate.sql. Safe to run if the row is absent.

BEGIN;

DELETE FROM feature_gates WHERE feature_key = 'engagement';

COMMIT;
