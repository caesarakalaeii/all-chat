-- Migration: 109_alert_profiles_custom_code (down)
-- Reverses 109_alert_profiles_custom_code.sql. Safe to run when the columns are absent.

BEGIN;

ALTER TABLE alert_profiles DROP COLUMN IF EXISTS custom_js;
ALTER TABLE alert_profiles DROP COLUMN IF EXISTS custom_css;
ALTER TABLE alert_profiles DROP COLUMN IF EXISTS custom_html;

COMMIT;
