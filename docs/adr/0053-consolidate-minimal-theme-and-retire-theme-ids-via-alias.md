# ADR-0053: Consolidate the Minimal Clean theme; retire theme ids by alias, not deletion

**Date**: 2026-08-18
**Status**: Accepted
**Revised**: 2026-11-18
**Deciders**: caesarakalaeii

## Context

Two bundled themes shipped side by side: `minimal-theme` and
`minimal-theme-fixed`, the second being a bugfix fork of the first that never
superseded it. Each carried defects the other had fixed: the fixed fork did no
inline event rendering and hid the "Shared Chat" indicator, while the original
used a broken style for the readability outline and hard-coded away platform
status indicators. Both were the two most-used themes on the platform, so the
duplication mattered.

Consolidation is not just deleting a file. `overlay_configs.theme_id` stores
the id, and theme CSS is resolved from the frontend bundle by that id at render
time (deliberately, so a theme fix reaches every overlay on deploy): a dangling
id resolves to `''` and renders with **no theme CSS at all**. Deleting
`minimal-theme-fixed.css` outright would have silently un-themed roughly 115
overlays.

## Decision

**One theme. `minimal-theme` is canonical; `minimal-theme-fixed` is retired via
an alias, not deleted.**

1. `docs/overlay-themes/minimal-theme.css` absorbs the fork's genuine fixes
   (precise platform-badge selector, badge-image `:not()` guards) and keeps its
   own inline events. Version 2.0.0.
2. `docs/overlay-themes/minimal-theme-fixed.css` is deleted, so the id leaves
   the theme picker.
3. `bundled-themes.ts` gains `THEME_ALIASES` + `resolveThemeId()`;
   `getBundledTheme()` resolves through it. **Retiring a bundled theme id now
   means aliasing it.**
4. Migration `087_consolidate_minimal_theme.sql` rewrites stored ids.

The alias survives the migration: it covers rows written by a client still
holding the old id between deploy and migration, and keeps the change rollback
safe. Tests assert every alias target exists, that an alias is never itself a
bundled id, and that aliases never chain.

Two fixes along the way:

- **Readability outline**: the blocky edge was not ADR-0044's layered
  `text-shadow` but bad placement (mixing 1px/2px radii, diagonals at the full
  radius). The consolidated theme uses 12 shadows on a circle of radius 2.
  ADR-0044 stands unchanged: still layered `text-shadow`, never
  `-webkit-text-stroke` / `paint-order`.
- **Line Height did nothing** (reported on `minimal-theme`; affected every
  inline-layout theme): the row's strut took its `line-height` from Tailwind
  preflight (1.5), which no customizer rule touched. Fixed in `events.css` (both
  `.overlay-live-body` and `.overlay-preview-body` blocks) by driving the strut
  from `--chat-line-height`, with preflight's own `1.5` as fallback so an unset
  control renders identically to before.
- **Spacing precedence**: each customizer declaration carried the platform
  default as its `var()` fallback, so an unset control still outranked the
  theme (`(customizer OR platform default) > theme`). The fallback is now a
  theme-intent step:
  `padding: var(--chat-bubble-padding, var(--theme-bubble-padding, 0.75rem))`,
  giving **customizer setting > theme intent > platform default**. Themes opt
  in by declaring `--theme-*`; themes that declare nothing behave as before.
  Rule for the customizer layer: a `var()` fallback there is the *theme's*
  value, never the platform default directly, and adding a customizer property
  means adding its `--theme-*` step.

## Consequences

- One Minimal Clean theme to maintain; Line Height is symmetric for every
  theme.
- Overlays formerly on `minimal-theme-fixed` keep rendering and gain inline
  events and the correct outline; their platform status indicators are hidden
  by default (overridable — the theme reads `--chat-show-platform-indicators`
  instead of hard-coding `none`) and their events render inline instead of as
  gold cards.
- Overlays on `minimal-theme` get their "Shared Chat" indicator back.
- From now on, retiring any bundled theme id requires an alias entry; deleting
  a theme file alone is a data-integrity bug, not a cleanup.
