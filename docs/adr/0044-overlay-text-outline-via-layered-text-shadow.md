# ADR-0044: Overlay chat text outlines use layered text-shadow, not -webkit-text-stroke

**Date**: 2026-07-24
**Status**: Accepted
**Deciders**: caesarakalaeii

## Context and Problem Statement

Chat text on an overlay sits on top of unpredictable live video, so usernames and
message text need a dark edge to stay legible. Two techniques had accumulated:

- A **layered `text-shadow`** outline (multiple hard offset shadows in 4 or 8
  directions), used by e.g. the high-contrast and minecraft themes.
- A **`-webkit-text-stroke` + `paint-order: stroke fill`** outline, used by the
  minimal theme family (`3px #000`), comic-speech (`1px`) and neo-brutalist
  (`2px`), plus a `0.5px` stroke on gradient usernames (`globals.css` and an
  inline `ref` in both overlay renderers).

External feedback (2026-07): the stroke *"looks very off and not clean."* It is:
`-webkit-text-stroke` centres the stroke on the glyph path, so half the width
eats **into** the glyph fill. At 3px this erodes thin strokes, closes counters,
and chunks corners — worst on small text and thin fonts. On gradient usernames
(transparent fill + `background-clip: text`) the stroke also muddied the colours,
which is why a separate ad-hoc stroke override existed at all. The minimal theme's
own comment even records that its stroke had *replaced* an earlier "12-layer
text-shadow" — i.e. the two approaches had been swapped back and forth.

## Decision

**Standardise on layered `text-shadow` for readability outlines; do not use
`-webkit-text-stroke` (or `paint-order: stroke fill`) for text legibility.**

- A layered `text-shadow` is painted **behind** the glyphs, so it never erodes
  the fill — corners stay clean and thin strokes survive. It also renders
  correctly behind gradient-filled (transparent) text.
- Migrated the three stroke themes to an equivalent-weight layered outline
  (comic-speech → 4-direction 1px; minimal + neo-brutalist → 8-direction 2px),
  preserving each theme's intended boldness. Themes whose `text-shadow` is an
  intentional **glow** (cyberpunk, neon-glass, vaporwave) are unchanged.
- Gradient usernames now use a clean layered drop shadow
  (`0 1px 2px rgba(0,0,0,.65), 0 0 2px rgba(0,0,0,.5)`) in both `globals.css` and
  the inline `ref` in the live overlay + preview embed, replacing the `0.5px`
  stroke.
- Theme CSS is authored in `docs/overlay-themes/*.css` and bundled into
  `frontend/src/lib/theme-marketplace/bundled-themes.generated.ts`
  (`npm run generate:themes`), so the fix reaches every overlay on deploy.

A regression test (`bundled-themes-outline.test.ts`) asserts no bundled theme's
*active* CSS (comments stripped) contains `-webkit-text-stroke` or
`paint-order: stroke`.

## Consequences

- Usernames and message text read cleanly over video at all sizes; the reported
  "off / not clean" outline is gone (verified visually before/after).
- New themes must use layered `text-shadow` for outlines. `text-shadow` for
  deliberate glow/neon effects remains fine — the ban is specifically on stroke
  used as a legibility outline.
- Slightly more shadow layers per glyph than a single stroke; negligible at chat
  volumes and worth the crispness.
