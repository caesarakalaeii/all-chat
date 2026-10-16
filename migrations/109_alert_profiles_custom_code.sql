-- Migration: 109_alert_profiles_custom_code
-- Description: Premium custom code on alert profiles (issue #954, ADR-0071):
--   optional HTML, CSS and JS that the alert-box overlay renders in a sandboxed
--   iframe (sandbox="allow-scripts", srcdoc, no allow-same-origin) instead of the
--   built-in alert layout. Empty string means "no custom code" (the built-in
--   layout renders). Size caps are validated in Go (shared/alerts).
--
-- Idempotent: the migration runner re-applies every migration on each pod start.

BEGIN;

ALTER TABLE alert_profiles ADD COLUMN IF NOT EXISTS custom_html TEXT NOT NULL DEFAULT '';
ALTER TABLE alert_profiles ADD COLUMN IF NOT EXISTS custom_css  TEXT NOT NULL DEFAULT '';
ALTER TABLE alert_profiles ADD COLUMN IF NOT EXISTS custom_js   TEXT NOT NULL DEFAULT '';

COMMIT;
