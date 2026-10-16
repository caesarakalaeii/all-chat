-- All-Chat Migration 103 (down): remove the UI languages feature gate
--
-- Reverses 103_ui_languages_gate.sql. Safe to run if the row is absent.
--
-- With the row gone the gate cache treats the key as unknown, which fails closed
-- on both dimensions: only beta testers and ambassadors can pick a language,
-- the same audience the seed starts with. Choices already stored in browsers are
-- untouched either way.

BEGIN;

DELETE FROM feature_gates WHERE feature_key = 'ui_languages';

COMMIT;
