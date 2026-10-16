-- Migration: 108_alerts_overlay_gate (down)
-- Reverses 108_alerts_overlay_gate.sql. Safe to run when the row is absent.

BEGIN;

DELETE FROM feature_gates WHERE feature_key = 'alerts_overlay';

COMMIT;
