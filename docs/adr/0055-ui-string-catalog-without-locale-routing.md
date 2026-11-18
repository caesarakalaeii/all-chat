# ADR-0055: A typed UI string catalog, with locale never in the URL

**Date**: 2026-08-24
**Status**: Accepted
**Revised**: 2026-11-18
**Deciders**: caesarakalaeii

ADR numbering is shared with the caesar-deployment repo, so this is 0055; the
slug in the title is the stable identifier.

## Context

All-Chat had no i18n layer: every user-facing string was a literal at its render
site, and `<html lang>` was hardcoded. Dates and numbers follow the viewer's
browser locale wherever `toLocaleString()` / `new Intl.*` run without an
explicit locale: `MaintenanceBanner` rendered English copy with a German-formatted
date on a German machine. `errorMessages.ts` already uses single-brace
placeholders and promised easy internationalization that never landed.

Constraints: overlay URLs are pasted into OBS, the extension and the plugins and
must never change; the app is statically rendered by default (no middleware, so
no request-time locale read); there is zero React Context under `frontend/src`
outside the shadcn registry primitives (Storybook renders components
standalone under a required CI gate).

## Decision

**A pure, typed catalog module at `frontend/src/lib/i18n/`. English is the
default and source locale, German the second (ADR-0066, ADR-0078). Locale never
enters the URL, and it is not resolved at request time.**

- `config.ts` — `SUPPORTED_LOCALES = ['en', 'de'] as const`, `Locale`,
  `DEFAULT_LOCALE`, `LOCALE_NATIVE_NAMES`, `isSupportedLocale`; adding a
  language is this array, an entry, and one directory under `messages/`.
- `messages/en/` — one file per namespace (`as const`), composed by a barrel
  exporting `enMessages`/`EnMessages`; namespaces camelCase, nested at most
  three levels. The barrel composes by naming each namespace in a literal
  object: spreading or annotating the composed type would widen `MessageKey`
  to `string` and silently switch the compile-time key check off.
- `translate.ts` — `translate(messages, key, params?)`: `MessageKey` flattens
  the catalog into a union of dotted paths, never widening to `string` and
  using no `any`; placeholders are single braces per the `errorMessages.ts`
  convention.
- `format.ts` — `formatNumber` / `formatDateTime` / `formatCurrency`,
  defaulting to the active locale, `Intl` constructors cached.
- `index.ts` — `getTranslations(locale?)` (Server Components, plain modules,
  tests) and `useTranslations()` (Client Components), both returning the same
  `TFunction`.

- **Lookup never throws** (deliberate): overlay pages render in OBS on live
  broadcasts; an exception unmounts the tree and the viewer sees a black
  overlay, whereas a visible key is an obvious bug. An unknown key resolves to
  the key string; a missing param leaves `{placeholder}`; both warn in dev.
- `useTranslations()` subscribes to the module-level active locale
  (`active-locale.ts`, ADR-0066) rather than a context: Storybook renders
  components standalone under a required CI gate; no Zustand store either.
- No new dependency: a catalog needs neither ICU plurals nor `select` for its
  languages, and a `frontend/` lockfile change has broken the Docker build.
  If plurals are needed they go behind the same `t()` signature.
- Rollout: `layout.tsx` (skip-link text, `lang={DEFAULT_LOCALE}`) and
  `MaintenanceBanner.tsx` were the pilot; the rest followed in issue #799,
  which added the lint gate and split the file layout.
- Lint gate (prevents draining back to literals): `frontend/eslint.i18n.config.mjs`
  standalone config, required check via `.github/workflows/frontend-i18n.yml`,
  shrink-only baseline in `eslint.i18n.suppressions.json`. Two rules:
  `jsx-no-literals` `noStrings` for JSX text nodes, plus a
  `no-restricted-syntax` selector over `aria-label`/`placeholder`/`title`/
  `alt`/`label` for props. `eslint-plugin-react` is resolved out of
  `eslint-config-next` via `createRequire`, never added to
  `frontend/package.json`.

## Rejected

`[locale]` route segments would move `/overlay/[id]` to `/en/overlay/[id]`,
silently breaking, mid-broadcast, every overlay URL already pasted into OBS
browser sources; rule: **locale never goes in the URL, now or later.**
Request-time resolution (`cookies()`/`headers()` in the root layout) makes
every statically rendered, indexed page dynamic for a value that cannot vary
at render time; the client applies the stored choice after hydration
(ADR-0066). `errorMessages.ts` was not turned into the catalog (its
`Record<ChatErrorType>` shape is error-rendering-driven), though it is now
backed by the catalog's `errors` namespace.

## Consequences

- `MaintenanceBanner` dates now read in English on every host where before they
  followed the browser or OBS locale (the intended fix, and the one thing a
  user could notice); everything else renders byte-identically.
- Remaining unpinned `Intl` call sites are inconsistent with the formatters;
  they move over case by case after deciding whether the value is UI copy or
  viewer data.
- Adding another locale is a bounded change; the checklist is in
  `docs/frontend/I18N.md`. Nothing lints for the AGPL header all files under
  `frontend/src` carry, so each new file carries it by hand.

Testing: unit tests in `frontend/src/lib/i18n/__tests__/` cover nested lookup,
interpolation, fallbacks, formatter caching and the compile-time key check
(`@ts-expect-error` lines in `translate.test.ts` are a load-bearing gate: if
`MessageKey` degrades to `string`, `tsc --noEmit` fails).

## References

[docs/frontend/I18N.md](../frontend/I18N.md) (adding a string, the
"Adding another locale" checklist); ADR-0066 (locale selection), ADR-0078
(homepage switch, German for everyone); `frontend/src/lib/errorMessages.ts`
(placeholder convention); `sitemap.ts` / `robots.ts` (the indexed unprefixed
routes making locale routing unaffordable).
