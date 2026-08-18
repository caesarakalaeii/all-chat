# ADR-0053: Consolidate the Minimal Clean theme; retire theme ids by alias, not deletion

**Date**: 2026-08-18
**Status**: Accepted
**Deciders**: caesarakalaeii

## Context and Problem Statement

Two bundled themes shipped side by side:

- `minimal-theme` — "Minimal Clean Theme"
- `minimal-theme-fixed` — "Minimal Clean Theme (Fixed Platform & Badges)"

The second was a **bugfix fork of the first**. It was created to fix the platform
icon / badge layout and then shipped *alongside* the theme it was meant to
replace, instead of superseding it. Both stayed selectable, and each then drifted:

| | `minimal-theme` | `minimal-theme-fixed` |
|---|---|---|
| Inline event rendering (`--event-*`) | yes | **no** — events fell back to the default gold card |
| Platform text badge hidden via | `.text-xs.font-semibold.uppercase` — **also matched the "Shared Chat" indicator and hid it** | `.platform-badge.platform-badge-text` (correct) |
| Badge-image `:not()` guards | **missing** | present |
| Readability outline | 8-direction, diagonals at full 2px (2.83px corners → octagonal) | 12 layers mixing 1px and 2px radii → **visibly blocky**, reported by a user |
| Platform status indicators | `display: none` hard-coded, **ignoring the customizer toggle** | not hidden |

So neither was the good one; each carried defects the other had fixed. Together
they were also the **two most-used themes on the platform** (115 + 107 overlays,
~72% of all themed overlays), so the duplication was not a niche problem.

A user on `minimal-theme` additionally reported that **Line Height did nothing**.
That turned out to be a separate, deeper bug — see "Line Height" below.

The blocking question for consolidation: `overlay_configs.theme_id` stores the
id, and theme CSS is resolved from the frontend bundle **by that id at render
time** (deliberately, so a theme fix reaches every overlay on deploy without a
per-overlay data migration). That design makes a dangling id *fatal*: an overlay
whose `theme_id` matches no bundled theme resolves to `''` and renders with **no
theme CSS at all**. Simply deleting `minimal-theme-fixed.css` would have silently
un-themed 115 overlays.

## Decision

**One theme. `minimal-theme` is canonical; `minimal-theme-fixed` is retired via an
alias, not deleted.**

1. `docs/overlay-themes/minimal-theme.css` absorbs the fork's genuine fixes
   (precise platform-badge selector, badge-image `:not()` guards) and keeps its
   own (inline events). Version 2.0.0.
2. `docs/overlay-themes/minimal-theme-fixed.css` is deleted, so the id leaves the
   theme picker.
3. `bundled-themes.ts` gains `THEME_ALIASES` + `resolveThemeId()`; `getBundledTheme()`
   resolves through it. **Retiring a bundled theme id now means aliasing it.**
4. Migration `087_consolidate_minimal_theme.sql` rewrites stored ids.

The alias is kept **after** the migration rather than replaced by it: it covers
rows written by a client still holding the old id between deploy and migration,
and it keeps the change safe to roll back. Tests assert every alias target exists,
that an alias is never itself a bundled id, and that aliases never chain.

### Readability outline

The blocky edge was *not* caused by ADR-0044's choice of layered `text-shadow` —
it was caused by placing the layers badly. Mixing 1px and 2px radii, and putting
diagonals at the full radius (so corners reach `r√2`), makes the outline lumpy.
The consolidated theme uses **12 shadows on a circle of radius 2** (every 30°,
diagonals scaled by cos/sin). Same weight, even edge. ADR-0044 stands unchanged:
still layered `text-shadow`, still no `-webkit-text-stroke` / `paint-order`.

### Line Height (the reported bug)

Independent of the consolidation, and it affected **every inline-layout theme**,
not just this one.

The minimal family renders a message inline (`username: message`), so the whole
row is a single line box inside the block content container (`.min-w-0.flex-1`).
A line box can never be shorter than its block container's **strut**, and the
strut takes its `line-height` from Tailwind preflight (`1.5`) — which no
customizer rule touched. `--chat-line-height` therefore shrank the inline spans
while the strut held the row at 24px: the control could **loosen rows but never
tighten them**. Setting Line Height to 1 measurably did nothing.

Fixed in `events.css` by driving the strut from the same variable, in both the
`.overlay-live-body` and `.overlay-preview-body` blocks:

```css
.overlay-live-body .min-w-0.flex-1 {
  line-height: var(--chat-line-height, 1.5) !important;
}
```

The `1.5` fallback is preflight's own value, so an *unset* Line Height renders
byte-identically to before — consistent with the rule that block already
documents: an unset control produces no visible change, a set control applies.

### Customizer / theme / platform precedence (the rest of the spacing bug)

Fixing the strut was not enough: the rows were still far too tall. The dominant
cost was **bubble padding the theme had explicitly deleted**.

The visual-customizer rules are `!important` inside `@layer visual-customizer`,
which beats a theme's *unlayered* `!important`. That is correct for a control the
user actually set. The defect was that each declaration also carried the
**platform default as its `var()` fallback**:

```css
padding: var(--chat-bubble-padding, 0.75rem) !important;   /* before */
```

so with the control UNSET, `0.75rem` still outranked the theme. Minimal is a
no-bubble theme asking for `padding: 0`, and got 12px on every row anyway — 24px
of vertical space per message that the theme had tried to remove, plus forced
`border-radius` and a forced `backdrop-filter: blur(4px)` on transparent rows.
Effective precedence was `(customizer OR platform default) > theme`.

The fallback is now a **theme-intent step**:

```css
padding: var(--chat-bubble-padding, var(--theme-bubble-padding, 0.75rem)) !important;
```

giving the intended order: **customizer setting > theme intent > platform
default**. A theme opts in by declaring `--theme-*` (`--theme-bubble-padding`,
`--theme-bubble-border-radius`, `--theme-bubble-border-width`,
`--theme-backdrop-blur`, `--theme-message-gap`); a theme that declares nothing
behaves exactly as before, so this is backwards-compatible across the bundle.

Measured on the reporter's saved settings (`lineHeight: 1`, `messageGap: 8px`),
row pitch went **48px → 24px** — which is now exactly `1 × 16px + 8px`, i.e. the
two controls finally produce the number they describe.

**New rule for the customizer layer**: a `var()` fallback in these blocks is the
*theme's* value, never the platform default directly. Adding a new customizer
property means adding its `--theme-*` step too.

## Consequences

- Line Height is symmetric for every theme; overlays can be made denser.
- One Minimal Clean theme to maintain. The picker loses an entry (16 themes).
- The 115 overlays on `minimal-theme-fixed` keep rendering, and gain inline
  events + the correct outline. Two visible changes for them: events render
  inline instead of as gold cards (the theme's actual premise), and platform
  status indicators are hidden by default — now overridable, since the theme
  reads `--chat-show-platform-indicators` instead of hard-coding `none`.
- The 107 overlays on `minimal-theme` get their "Shared Chat" indicator back.
- **Retiring any bundled theme id from now on requires an alias entry**; deleting
  a theme file on its own is a data-integrity bug, not a cleanup.
