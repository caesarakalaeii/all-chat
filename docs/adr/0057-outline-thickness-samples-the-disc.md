# ADR-0057: A settable text outline samples the whole disc, not the compass

**Date**: 2026-09-03
**Status**: Accepted
**Deciders**: caesarakalaeii

## Context and Problem Statement

PR #833 made the overlay chat outline's thickness settable (1-6px, issue #832,
from streamer feedback asking for a heavier black edge). It shipped broken: at
the widths that were the whole point of the feature, chat rendered with legible
ghost copies of the text above and below the line and the background showing
through the gaps between them.

The cause is what a `text-shadow` layer actually is. A layer with zero blur
paints **one opaque copy of the glyph, translated by that layer's offset**. It
does not paint an outline. The outline is the *union* of the copies, so the
offsets have to be dense enough that the copies overlap, and close enough that
none of them detaches.

#833 sampled the **eight compass points at radius `w`**. At `w = 1` the copies
still touch and it looks like an outline, which is why the technique had been
fine as a fixed 1px preset for a year. From `w = 2` up it comes apart:

- the four diagonal copies sit at distance `w * sqrt(2)`, i.e. 4.24px out at
  `w = 3` and 8.49px at `w = 6`, well past the thickness the user asked for;
- between the eight copies there is nothing at all, so the "outline" is eight
  separated silhouettes rather than a band.

Its unit tests passed, and are the more interesting half of this record. They
asserted `emits eight layers, one per compass direction`, round-tripped the
value through the module's own parser, and pinned the colour. Every one of
those is satisfied by a value that renders as garbage: they described the
**shape of the string**, and the string was never the thing that was wrong.
They also never ran. See "Enforcement" below.

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

**The cost is accepted rather than engineered away.** Layer count is the disc's
area: 4 at 1px, 12 at 2px, 28, 48, 80, and 112 at 6px. Measured in headless
Chromium under a deliberately pessimistic load (40 rows of 16px bold text,
fully invalidated and repainted every frame, which is far worse than an overlay
repainting on a new message): everything through 4px / 48 layers stayed pinned
at the 16.7ms vsync floor, so it is free; 5px / 80 layers cost 21.1ms per frame
and 6px / 112 layers 25.7ms. The 1-6px range shipped in #833 is kept, and that
measurement is why it stops at 6.

**The width stays encoded in the declaration, but the declaration is re-derived
on the way out.** #833's choice to carry the width inside the `text-shadow`
string rather than in a sibling settings field is kept: it is what made the
feature need no migration, and it keeps one source of truth. What is added is
that `visual-settings-to-css.ts` passes the stored value through
`canonicalizeTextShadow` when emitting it, rewriting any recognised outline at
the same width in the current geometry. The width is the setting; the
declaration is only a rendering of it. So every overlay saved during the hours
#833 was live renders correctly on the next deploy, with no data migration and
without waiting for its owner to open the editor again.

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

Two gates, because two independent failures were needed to land this.

**1. A test of what the pixels do.** `text-outline.test.ts` rasterises the
offsets the way the browser does (union of translated copies) over glyph-like
probe shapes chosen for the features an outline gets wrong first: a lone pixel,
a 1px stem, a diagonal, two stems 4px apart, a ring with a counter. It then
asserts the two properties that *are* the definition of an outline:

- **reach**: nothing is painted farther than `w` from the text. This is the
  ghost text in the bug report.
- **solidity**: everything within `w` of the text is painted. This is the
  background showing through.

Both are exact for a disc of integer offsets, so neither needs a tolerance. A
further test re-runs both checks against the exact string #833 shipped at 3px
and requires them to **fail** on it, so the gate cannot pass vacuously.

This is a rasterisation in TypeScript rather than a browser screenshot on
purpose. `flake.nix` records that Playwright's downloaded browsers cannot run
on a NixOS dev host (no host `ldso`) and that the nixpkgs alternative has a
2.3GB closure, so a screenshot gate would be runnable **only in CI**, which is
the same "nobody can check this locally" shape as the bug. The correspondence
between the rasterisation and real rendering was verified by hand against
system Chromium while fixing this, in both directions: the disc renders as a
clean outline at every width, and the compass reproduces the reported artefact.

**2. The frontend unit project now runs on a pull request.** It did not. The
required contexts on `main` are `test (<go service>)`, `test-node`
(support-bot only) and `frontend-a11y.yml`'s three jobs, and the only vitest
steps among them named three files by path. So #833's 97 lines of unit test were
never executed by CI, and a correct test would not have blocked the merge
either. `a11y-static` gained a `npx vitest --project unit --run` step; it is
already a required context, so this needs no branch-protection change. The
existing design-system steps are kept alongside it for failure attribution.

## Consequences

- The outline thickens smoothly and cleanly across 1-6px, verified visually in
  Chromium against the reported case.
- Overlays saved with the broken value are fixed by deploy, not by migration.
  Nothing rewrites a stored row, so a rollback is still just a rollback.
- A 6px outline is ~3KB of CSS in `visual_settings`. That column is free-form
  JSONB with no per-field validation, it is written once per overlay rather
  than per message, and the only sanitiser it passes (`hasBalancedParens`) is
  indifferent to length.
- At 6px on 16px text the outlines of adjacent words merge into a black band.
  That is the geometry doing what it says, not a defect, and it is the visual
  reason the range does not go higher.
- New rule for this module: a test that asserts the shape of a generated CSS
  string is not a test of the thing the user sees. Assert the rendered
  geometry.
