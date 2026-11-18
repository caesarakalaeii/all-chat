# ADR-0057: A settable text outline samples the whole disc, not the compass

**Date**: 2026-09-03
**Status**: Accepted
**Revised**: 2026-11-18
**Deciders**: caesarakalaeii

## Context

PR #833 made the overlay chat outline's thickness settable (1-6px) and shipped
broken: at the widths that were the point, chat rendered with ghost copies of
the glyph above and below the line and background showing through the gaps.

Cause: a zero-blur `text-shadow` layer paints **one opaque glyph copy
translated by that layer's offset**; the "outline" is the union of the copies.
#833 sampled eight compass points at radius `w`, which works only at `w = 1`;
the four diagonals sit at `w * sqrt(2)` (8.49px out at `w = 6`), and between
the eight copies there is nothing, so the result is eight silhouettes, not a
band. Its unit tests passed because they asserted the *shape of the string*
the module emitted, and never ran in CI (see the Enforcement fix below).

## Decision

**The offsets are every integer point inside the disc of radius `w`.** That
makes the painted region the glyph dilated by that disc, which is the
definition of an outline of thickness `w`.

Integer spacing is load-bearing: neighbouring copies are 1px apart and any
glyph feature the eye can resolve is at least a pixel across, so consecutive
copies always overlap. Sub-sampling is tempting and does cut the layer count,
but a half-lattice opens hairline gaps on thin fonts, and a bare ring (offsets
at radius `w` only) leaves a hole around 1px features: a period, or the dot of
an `i`, becomes a black annulus with the background inside it.

`docs/overlay-themes/*.css` keeps its 12-point **ring** at a fixed radius 2
(ADR-0053) and does not need changing. A ring is dense enough when the radius
is fixed and small; a control that goes to 6px is neither.

**The cost is accepted rather than engineered away.** Layer count is the
disc's area (4 at 1px up to 112 at 6px). Measured in headless Chromium under a
deliberately pessimistic load, everything through 4px stayed pinned at the
16.7ms vsync floor; 5px cost 21.1ms and 6px 25.7ms per frame — which is why
the range stops at 6.

**The width stays encoded in the declaration but is re-derived on the way
out.** Carrying it inside the `text-shadow` string (not a sibling settings
field) kept the feature migration-free with one source of truth;
`visual-settings-to-css.ts` additionally passes the stored value through
`canonicalizeTextShadow`, rewriting any recognised outline at the same width
in the current geometry. The width is the setting; the declaration is only a
rendering of it, so every overlay saved while the broken version was live
renders correctly on the next deploy.

`parseOutlineWidth` matches on the offsets it **decodes** rather than on the
whole string, and recognises both historic samplings (#833's compass, and the
four-diagonal 1px form that predates it). Whole-string equality was fragile in
a way that would have surfaced eventually anyway: a value that has been through
any CSS serialiser comes back respaced and reordered, and would then have
demoted the picker to its read-only "Custom" entry and lost the slider.

ADR-0044 is unchanged and still governs the technique: `-webkit-text-stroke`
and `paint-order: stroke fill` remain banned as legibility outlines, so the
answer to "make it thicker" is more layers, not a stroke.

## Enforcement

- `text-outline.test.ts` rasterises the offsets the way the browser does
  (union of translated copies) over glyph-like probes (lone pixel, 1px stem,
  diagonal, two stems 4px apart, a ring with a counter) and asserts exactly
  the two properties that are the definition of an outline: **reach** (nothing
  painted farther than `w`) and **solidity** (everything within `w` painted);
  a separate test requires the exact string #833 shipped to fail both, so the
  gate cannot pass vacuously. It is a TypeScript rasterisation rather than a
  screenshot because Playwright browsers cannot run on a NixOS dev host; the
  rasterisation was hand-verified against system Chromium in both directions.
- The frontend unit project now runs on a pull request: `a11y-static` (already
  required) gained `npx vitest --project unit --run`. #833's unit tests were
  never executed by CI, so the correct tests never blocked the merge.


## Consequences

- The outline thickens cleanly across 1-6px; overlays saved with the broken
  value are fixed by deploy, not by migration (a rollback stays just a
  rollback).
- A 6px outline is ~3KB of CSS in `visual_settings` (free-form JSONB, written
  once per overlay). At 6px on 16px text the outlines of adjacent words merge
  into a black band: the geometry doing what it says, and the visual reason
  the range stops at 6.
- New rule for this module: a test of a generated CSS string's shape is not a
  test of what the user sees; assert the rendered geometry.
