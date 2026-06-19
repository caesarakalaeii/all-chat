-- All-Chat Migration 061 (down): remove the moderation feature gate
--
-- Reverses 061_moderation_feature_gate.sql. Safe to run if the row is absent.

BEGIN;

DELETE FROM feature_gates WHERE feature_key = 'moderation';

COMMIT;
