# ADR-0055: A typed UI string catalog, with locale never in the URL

**Date**: 2026-08-24
**Status**: Accepted
**Deciders**: caesarakalaeii

(ADR numbering is shared with the caesar-deployment repo, so this is 0055. The
slug `ui-string-catalog-without-locale-routing` is the stable identifier; if the
number has to move, the slug does not.)

## Context and Problem Statement

All-Chat had no i18n layer of any kind. Every user-facing string was a literal at
its render site: no `next-intl`, `react-intl` or `i18next` in
`frontend/package.json`, and no match for any of them under `frontend/src`. The
document language was hardcoded, `<html lang="en">` in
`frontend/src/app/layout.tsx`.

The intent was on record and never acted on. `frontend/src/lib/errorMessages.ts`
opens with "Designed for easy internationalization in the future", and already
uses single-brace placeholders — `'Click the "Sign in with {platform}" button'`.

There is a second, quieter problem. 38 non-test call sites under `frontend/src`
call `toLocaleString()` or an `Intl.*` constructor with no explicit locale. That
means dates and numbers follow the **viewer's browser locale**, independently of
the UI language. `MaintenanceBanner` was the clearest case: English copy, a
formatter built as `new Intl.DateTimeFormat(undefined, ...)`, and therefore
"Expected completion: 4. März, 13:00" on a German machine. The same banner
rendered different text on different hosts, which is not a translation and not a
setting anyone chose.

Two properties of this codebase constrain any design, and both are easy to get
wrong.

**Overlay URLs are permanent and pasted everywhere.** `/overlay/[id]` goes into
OBS browser sources, shared links, the browser extension and the Stream Deck and
StreamController plugins. Public routes are indexed — `frontend/src/app/sitemap.ts`
and `frontend/src/app/robots.ts` list them unprefixed.

**The app is statically rendered by default.** There is no
`frontend/src/middleware.ts`, and exactly two routes opt into `force-dynamic`
(`src/app/legal/impressum/page.tsx`, `src/app/dev/theme-contrast/page.tsx`).

And one architectural fact: the frontend contains **zero React Context**.
`grep -rn 'createContext\|useContext' frontend/src` returns nothing;
`ToastProvider` is not a context provider. State is Zustand
(`frontend/src/lib/stores/`) or module singletons.

## Decision

**A pure, typed catalog module at `frontend/src/lib/i18n/`. English is the only
locale. Locale never enters the URL, and it is not resolved at request time.**

- `config.ts` — `SUPPORTED_LOCALES = ['en'] as const`, `Locale`,
  `DEFAULT_LOCALE`, `isSupportedLocale`. Adding a language means editing this one
  array and adding one file under `messages/`.
- `messages/en.ts` — a nested object literal ending in `as const`. Namespaces are
  camelCase, nested at most three levels, keyed by surface.
- `translate.ts` — `translate(messages, key, params?)`. `MessageKey` is a
  recursive mapped type flattening the catalog into a union of dotted paths. It
  does not widen to `string` and does not use `any`. Placeholders are single
  braces, reusing the `errorMessages.ts` convention rather than inventing a
  second syntax.
- `format.ts` — `formatNumber` and `formatDateTime`, both defaulting `locale` to
  `DEFAULT_LOCALE`, with the `Intl` constructors cached by locale plus serialised
  options.
- `index.ts` — `getTranslations(locale?)` for Server Components, plain modules
  and tests; `useTranslations()` for Client Components. Both return the identical
  `TFunction`.

### Lookup never throws

An unknown key resolves to the key string; a missing param leaves the
`{placeholder}` in place. Both warn via `console.warn` when
`process.env.NODE_ENV !== 'production'`.

This is not defensive habit. Overlay pages render inside OBS browser sources on
live broadcasts. An exception raised from a string lookup unmounts the tree and
the viewer sees a black overlay for the rest of the stream. A visible
`maintenanceBanner.activeLabel` is ugly and obviously a bug; a blank overlay is
someone's broadcast. The degradation is chosen in that direction deliberately.

### Two locales' worth of seam, one locale's worth of code

`useTranslations()` is today a thin wrapper over `getTranslations(DEFAULT_LOCALE)`.
It exists so that adding a locale is a change inside `src/lib/i18n/` rather than a
sweep across every component. It is **not** a hook over a context: Storybook
renders components standalone and `npx vitest --project storybook --run` is a
required CI gate, so a component that needs a wrapping provider to render would
break it. There is no Zustand store for locale either, for the same reason there
is no resolver — nothing would read it yet.

### No new dependency

A single-locale catalog needs neither ICU plurals nor `select`. `next-intl` and
friends bring a lockfile change, and a `frontend/` lockfile change has broken the
Docker build before. When plural rules are genuinely needed they go behind the
same `t()` signature, which keeps this a swappable engine rather than a call-site
rewrite.

### Two pilot surfaces, not a migration

`layout.tsx` (server path: skip-link text, and `lang={DEFAULT_LOCALE}` so the
attribute stops being a second source of truth) and `MaintenanceBanner.tsx`
(client path). Legal pages, docs pages, admin screens, the editor, the onboarding
copy and `errorMessages.ts` are untouched. Migrating everything in the same change
would bury the design under a diff nobody can review, and the design is the part
that is expensive to retrofit.

## Considered Options

### Rejected: `[locale]` route segments

The conventional Next.js App Router answer: `src/app/[locale]/...`, locale
negotiated in `middleware.ts` and redirected into the path.

Rejected because **it would break every overlay that already exists**. An overlay
URL is not a page a user navigates to and forgets; it is configuration pasted into
an OBS browser source, into the browser extension, into a Stream Deck button, and
left there for months. Moving `/overlay/[id]` to `/en/overlay/[id]` invalidates all
of them at once, and the failure is silent and mid-broadcast. Redirects do not save
it either: they change the URL the streamer copied out, and OBS browser sources are
not a place anyone goes to fix a redirect.

It also splits every indexed public route into a canonical and a prefixed form, for
`sitemap.ts` and `robots.ts` to reconcile, in exchange for a single locale.

The consequence is stated as a rule, not a preference: **locale never goes in the
URL, now or later.** Locale #2 has to find a mechanism that leaves paths alone.

### Rejected: request-time resolution via `cookies()` / `headers()`

Read a cookie or `Accept-Language` in the root layout, pick a locale, pass it down.

Rejected because reading `cookies()` or `headers()` in the root layout makes
**every page dynamic** — the marketing homepage, the docs pages, every public route
that is currently static and indexed. With exactly one supported locale it buys
nothing at all: the answer is always `'en'`. It is an unconditional de-optimisation
of the whole app in exchange for a value that cannot vary.

So locale *selection* is deferred outright. There is no `middleware.ts`, no
`Accept-Language` parsing, no persisted account preference and no locale column.
A resolver nothing calls is untested code that looks load bearing.

### Rejected: extend `errorMessages.ts` into the catalog

It is the closest thing the repo had, and it is where the placeholder convention
comes from. But it is a `Record<ChatErrorType, ...>` keyed by an enum, whose values
carry `actionableSteps` arrays — a shape driven by error rendering, not by string
lookup. Bending the general catalog to fit it, or bending it to fit the catalog,
are both worse than migrating it on its own once the foundation is proven.

## Consequences

**Pinning the formatter is a visible behaviour change.** `MaintenanceBanner` dates
now read in English on every host, where before they followed the browser or OBS
locale. This is the intended fix; it is also the one thing in this change a user
could notice. Everything else renders byte-identically, em dash separators
included, pinned by `frontend/src/components/__tests__/MaintenanceBanner.test.tsx`.

**The `@ts-expect-error` line in `__tests__/translate.test.ts` is a load-bearing
gate, not a comment.** `frontend/tsconfig.json` includes `**/*.ts`, so test files
are type checked by `npx tsc --noEmit`. If `MessageKey` ever degrades to `string`,
those suppressions become unused and `tsc` fails. Verified by hand: adding
`| string` to `MessageKeyOf` produces three such errors.

**The remaining 37 unpinned `Intl` call sites are now inconsistent with this one.**
That is a deliberate scope line, not an oversight: each needs its own look at
whether the value is UI copy or viewer data. `format.ts` exists for them to move
onto.

**Adding locale #2 is now a bounded change**, and it needs its own decision about
static rendering — which is why that decision is not pre-empted here. The checklist
is in `docs/frontend/I18N.md`.

**Nothing lints for the AGPL header** that all 379 `.ts`/`.tsx` files under
`frontend/src` carry, so every new file in this module carries it by hand.

## Testing

`frontend/src/lib/i18n/__tests__/translate.test.ts` covers nested lookup,
interpolation, the same placeholder twice, an unknown key falling back to the key,
a namespace passed as a key, a missing param leaving the placeholder, and the
compile-time key check.

`frontend/src/lib/i18n/__tests__/format.test.ts` covers both formatters and, by
spying on the `Intl` constructors, that they are cached. Constructing a formatter
per render is the usual regression when code moves off a module-level constant, and
it is invisible in the output, so nothing but that assertion would catch it.

`frontend/src/components/__tests__/MaintenanceBanner.test.tsx` asserts the
in-progress copy, the scheduled range copy and the dismiss `aria-label` against an
oracle built from `Intl.DateTimeFormat('en', ...)` rather than from the component's
own formatter — so it is independent of the host time zone and reproduced the
original bug: under `LC_ALL=de_DE.UTF-8` it failed with the German month name
before the change and passes after it.

## References

- [docs/frontend/I18N.md](../frontend/I18N.md) — how to add a string, the
  namespace convention, and the checklist for locale #2.
- `frontend/src/lib/errorMessages.ts` — source of the `{platform}` placeholder
  convention and of the "designed for easy internationalization" comment this
  finally acts on. Not migrated here.
- `frontend/src/app/sitemap.ts`, `frontend/src/app/robots.ts` — the indexed
  unprefixed public routes that make locale routing unaffordable.
- `.github/workflows/frontend-a11y.yml` — the a11y ESLint and Storybook axe gates
  any component change keeps green, and the reason `useTranslations` is provider
  free.
