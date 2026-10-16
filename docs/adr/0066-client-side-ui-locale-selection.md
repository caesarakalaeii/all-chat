# ADR-0066: German as a second UI language, chosen per browser and applied after hydration

**Date**: 2026-10-15
**Status**: Proposed
**Deciders**: caesarakalaeii

(ADR numbering is shared with the caesar-deployment repo. If 0066 is already
taken there, renumber this file; the slug `client-side-ui-locale-selection` is
the stable identifier.)

## Context and Problem Statement

ADR-0055 built a typed UI string catalog with English as its only locale and
deliberately deferred the question of how a second locale is chosen. Its two
constraints still hold: locale never goes in the URL (overlay URLs are pasted
into OBS, the extension and the Stream Deck plugins), and reading `cookies()` or
`headers()` in the root layout would make every statically rendered page
dynamic. ADR-0063 then gave beta testers a tool to translate the catalog, but
no locale was ever wired in.

German is the obvious first language: the operator and much of the early
audience are German-speaking. This record decides how a user picks it, where
the choice lives, how it reaches components that have no React Context, and
who gets it first.

## Decision Drivers

- No locale segment in any path, ever (ADR-0055).
- Static pages stay static: no request-time locale read.
- No React Context: Storybook renders components standalone under a required
  CI gate (ADR-0055).
- An OBS browser source must never change language on its own mid-broadcast.
- The first catalog is a machine translation, so it needs reviewers before it
  reaches everyone.
- Binding legal text must not be published in a language nobody reviewed.

## Considered Options

1. **Account preference in the database, resolved at request time.**
   Rejected: request-time resolution is what ADR-0055 ruled out, for the same
   reason (every page becomes dynamic), and a server-side preference cannot reach
   a statically rendered page without it.
2. **`Accept-Language` / `navigator.language` detection.** Rejected: an OBS
   instance on a German machine would switch a streamer's overlay chrome to
   German without anyone choosing it, and the server-side variant is a
   request-time read again.
3. **Explicit choice stored in the browser, applied on the client after
   hydration.** Chosen.

## Decision Outcome

**Chosen**: option 3.

- `frontend/src/lib/i18n/active-locale.ts` holds the active locale as a module
  singleton with a subscribe/notify pair. It starts at `DEFAULT_LOCALE` on the
  server and on the client.
- `components/LocaleSync.tsx`, mounted in the root layout, calls
  `restoreStoredLocale()` in an effect: it reads `localStorage.allchat_locale`,
  loads that catalog with a dynamic `import()`, then activates it, which sets
  `<html lang>` and notifies subscribers. Because nothing changes before that
  effect, the first client render matches the static HTML for every reader of
  the locale, so there is no hydration mismatch.
- `useTranslations()` subscribes through `useSyncExternalStore` (server
  snapshot `DEFAULT_LOCALE`), so a switch re-renders every subscriber. It returns
  one translator for the life of the page that resolves the active locale per
  call: the stored choice lands right after hydration, and if `t` changed
  identity then, every effect listing it would run twice per page load, some of
  them one-shot (the OAuth callback's single-use code exchange). A `useMemo`
  keyed only on `t` therefore keeps its first copy.
  It lives in its own `'use client'` module because Server Components import the
  same barrel.
- `getTranslations()` without an argument returns a translator that resolves the
  active locale per call, so module-scope translators in plain modules
  (`errorMessages.ts`) follow a switch. `formatNumber` / `formatDateTime`
  default to the active locale too.
- Non-English catalogs are partial by type (`LocaleMessages`). A key a catalog
  lacks resolves to English, never to the raw key. Hand-written namespace files
  are annotated with the complete `LocaleNamespace<'ns'>` shape, so a missing or
  extra key in them is a compile error.
- German translates every namespace except `legal`, which stays English until a
  legal review signs off a German privacy policy, terms and cookie notice.
- The picker is a card on `/settings`. Who may pick a non-English language is
  the `ui_languages` feature gate (migration 103), enforced by share-service on
  `GET /api/v1/ui-languages`, which the card probes. Seeded
  `early_access=TRUE, is_premium=FALSE`: beta testers and ambassadors first,
  then everyone, free, by flipping `early_access` in the admin UI. English stays
  selectable in every state so nobody is stuck in a language they lost access to.

## Consequences

### Positive

- No URL changes, no new dependency, no middleware, no static page made dynamic.
- Components and call sites are untouched: the switch is entirely inside
  `src/lib/i18n/`, as ADR-0055 intended.
- The English bundle does not grow per language: a catalog is fetched only by a
  browser that chose it.
- The rollout is reversible from the admin UI without a deploy.

### Negative

- **Server-rendered pages stay English.** The marketing homepage, guides, docs,
  blog chrome, metadata and the skip link are Server Components and always render
  `DEFAULT_LOCALE`. The German catalog covers their namespaces anyway, so they
  switch the day those surfaces resolve a locale client-side.
- **A brief English frame** on load for a German user, before the effect runs.
- **The choice is per browser, not per account.** An OBS browser source has its
  own storage, so overlays in OBS stay English unless chosen there; this is
  intended, but it also means a second device needs the choice again.
- **Strings resolved once at module load stay English**, e.g. the editor's
  `EDITOR_LOADING_LABEL` constants.
- The gate controls who can pick a language, not what renders: a stored choice
  keeps applying after a gate flip.
- The German catalog is not in the localization tool's database. Regenerating
  `messages/de/` from an export overwrites each namespace file whole and drops
  every key the export lacks; merge instead.

## Implementation

- **Files**: `frontend/src/lib/i18n/{config,active-locale,translators,use-translations,translate,format,index}.ts`,
  `frontend/src/lib/i18n/messages/de/`, `frontend/src/components/LocaleSync.tsx`,
  `frontend/src/components/settings/LanguageSettingsCard.tsx`,
  `shared/featuregates/cache.go` (`GateUILanguages`),
  `services/share-service/cmd/main.go`, `services/api-gateway/` (route + proxy prefix),
  `migrations/103_ui_languages_gate.sql`.
- **Tests**: `frontend/src/lib/i18n/__tests__/index.test.tsx` (switching without
  re-running effects, fallback, stored choice), `__tests__/de.test.ts`
  (namespace coverage, placeholder parity),
  `frontend/src/components/settings/__tests__/LanguageSettingsCard.test.tsx`
  (gate states), `shared/featuregates/ui_languages_gate_test.go` (seed).

## Related Decisions

- [ADR-0055](./0055-ui-string-catalog-without-locale-routing.md): the catalog, and
  the rejection of locale routing and request-time resolution this builds on.
- [ADR-0063](./0063-localization-contribution.md): the contributor tool that
  improves the German catalog.
- [ADR-0008](./0008-feature-gate-infrastructure.md) and
  [ADR-0020](./0020-beta-tester-role.md): feature gates and early access.
