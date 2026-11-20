# ADR-0066: German as a second UI language, chosen per browser and applied after hydration

**Date**: 2026-10-15
**Status**: Accepted
**Revised**: 2026-11-20 (French and Spanish added)
**Deciders**: caesarakalaeii

## Context and Problem Statement

ADR-0055 built a typed UI string catalog and deliberately deferred the question
of how a second locale is chosen. Its two constraints still hold: locale never
goes in the URL (overlay URLs are pasted into OBS, the extension and the Stream
Deck plugins), and reading `cookies()` or `headers()` in the root layout would
make every statically rendered page dynamic. ADR-0063 then gave beta testers a
tool to translate the catalog, but no locale was wired in at decision time.

German is the obvious first language: the operator and much of the early
audience are German-speaking. This record decides how a user picks it, where
the choice lives, how it reaches components that have no React Context, and
who gets it first.

## Decision Outcome

**Explicit choice, stored per browser, applied on the client after
hydration.** Rejected: account preference resolved at request time (the
ADR-0055 ruling); `Accept-Language` / `navigator.language` detection (an OBS
instance on a German machine would switch a live overlay's language without
anyone choosing it).

- `frontend/src/lib/i18n/active-locale.ts` holds the active locale as a
  module singleton with a subscribe/notify pair, starting at `DEFAULT_LOCALE`
  on server and client. `components/LocaleSync.tsx`, mounted in the root
  layout, restores `localStorage.allchat_locale` in an effect (dynamic
  `import()` of the catalog, then activate: sets `<html lang>` and notifies).
  Nothing changes before that effect, so the first client render matches the
  static HTML and there is no hydration mismatch.
- `useTranslations()` subscribes via `useSyncExternalStore` (server snapshot
  `DEFAULT_LOCALE`) and stays one translator for the life of the page
  (`useMemo` keyed on it): if it changed identity when the stored choice
  lands, every effect listing it would run twice per page load, some one-shot
  (the OAuth callback's single-use code exchange). Lives in its own
  `'use client'` module because Server Components import the same barrel.
- `getTranslations()` without an argument returns a translator resolving the
  active locale per call, so module-scope translators (`errorMessages.ts`)
  follow a switch; the formatters in `format.ts` default to the active locale
  too.
- Non-English catalogs are partial by type (`LocaleMessages`): a missing key
  resolves to English, never to the raw key; hand-written namespace files are
  annotated with the complete `LocaleNamespace` shape, so a missing or extra
  key is a compile error.
- German, French and Spanish translate every namespace except `legal`, which
  stays English until legal review signs off a translated text. French and
  Spanish were added on 2026-11-20: machine translated, with the French strings
  volunteers submitted through the contributor tool (ADR-0063) taken over. The picker is a card on
  `/settings`; who may pick a non-English language is the `ui_languages`
  feature gate (migration 103), probed via share-service's
  `GET /api/v1/ui-languages` and enforced on the card. English stays
  selectable in every state.

## Consequences

No URL changes, no dependency, no middleware, no static page made dynamic;
the switch is entirely inside `src/lib/i18n/`; the English bundle does not
grow per language; rollout reversible from the admin UI. Costs:

- Server-rendered pages stay English (Server Components render
  `DEFAULT_LOCALE`) plus a brief English frame on load; strings resolved once
  at module load stay English too.
- The choice is per browser, not per account: an OBS browser source has its
  own storage (intended), and a second device needs the choice again.
- The gate controls who can pick, not what renders: a stored choice keeps
  applying after a gate flip.
- The shipped catalogs are not in the localization tool's DB (French only in
  part): regenerating `messages/<locale>/` from an export overwrites each namespace file whole; merge
  instead.

## Implementation

- **Files**: `frontend/src/lib/i18n/{config,active-locale,translators,use-translations,translate,format,index}.ts`,
  `frontend/src/lib/i18n/messages/{de,fr,es}/`, `frontend/src/components/LocaleSync.tsx`,
  `frontend/src/components/settings/LanguageSettingsCard.tsx`,
  `frontend/src/components/home/LanguageToggle.tsx` (ADR-0078),
  `shared/featuregates/cache.go` (`GateUILanguages`),
  `services/share-service/cmd/main.go`, `services/api-gateway/` (route + proxy prefix),
  `migrations/103_ui_languages_gate.sql`.
- **Tests**: `frontend/src/lib/i18n/__tests__/index.test.tsx` (switching without
  re-running effects, fallback, stored choice), `__tests__/locales.test.ts`
  (key coverage, placeholder parity, per locale),
  `frontend/src/components/settings/__tests__/LanguageSettingsCard.test.tsx`
  (gate states), `shared/featuregates/ui_languages_gate_test.go` (seed).

## Related Decisions

- [ADR-0055](./0055-ui-string-catalog-without-locale-routing.md): the catalog, and
  the rejection of locale routing and request-time resolution this builds on.
- [ADR-0063](./0063-localization-contribution.md): the contributor tool that
  improves the translated catalogs.
- [ADR-0078](./0078-homepage-language-switch-and-german-for-everyone.md): amends
  where the picker lives (a homepage switch too) and who the gate admits
  (everyone).
- [ADR-0008](./0008-feature-gate-infrastructure.md) and
  [ADR-0020](./0020-beta-tester-role.md): feature gates and early access.
