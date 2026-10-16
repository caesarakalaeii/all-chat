-- Migration: 111_list_configs (down)
-- Reverses 111_list_configs.sql. Safe to run when either object is absent.

BEGIN;

DELETE FROM feature_gates WHERE feature_key = 'list_overlay';
DROP TABLE IF EXISTS list_configs;

COMMIT;
