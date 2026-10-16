-- Migration: 110_goal_configs (down)
-- Reverses 110_goal_configs.sql. Safe to run when either object is absent.

BEGIN;

DELETE FROM feature_gates WHERE feature_key = 'goal_overlay';
DROP TABLE IF EXISTS goal_configs;

COMMIT;
