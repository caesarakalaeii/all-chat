/**
 * This file is part of All-Chat.
 * Copyright (C) 2026 caesarakalaeii
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program. If not, see <https://www.gnu.org/licenses/>.
 */

import { describe, it, expect } from 'vitest'
import {
  buildOutlineShadow,
  canonicalizeTextShadow,
  outlineOffsets,
  parseOutlineWidth,
  MIN_OUTLINE_WIDTH_PX,
  MAX_OUTLINE_WIDTH_PX,
  OUTLINE_COLOR,
} from '../text-outline'

const SOFT_PRESET = '0 1px 2px rgba(0, 0, 0, 0.6)'
const STRONG_PRESET = '0 1px 2px rgba(0, 0, 0, 0.9), 0 2px 6px rgba(0, 0, 0, 0.7)'

// The two samplings real overlays have stored, spelled out rather than
// imported: these are strings in the database, so the test has to fail if the
// module stops recognising them. Comparing against a value the module built
// would only prove the module agrees with itself.
//
// PRE_833 is the four-diagonal 1px outline the editor hard-coded before the
// width was settable. GHOSTED_833 is what #833 emitted at 3px — eight compass
// points at radius 3 — which rendered as separated copies of the text instead
// of an outline, and is the bug this module exists to have fixed.
const PRE_833_1PX =
  '1px 1px 0 rgba(0, 0, 0, 0.85), -1px 1px 0 rgba(0, 0, 0, 0.85), 1px -1px 0 rgba(0, 0, 0, 0.85), -1px -1px 0 rgba(0, 0, 0, 0.85)'
const GHOSTED_833_3PX =
  '3px 0px 0 rgba(0, 0, 0, 0.85), 0px 3px 0 rgba(0, 0, 0, 0.85), -3px 0px 0 rgba(0, 0, 0, 0.85), 0px -3px 0 rgba(0, 0, 0, 0.85), 3px 3px 0 rgba(0, 0, 0, 0.85), -3px 3px 0 rgba(0, 0, 0, 0.85), -3px -3px 0 rgba(0, 0, 0, 0.85), 3px -3px 0 rgba(0, 0, 0, 0.85)'

const WIDTHS = Array.from(
  { length: MAX_OUTLINE_WIDTH_PX - MIN_OUTLINE_WIDTH_PX + 1 },
  (_, i) => MIN_OUTLINE_WIDTH_PX + i
)

/*
 * ---------------------------------------------------------------------------
 * The rendering gate
 * ---------------------------------------------------------------------------
 * #833 shipped an outline that rendered as eight legible ghost copies of the
 * chat text, and its tests passed — because they asserted the SHAPE OF THE
 * STRING ("emits eight layers, one per compass direction") and round-tripped
 * the value through the module's own parser. Both are satisfied by a value
 * that renders as garbage. So the checks below assert what the pixels do
 * instead, and they are the reason this file exists.
 *
 * A zero-blur `text-shadow` layer paints one opaque copy of the glyph shifted
 * by that layer's offset, so the rendered outline is the union of those
 * copies: `rasterize` reproduces exactly that, over glyph-like probe shapes,
 * and the two properties below are what "an outline of thickness w" means.
 *
 *   reach    — nothing is painted farther than w from the text. Violated by
 *              detached copies: this is the ghost text in the bug report.
 *   solidity — everything within w of the text IS painted. Violated by any
 *              sampling with holes in it: the background shows through.
 *
 * Both are exact for a disc of integer offsets, so neither needs a tolerance.
 */

type Pixel = readonly [x: number, y: number]

const key = ([x, y]: Pixel): string => `${x},${y}`

/** Glyph-like shapes, chosen for the features an outline gets wrong first. */
const PROBE_SHAPES: ReadonlyArray<{ name: string; pixels: readonly Pixel[] }> = [
  // The dot of an `i`, or a period: a lone pixel is where a sampling that
  // skips the disc's interior leaves a visible hole.
  { name: 'single pixel', pixels: [[0, 0]] },
  // A 1px stem, as in `l` — the thinnest feature real chat text has.
  { name: 'thin vertical stem', pixels: Array.from({ length: 12 }, (_, i): Pixel => [0, i]) },
  { name: 'thin horizontal bar', pixels: Array.from({ length: 12 }, (_, i): Pixel => [i, 0]) },
  // A diagonal, as in `/`: neighbouring pixels touch only at their corners.
  { name: 'diagonal', pixels: Array.from({ length: 10 }, (_, i): Pixel => [i, i]) },
  // Two stems a few pixels apart, as in `ll` — the gap between glyphs.
  {
    name: 'two stems 4px apart',
    pixels: Array.from({ length: 12 }, (_, i): Pixel => [0, i]).concat(
      Array.from({ length: 12 }, (_, i): Pixel => [4, i])
    ),
  },
  // A counter, as in `o`: the outline fills it from the inside too.
  {
    name: 'ring with a counter',
    pixels: (() => {
      const out: Pixel[] = []
      for (let y = 0; y < 9; y++)
        for (let x = 0; x < 9; x++) if (x === 0 || y === 0 || x === 8 || y === 8) out.push([x, y])
      return out
    })(),
  },
]

/** The union of one copy of `shape` per offset — what the browser paints. */
function rasterize(shape: readonly Pixel[], offsets: readonly Pixel[]): Set<string> {
  const painted = new Set<string>()
  for (const [ox, oy] of offsets) for (const [sx, sy] of shape) painted.add(key([sx + ox, sy + oy]))
  return painted
}

/** Squared distance from `p` to the nearest pixel of `shape`. */
function nearestSquared(p: Pixel, shape: readonly Pixel[]): number {
  let best = Infinity
  for (const [sx, sy] of shape) {
    const dx = p[0] - sx
    const dy = p[1] - sy
    best = Math.min(best, dx * dx + dy * dy)
  }
  return best
}

const toPixel = (k: string): Pixel => {
  const [x, y] = k.split(',').map(Number)
  return [x, y]
}

/** Painted pixels farther than `width` from the text: detached ghost copies. */
function outOfReach(shape: readonly Pixel[], offsets: readonly Pixel[], width: number): Pixel[] {
  const shapeKeys = new Set(shape.map(key))
  return [...rasterize(shape, offsets)]
    .map(toPixel)
    .filter((p) => !shapeKeys.has(key(p)) && nearestSquared(p, shape) > width * width)
}

/** Pixels within `width` of the text that nothing paints: gaps in the band. */
function unpainted(shape: readonly Pixel[], offsets: readonly Pixel[], width: number): Pixel[] {
  const shapeKeys = new Set(shape.map(key))
  const painted = rasterize(shape, offsets)
  const xs = shape.map(([x]) => x)
  const ys = shape.map(([, y]) => y)
  const pad = width + 2
  const holes: Pixel[] = []
  for (let y = Math.min(...ys) - pad; y <= Math.max(...ys) + pad; y++) {
    for (let x = Math.min(...xs) - pad; x <= Math.max(...xs) + pad; x++) {
      const p: Pixel = [x, y]
      if (shapeKeys.has(key(p))) continue
      if (nearestSquared(p, shape) <= width * width && !painted.has(key(p))) holes.push(p)
    }
  }
  return holes
}

/**
 * Splits a shadow list into layers. The lookahead requires a `px` length, not
 * just a digit: every layer's `rgba()` colour carries commas followed by bare
 * numbers of its own, and splitting on those instead is a trap this file fell
 * into once already.
 */
const splitLayers = (value: string): string[] => value.split(/,\s*(?=-?\d+px)/)

/** Reads the offsets back out of a `text-shadow` string. */
function parseShadowOffsets(value: string): Pixel[] {
  return [...value.matchAll(/(-?\d+)px\s+(-?\d+)px\s+0(?:px)?\s+rgba\([^)]*\)/g)].map(
    (m): Pixel => [Number(m[1]), Number(m[2])]
  )
}

describe('the rendered outline', () => {
  it.each(WIDTHS)('paints nothing farther than %dpx from the text', (width) => {
    for (const { name, pixels } of PROBE_SHAPES) {
      const ghosts = outOfReach(pixels, outlineOffsets(width), width)
      expect(ghosts, `${name}: ${ghosts.length} pixel(s) painted beyond ${width}px`).toEqual([])
    }
  })

  it.each(WIDTHS)('leaves no gap inside the %dpx band around the text', (width) => {
    for (const { name, pixels } of PROBE_SHAPES) {
      const holes = unpainted(pixels, outlineOffsets(width), width)
      expect(holes, `${name}: ${holes.length} unpainted pixel(s) within ${width}px`).toEqual([])
    }
  })

  // Without this, the two checks above could both be passing vacuously. This is
  // the exact value #833 shipped at 3px, and both checks fail on it: the
  // compass sampling puts its diagonal copies 3*sqrt(2) ~ 4.24px out, and
  // leaves the band between the copies unpainted.
  it('would have rejected the sampling that shipped in #833', () => {
    const compass = parseShadowOffsets(GHOSTED_833_3PX)
    const stem = PROBE_SHAPES[1].pixels
    expect(outOfReach(stem, compass, 3).length).toBeGreaterThan(0)
    expect(unpainted(stem, compass, 3).length).toBeGreaterThan(0)
  })

  it('reaches the full chosen thickness, and no further', () => {
    for (const width of WIDTHS) {
      const radii = outlineOffsets(width).map(([x, y]) => Math.hypot(x, y))
      expect(Math.max(...radii)).toBeLessThanOrEqual(width)
      expect(Math.max(...radii)).toBeGreaterThan(width - 1)
    }
  })

  it('thickens monotonically, every width containing the one below', () => {
    for (const width of WIDTHS.slice(1)) {
      const thinner = new Set(outlineOffsets(width - 1).map(key))
      const thicker = new Set(outlineOffsets(width).map(key))
      expect([...thinner].every((k) => thicker.has(k))).toBe(true)
      expect(thicker.size).toBeGreaterThan(thinner.size)
    }
  })
})

describe('buildOutlineShadow', () => {
  it('emits a well-formed, hard-edged, uniformly black shadow list', () => {
    for (const width of WIDTHS) {
      const layers = splitLayers(buildOutlineShadow(width))
      expect(layers).toHaveLength(outlineOffsets(width).length)
      for (const layer of layers)
        expect(layer).toMatch(/^-?\d+px -?\d+px 0 rgba\(0, 0, 0, 0\.85\)$/)
    }
    expect(OUTLINE_COLOR).toBe('rgba(0, 0, 0, 0.85)')
  })

  it('round-trips through parseOutlineWidth at every offered width', () => {
    for (const width of WIDTHS) expect(parseOutlineWidth(buildOutlineShadow(width))).toBe(width)
  })

  it('clamps a width the slider could never produce', () => {
    expect(buildOutlineShadow(0)).toBe(buildOutlineShadow(MIN_OUTLINE_WIDTH_PX))
    expect(buildOutlineShadow(99)).toBe(buildOutlineShadow(MAX_OUTLINE_WIDTH_PX))
  })

  // Never hyphenate after "stroke" in this file: design-tokens.test.ts scans
  // src/ with the real Tailwind scanner, and `stroke-<word>` in prose is picked
  // up as a dead `stroke-*` utility.
  it('uses no stroke to draw the outline (ADR-0044)', () => {
    for (const width of WIDTHS) {
      expect(buildOutlineShadow(width)).not.toMatch(/-webkit-text-stroke/)
      expect(buildOutlineShadow(width)).not.toMatch(/paint-order\s*:\s*stroke/i)
    }
  })
})

describe('parseOutlineWidth', () => {
  it('reads the width out of both samplings real overlays have stored', () => {
    expect(parseOutlineWidth(PRE_833_1PX)).toBe(1)
    expect(parseOutlineWidth(GHOSTED_833_3PX)).toBe(3)
  })

  // A value that has been through a CSS serialiser comes back respaced and
  // reordered. Matching the whole string would demote the control to the
  // read-only "Custom" entry and lose the slider.
  it('survives respacing and reordering', () => {
    const canonical = buildOutlineShadow(2)
    expect(parseOutlineWidth(canonical.replace(/, /g, ','))).toBe(2)
    expect(parseOutlineWidth(splitLayers(canonical).reverse().join(', '))).toBe(2)
    expect(parseOutlineWidth(`  ${canonical}  `)).toBe(2)
  })

  it('returns null for values that are not outlines', () => {
    expect(parseOutlineWidth(undefined)).toBeNull()
    expect(parseOutlineWidth('')).toBeNull()
    expect(parseOutlineWidth('   ')).toBeNull()
    expect(parseOutlineWidth(SOFT_PRESET)).toBeNull()
    expect(parseOutlineWidth(STRONG_PRESET)).toBeNull()
    expect(parseOutlineWidth('2px 2px 0 #ff00ff')).toBeNull()
    // Right colour, wrong geometry: a partial ring is not an outline.
    expect(parseOutlineWidth(`1px 0px 0 ${OUTLINE_COLOR}`)).toBeNull()
    // Right geometry, but blurred rather than hard-edged.
    expect(parseOutlineWidth(`1px 0px 2px ${OUTLINE_COLOR}`)).toBeNull()
  })
})

describe('canonicalizeTextShadow', () => {
  it('rewrites a stored outline in the current geometry', () => {
    expect(canonicalizeTextShadow(GHOSTED_833_3PX)).toBe(buildOutlineShadow(3))
    expect(canonicalizeTextShadow(PRE_833_1PX)).toBe(buildOutlineShadow(1))
    expect(canonicalizeTextShadow(buildOutlineShadow(5))).toBe(buildOutlineShadow(5))
  })

  it('passes anything that is not an outline through untouched', () => {
    expect(canonicalizeTextShadow(SOFT_PRESET)).toBe(SOFT_PRESET)
    expect(canonicalizeTextShadow(STRONG_PRESET)).toBe(STRONG_PRESET)
    expect(canonicalizeTextShadow('0 0 8px #0ff')).toBe('0 0 8px #0ff')
  })
})
