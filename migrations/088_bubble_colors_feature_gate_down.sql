-- All-Chat Migration 088 (down): remove the bubble colors feature gate
--
-- Reverses 088_bubble_colors_feature_gate.sql. Safe to run if the row is absent.
--
-- With the row gone the gate cache reports the key as not-premium, so the feature
-- stays open — the same state it ships in. Nothing to migrate back.

BEGIN;

DELETE FROM feature_gates WHERE feature_key = 'bubble_colors';

COMMIT;
