# ADR-0087: A site-wide light theme

**Date**: 2026-11-02
**Status**: Accepted
**Deciders**: caesarakalaeii

## Context and Problem Statement

The site was dark-only (ADR-0056: "a single authoritative dark-only token
system"). The legal pages carried a light-mode button, but it did nothing
visible: it set light values on the `--color-*` tokens inside
`#legal-wrapper`, and the legal pages had since moved onto the lanes art
direction (`.lanes-app`, `.lanes-panel`), which paints with hard-coded
`#000`, `#fff`, `#f4f3ef` and its own `--lanes-*` variables. Users asked for
a working light mode, across the whole site rather than only the legal pages.

The site paints with three color systems:

1. the design tokens (`--color-*` in `@theme`), used by shadcn primitives,
   the admin pages and most components;
2. the lanes art direction (homepage, product pages, legal, docs, blog),
   which used literal colors in `globals.css` and `border-white/20`-style
   utilities in markup;
3. broadcast art: OBS overlays (`/overlay/*`), the editor's preview iframes,
   chat-theme previews and the homepage's stream mockups, which show what
   viewers see on stream.

## Decision Drivers

- Nothing changes for anyone who does not opt in: dark stays the default.
- Broadcast art must never change with the site theme. An OBS browser
  source on a streamer's machine must render exactly as configured.
- No flash of the wrong theme on load.
- Every light text color clears WCAG 2.2 AA, locked by a test like the dark
  ones.
- One place per color decision, so a new color cannot be added dark-only by
  accident.

## Considered Options

1. **Light values for the existing tokens and role variables, switched by
   `data-theme="light"` on `<html>`.**
   - ✅ The 119 components on tokens switch with no markup change; the lanes
     CSS moves onto role variables whose dark values are its old literals,
     so dark mode is unchanged pixel for pixel.
   - ❌ Areas that must stay dark need the dark values restated (see below).
2. **CSS `light-dark()` per token, driven by `color-scheme`.**
   - ✅ Both values sit side by side in one declaration.
   - ❌ `color-scheme` on the root changes the canvas background, which is
     how an OBS browser source stays transparent; scoping it away from the
     overlay routes makes those routes resolve every token to its light
     value. Rejected.
3. **Invert the page with a CSS filter.**
   - ❌ Inverts images, emotes and brand colors and cannot be undone per
     element cleanly. Rejected.
4. **Follow the OS (`prefers-color-scheme`) by default.**
   - ❌ Would flip the site for every visitor on a light OS on their next
     visit. The maintainer chose dark by default with an explicit,
     remembered toggle.

## Decision Outcome

**Chosen**: Option 1.

- `lib/theme.ts` keeps the choice in `localStorage` (`allchat_theme`). An
  inline `<head>` script (`THEME_INIT_SCRIPT`, built from the same key and
  route rule) sets `data-theme="light"` before the first paint;
  `components/ThemeSync.tsx` keeps it right across client navigation.
- Unthemed routes: `/overlay/*` (OBS browser sources) and
  `/overlays/[id]/preview/*` (the editor's preview iframes) never get the
  attribute.
- `globals.css` restates the semantic tokens under
  `:root[data-theme='light']`. The shadcn aliases follow on their own because
  they are computed on the same element. Platform and status colors get
  darker light values so they stay AA as text.
- The lanes art direction paints with role variables on `:root`:
  `--lanes-fg`, `--lanes-fg-strong`, `--lanes-fg-dim`, `--lanes-page`,
  `--lanes-panel` and the five lane colors. Their dark values are the
  literals the pages were designed with. Markup uses them through
  `@theme inline` Tailwind colors (`border-lanes-fg-strong/20`,
  `bg-lanes-panel`, `text-lanes-fg`).
- `.theme-dark` marks an area that stays dark in either theme: chat-theme
  and credit-roll previews, the theme-showcase card, the homepage hero stage
  (its WebGL bands are translucent tints drawn from fixed colors and wash
  out over a light page) and the stream mockups. Custom properties inherit
  as computed values, so `.theme-dark` restates the dark values instead of
  pointing back at them; `token-contrast.test.ts` fails if the restatement
  misses a value the light theme changes or an alias computed from one.
- The toggle (`components/ThemeToggle.tsx`) sits in every site nav: AppNav,
  the homepage and extension-page navs, and the admin sidebar. It replaces
  the legal pages' own button, which is removed.
- Not premium-gated: it is a display preference and partly an
  accessibility one (maintainer decision), so it has no feature gate,
  onboarding entry or `/upgrade` row.

## Consequences

### Positive

- One switch for the whole site, remembered per browser, no flash.
- Dark mode unchanged except the new toggle, verified by pixel-diffing
  dark screenshots against `main`.
- Light text colors are AA-locked by the same test as dark ones.
- The dark-tuned raw palette text classes (`text-red-400` and friends) that
  failed on light backgrounds moved onto the status tokens, as ADR-0056
  already asked.

### Negative

- Every new color needs a light value, and every new broadcast-art area
  needs `.theme-dark`. A hard-coded `#fff` or `text-white` on a lanes page
  silently disappears in light mode.
- `.theme-dark` duplicates the dark token values. The test keeps the copy
  exact, but it is a second place to edit when a dark token changes.
- The server always renders dark; a light choice is applied by the head
  script, so `<html>` carries `suppressHydrationWarning`.
- Native form controls and scrollbars still follow the browser default
  (`color-scheme` is not set), as before.

## Related Decisions

- [ADR-0056](./0056-shadcn-token-vocabulary-as-the-design-system-contract.md):
  the token vocabulary this extends with a second set of values; no longer
  dark-only.
- [ADR-0053](./0053-consolidate-minimal-theme-and-retire-theme-ids-via-alias.md):
  overlay chat themes, the broadcast art the site theme must not touch.
