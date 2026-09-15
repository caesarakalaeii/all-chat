-- Migration 097 down: beta-tester localization contribution (ADR-0063)
--
-- Drops the two localization tables and their feature gate. The English
-- source catalog lives in the repo and is untouched. No generated locale
-- files are removed by this migration — any already-generated
-- frontend/src/lib/i18n/messages/<locale>/ directories are removed in the
-- reverting commit, not here.

BEGIN;

DELETE FROM feature_gates WHERE feature_key = 'localization_contribution';

DROP TABLE IF EXISTS localization_translations;
DROP TABLE IF EXISTS localization_locales;

COMMIT;
