-- Migration 097: Beta-tester localization contribution (ADR-0063)
--
-- The localization contributor tool lets beta testers translate the UI string
-- catalog (frontend/src/lib/i18n, ADR-0055) into new locales. Answered design:
-- open, request-based locales; submit -> admin review; approved translations
-- exported to JSON and turned into catalog files by a script, so the repo
-- stays the single source of truth and nothing auto-commits.
--
-- Two tables:
--
-- 1. localization_locales — the locale registry. 'requested' locales are
--    proposed by contributors and invisible to other contributors until an
--    admin approves them; 'approved' locales accept translations. No
--    'rejected' status: rejecting a locale request means deleting the row,
--    which keeps the state machine two-state.
--
-- 2. localization_translations — one row per (locale, key), the currently
--    accepted submission for that key. Resubmitting overwrites the row and
--    resets it to 'pending'. A rejected row keeps its content so the
--    contributor sees why it was rejected; the review queue treats 'pending'
--    as actionable and everything else as history.
--
-- The key list itself is NOT stored here. It is derived from the English
-- catalog at export/generation time (scripts/generate-locale-catalog.mjs),
-- so it can never drift from the code the keys were read from. The backend
-- validates key shape (namespace.camelCase.path, <= 3 levels) but not
-- existence.
--
-- Access control: /api/v1/localization/* is gated by feature gate
-- 'localization_contribution' seeded early_access = TRUE (ADR-0020: beta
-- testers + ambassadors), enforced by shared/middleware.RequireEarlyAccess.
-- Flipping early_access FALSE opens contribution to all authenticated users.
--
-- IDEMPOTENCY: the migration runner replays every migration on each pod
-- restart, so every statement is safe to re-execute (CREATE IF NOT EXISTS,
-- ON CONFLICT DO NOTHING).

BEGIN;

CREATE TABLE IF NOT EXISTS localization_locales (
    code           VARCHAR(16) PRIMARY KEY,            -- BCP 47 code, e.g. 'de', 'pt-BR'
    english_name   VARCHAR(100) NOT NULL,             -- 'German' — shown in the picker
    native_name    VARCHAR(100) NOT NULL,              -- 'Deutsch' — shown to contributors
    status         VARCHAR(16) NOT NULL DEFAULT 'requested', -- 'requested' | 'approved'
    requested_by   UUID REFERENCES users(id) ON DELETE SET NULL,
    reviewed_by    UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT localization_locales_status_chk CHECK (status IN ('requested', 'approved'))
);

COMMENT ON TABLE localization_locales IS
    'Locale registry for the beta-tester localization tool (ADR-0063). '
    '''requested'' locales are proposals invisible to other contributors until '
    'an admin approves them; ''approved'' locales accept translations. The '
    'English source catalog is never a row here — it lives in the repo '
    '(frontend/src/lib/i18n/messages/en, ADR-0055).';

CREATE TABLE IF NOT EXISTS localization_translations (
    locale        VARCHAR(16) NOT NULL REFERENCES localization_locales(code) ON DELETE CASCADE,
    key           VARCHAR(200) NOT NULL,               -- dotted i18n key, e.g. 'common.save'
    value         TEXT NOT NULL,                       -- translated string with {placeholder} syntax
    status        VARCHAR(16) NOT NULL DEFAULT 'pending', -- 'pending' | 'approved' | 'rejected'
    submitted_by  UUID REFERENCES users(id) ON DELETE SET NULL,
    reviewed_by   UUID REFERENCES users(id) ON DELETE SET NULL,
    review_note   TEXT,                                -- shown to the contributor on rejection
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (locale, key),
    CONSTRAINT localization_translations_status_chk CHECK (status IN ('pending', 'approved', 'rejected'))
);

CREATE INDEX IF NOT EXISTS idx_localization_translations_status
    ON localization_translations (status);

COMMENT ON TABLE localization_translations IS
    'One row per (locale, key): the currently accepted submission for that '
    'key (ADR-0063). Resubmitting overwrites and resets to ''pending''. '
    'Approved rows are exported by the admin export endpoint and converted '
    'into a frontend catalog by scripts/generate-locale-catalog.mjs; a key '
    'missing here stays English. Rejected rows keep content + review_note so '
    'the contributor can revise.';

INSERT INTO feature_gates (feature_key, is_premium, early_access, description)
VALUES (
    'localization_contribution',
    TRUE,
    TRUE,
    'Beta-tester localization contributor tool (ADR-0063): translate the UI catalog into new locales. Submissions go through admin review before they reach the repo'
)
ON CONFLICT (feature_key) DO NOTHING;

COMMIT;
