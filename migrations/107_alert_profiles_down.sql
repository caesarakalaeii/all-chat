-- Migration: 107_alert_profiles (down)
-- Reverses 107_alert_profiles.sql. Safe to run when either object is absent.

BEGIN;

DELETE FROM feature_gates WHERE feature_key = 'alerts_editor';
DROP TABLE IF EXISTS alert_profiles;

COMMIT;
