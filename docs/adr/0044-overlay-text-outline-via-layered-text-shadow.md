# ADR-0044: Overlay Chat Text Outlines Use Layered Text-Shadow, Not -webkit-text-stroke

**Date**: 2026-07-24
**Status**: Accepted
**Deciders**: caesarakalaeii

## Context

Chat text on an overlay sits on top of unpredictable live video, so usernames and message text need a dark edge. Two techniques had accumulated: a layered `text-shadow` outline (multiple hard offset shadows in 4 or 8 directions) in some themes, and `-webkit-text-stroke` + `paint-order: stroke fill` in others (minimal 3px, comic-speech 1px, neo-brutalist 2px, plus a 0.5px stroke on gradient usernames). External feedback (2026-07): the stroke "looks very off and not clean". It is: `-webkit-text-stroke` centres the stroke on the glyph path, so half the width eats into the glyph fill — at 3px it erodes thin strokes, closes counters and chunks corners, worst on small text and thin fonts; on gradient usernames (transparent fill + `background-clip: text`) it muddied the colours, which is why an ad-hoc override existed at all. The minimal theme's own comment even records that its stroke had replaced an earlier text-shadow, i.e. the two approaches had been swapped back and forth.

## Decision

**Standardise on layered `text-shadow` for readability outlines; do not use `-webkit-text-stroke` (or `paint-order: stroke fill`) for text legibility.** A layered `text-shadow` paints behind the glyphs, so it never erodes the fill and renders correctly behind gradient-filled text.

- The three stroke themes migrated to equivalent-weight layered outlines (comic-speech: 4-direction 1px; minimal and neo-brutalist: 8-direction 2px), preserving intended boldness. Themes whose `text-shadow` is an intentional glow (cyberpunk, neon-glass, vaporwave) are unchanged.
- Gradient usernames use a layered drop shadow instead of the 0.5px stroke, in `globals.css` and the inline style in both overlay renderers.
- Themes are authored in `docs/overlay-themes/*.css` and bundled into `frontend/src/lib/theme-marketplace/bundled-themes.generated.ts` (`npm run generate:themes`), so the fix reaches every overlay on deploy.
- A regression test (`bundled-themes-outline.test.ts`) asserts no bundled theme's active CSS contains `-webkit-text-stroke` or `paint-order: stroke`.

## Consequences

- Usernames and message text read cleanly over video at all sizes.
- New themes must use layered `text-shadow` for outlines; `text-shadow` for deliberate glow/neon effects remains fine — the ban is specifically on stroke as a legibility outline.
- Slightly more shadow layers per glyph than a single stroke; negligible at chat volumes.
