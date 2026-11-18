# ADR-0087: A site-wide light theme

**Date**: 2026-11-02
**Status**: Accepted
**Deciders**: caesarakalaeii

## Context

The site was dark-only (ADR-0056) and the legal pages' light-mode button did nothing
visible: those pages had moved onto the lanes art direction (`.lanes-app`,
`.lanes-panel`), which painted with literal `#000`/`#fff` colors and its own `--lanes-*`
variables. Users asked for a working light mode across the whole site. The site paints
with three color systems: the design tokens (`--color-*` in `@theme`, used by shadcn
primitives, the admin pages and most components), the lanes art direction (homepage,
product pages, legal, docs, blog), and broadcast art (OBS overlays, preview iframes,
chat-theme previews, stream mockups) that shows what viewers see on stream. Constraints:
dark stays the default; broadcast art never changes with the site theme; no flash of the
wrong theme on load; light text is AA-locked by a test; one place per color decision.

## Considered options

- Light values for the existing tokens switched by `data-theme="light"` (chosen): the
  token-consuming components switch with no markup change; the lanes CSS moves onto role
  variables whose dark values are the old literals, so dark mode is pixel-identical. Cost:
  areas that must stay dark need the dark values restated.
- CSS `light-dark()` per token: rejected; `color-scheme` on the root changes the canvas
  background (how an OBS browser source stays transparent), and scoping it away from
  overlay routes would resolve every token to its light value there.
- Invert with a CSS filter: rejected; inverts images and brand colors, unclean per
  element.
- Follow `prefers-color-scheme` by default: rejected; the maintainer chose an explicit,
  remembered dark default.

## Decision

`lib/theme.ts` keeps the choice in `localStorage` (`allchat_theme`); an inline head script
(`THEME_INIT_SCRIPT`) sets `data-theme="light"` before first paint and
`components/ThemeSync.tsx` maintains it across navigation. `/overlay/*` (OBS) and
`/overlays/[id]/preview/*` (preview iframes) never get the attribute. `globals.css`
restates the semantic tokens under `:root[data-theme='light']` (shadcn aliases follow
since they compute on the same element; platform and status colors get darker light
values for AA). The lanes art direction paints with role variables on `:root`
(`--lanes-fg`, `--lanes-fg-strong`, `--lanes-fg-dim`, `--lanes-page`, `--lanes-panel`,
five lane colors), their dark values being the original literals, consumed via
`@theme inline` Tailwind colors. `.theme-dark` marks areas that stay dark in either
theme (chat-theme and credit-roll previews, theme-showcase card, homepage hero stage,
stream mockups); it restates the dark values rather than aliasing them, and
`token-contrast.test.ts` fails on any missed value. The toggle
(`components/ThemeToggle.tsx`) sits in every site nav (AppNav, homepage and extension
navs, admin sidebar) and replaces the legal pages' button. Not premium-gated: a display
preference with no feature gate, onboarding entry or `/upgrade` row.

## Consequences

- One remembered, flash-free switch; dark mode pixel-unchanged apart from the toggle
  (verified by dark screenshot pixel-diff); light AA locked by the same test as dark; the
  dark-tuned raw palette text classes (`text-red-400` etc.) moved onto status tokens.
- Every new color needs a light value and every new broadcast area needs `.theme-dark`; a
  hard-coded `#fff` on a lanes page silently disappears in light mode. `.theme-dark`
  duplicates the dark values (locked exact by the test). The server renders dark and
  `<html>` carries `suppressHydrationWarning`; form controls and scrollbars still follow
  the browser default.
