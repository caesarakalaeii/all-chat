# ADR-0078: Homepage Language Switch, and German Opened to Everyone

**Date**: 2026-10-19
**Status**: Accepted
**Deciders**: caesarakalaeii

## Context and Problem Statement

ADR-0066 put the language picker on `/settings` only, behind the `ui_languages`
feature gate, which share-service checks on `GET /api/v1/ui-languages` for the
signed-in user. That leaves the people a second language helps most, first-time
visitors deciding whether to sign up, with no way to pick German: they are
signed out, so the probe can never admit them, and they never see `/settings`.

ADR-0066 also expected the homepage to stay English because `app/page.tsx` is a
Server Component. In practice the page body is `HomeClient` and its sections,
which read copy through `useTranslations()`, so they re-render in the chosen
language like any other client surface.

## Decision Drivers

- A visitor should be able to read the homepage in German before signing in.
- One answer to "who may pick German", not one per surface.
- Locale still never enters the URL, and nothing reads request state (ADR-0066).

## Considered Options

1. **A switch in the homepage nav, not gated, with German opened to everyone by
   turning off `early_access` on the `ui_languages` gate (chosen).**
2. A switch that probes the gate like the `/settings` card. Correct, but signed
   out visitors always fail the probe, so it would show nothing to the audience
   it is for.
3. A switch that is not gated on the homepage while the gate stays beta-only.
   The homepage and `/settings` would then disagree on who may pick German.

## Decision Outcome

Option 1.

- `components/home/LanguageToggle.tsx` renders one button per entry in
  `SUPPORTED_LOCALES` at the right end of the lanes nav, and calls
  `chooseLocale()` like the settings card. It makes no request. Below 480px the
  nav's "themes" link is hidden to make room; `/docs` links to it.
- The `ui_languages` gate is flipped to `early_access=FALSE` in the admin UI,
  so the `/settings` card offers German to every signed-in user too.

### Consequences

- Good: German reaches signed-out visitors, and the homepage switch and
  `/settings` agree.
- Bad: the switch lists every supported locale and does not check the gate.
  A later language that should stay in beta needs the switch filtered first,
  and closing the gate again would hide German on `/settings` only.
- Neutral: the parts ADR-0066 lists as English stay English: `metadata`, the
  JSON-LD, Server Components and `legal.*`.

## Related Decisions

- [ADR-0066](./0066-client-side-ui-locale-selection.md): per-browser locale
  choice, the `ui_languages` gate. This record amends where the picker lives
  and who the gate admits.
- [ADR-0008](./0008-feature-gate-infrastructure.md): feature gates.
