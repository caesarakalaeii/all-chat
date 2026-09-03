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

/**
 * The legibility outline behind overlay chat text, as a `text-shadow` value.
 *
 * ADR-0044 bans `-webkit-text-stroke` / `paint-order: stroke fill` for this
 * job. A layered `text-shadow` paints behind the glyphs instead, so it never
 * erodes the fill and it works on gradient (transparent-fill) usernames.
 *
 * ADR-0057 decides the GEOMETRY, which is the part that is easy to get wrong.
 * A zero-blur `text-shadow` layer paints one opaque copy of the glyph, shifted
 * by that layer's offset — it does not paint an outline. So the outline is the
 * union of the copies, and it is a true `width`-thick outline only when the
 * offsets fill the whole disc of radius `width`. Offsets sampled on the eight
 * compass points instead put eight *separated* copies of the text around it:
 * legible ghost text above and below the line, with the background showing
 * through the gaps between them. That is what shipped and had to be fixed.
 *
 * There is no separate width field in VisualSettings. The width lives in the
 * `textShadow` declaration itself, and `parseOutlineWidth` reads it back out
 * so the editor can put the slider where the user left it.
 */

/** Kept from the original fixed preset so switching widths never shifts hue. */
export const OUTLINE_COLOR = 'rgba(0, 0, 0, 0.85)'

/** Already thicker than the 1px preset this replaces, which is the ask. */
export const DEFAULT_OUTLINE_WIDTH_PX = 2

export const MIN_OUTLINE_WIDTH_PX = 1
export const MAX_OUTLINE_WIDTH_PX = 6

type Offset = readonly [x: number, y: number]

const clampWidth = (widthPx: number): number =>
  Math.min(MAX_OUTLINE_WIDTH_PX, Math.max(MIN_OUTLINE_WIDTH_PX, Math.round(widthPx)))

/**
 * Every integer offset inside the disc of radius `width`, in a fixed row-major
 * order. Unioning a copy of the glyph at each of these dilates the glyph by
 * that disc, which is exactly an outline of that thickness.
 *
 * Integer steps are what make it gap-free: neighbouring copies are 1px apart,
 * and any glyph feature the eye can see is at least a pixel across, so
 * consecutive copies always overlap. Sub-sampling the disc (every other
 * lattice point, or rings without their interior) is tempting and does cut the
 * layer count, but it opens hairline gaps on thin fonts and leaves a hole
 * around 1px features like a period or the dot of an `i`.
 *
 * Cost is the price of correctness: the layer count grows as the disc's area,
 * from 4 layers at 1px to 112 at 6px. Measured in headless Chromium against a
 * deliberately pessimistic load — 40 rows of 16px bold text, fully invalidated
 * and repainted every frame — everything up to 4px (48 layers) stayed pinned
 * at the 16.7ms vsync floor, i.e. free; 5px (80) cost 21ms/frame and 6px (112)
 * 25.7ms/frame. Real overlays repaint on a new message or a scroll, not on
 * every frame, so the top of the range is affordable but is the reason the
 * range stops at 6.
 */
export function outlineOffsets(widthPx: number): readonly Offset[] {
  const width = clampWidth(widthPx)
  const offsets: Offset[] = []
  for (let y = -width; y <= width; y++) {
    for (let x = -width; x <= width; x++) {
      if (x === 0 && y === 0) continue
      if (x * x + y * y <= width * width) offsets.push([x, y])
    }
  }
  return offsets
}

/** A layered outline `text-shadow` of the given thickness in pixels. */
export function buildOutlineShadow(widthPx: number): string {
  return outlineOffsets(widthPx)
    .map(([x, y]) => `${x}px ${y}px 0 ${OUTLINE_COLOR}`)
    .join(', ')
}

/**
 * The compass-point sampling that shipped in #833 and rendered as ghost copies
 * of the text rather than an outline. Recognised, never emitted: overlays were
 * saved with it, and `parseOutlineWidth` reporting its width is what lets the
 * editor show the slider and the CSS generator re-emit the value in the
 * corrected geometry.
 */
const ghostedCompassOffsets = (width: number): readonly Offset[] =>
  (
    [
      [1, 0],
      [0, 1],
      [-1, 0],
      [0, -1],
      [1, 1],
      [-1, 1],
      [-1, -1],
      [1, -1],
    ] as const
  ).map(([x, y]): Offset => [x * width, y * width])

/**
 * The four-diagonal sampling the editor hard-coded at 1px before the width was
 * settable. Also recognised rather than emitted, for the same reason; at 1px
 * the corrected value is visually equivalent to it.
 */
const diagonalOffsets = (width: number): readonly Offset[] => [
  [width, width],
  [-width, width],
  [width, -width],
  [-width, -width],
]

/** Every sampling a stored value may legitimately be in, current one first. */
const RECOGNISED_SAMPLINGS: ReadonlyArray<(width: number) => readonly Offset[]> = [
  outlineOffsets,
  ghostedCompassOffsets,
  diagonalOffsets,
]

const offsetKeys = (offsets: readonly Offset[]): Set<string> =>
  new Set(offsets.map(([x, y]) => `${x},${y}`))

const sameOffsets = (a: Set<string>, b: Set<string>): boolean =>
  a.size === b.size && [...a].every((key) => b.has(key))

/** Splits a comma-separated CSS list on the commas that are not inside `()`. */
function splitTopLevel(value: string): string[] {
  const parts: string[] = []
  let depth = 0
  let start = 0
  for (let i = 0; i < value.length; i++) {
    const c = value[i]
    if (c === '(') depth++
    else if (c === ')') depth--
    else if (c === ',' && depth === 0) {
      parts.push(value.slice(start, i))
      start = i + 1
    }
  }
  parts.push(value.slice(start))
  return parts
}

/** `<x>px <y>px 0 <colour>` — a hard-edged shadow layer, no blur, no spread. */
const HARD_SHADOW_LAYER = /^(-?\d+)px\s+(-?\d+)px\s+0(?:px)?\s+(.+)$/

const normaliseColor = (color: string): string => color.replace(/\s+/g, '').toLowerCase()

/**
 * The thickness of an outline `text-shadow`, or null when the value is
 * anything else — unset, empty, one of the soft/strong shadow presets, a
 * theme's own declaration, or a hand-written custom value. The editor shows
 * its read-only "Custom" entry for the rest.
 *
 * Matches on the offsets it decodes rather than on the whole string, so a
 * value that has been through a CSS serialiser — different spacing, `0px` for
 * `0`, layers in another order — still reads as the outline it is instead of
 * silently demoting the control to "Custom".
 */
export function parseOutlineWidth(value: string | undefined): number | null {
  if (value === undefined || value.trim() === '') return null

  const layers = splitTopLevel(value).map((layer) => layer.trim())
  const offsets = new Set<string>()
  for (const layer of layers) {
    const match = HARD_SHADOW_LAYER.exec(layer)
    if (match === null) return null
    if (normaliseColor(match[3]) !== normaliseColor(OUTLINE_COLOR)) return null
    offsets.add(`${Number(match[1])},${Number(match[2])}`)
  }

  for (let width = MIN_OUTLINE_WIDTH_PX; width <= MAX_OUTLINE_WIDTH_PX; width++) {
    for (const sampling of RECOGNISED_SAMPLINGS) {
      if (sameOffsets(offsets, offsetKeys(sampling(width)))) return width
    }
  }
  return null
}

/**
 * A stored `textShadow` rewritten in the current geometry when it is an
 * outline, and passed through untouched when it is not.
 *
 * This is what makes the fix reach overlays that were saved while the broken
 * sampling was live: the width is the setting, the declaration is only a
 * rendering of it, so re-deriving the declaration at CSS-generation time fixes
 * every affected overlay on deploy without rewriting a single stored row or
 * waiting for its owner to touch the slider again.
 */
export function canonicalizeTextShadow(value: string): string {
  const width = parseOutlineWidth(value)
  return width === null ? value : buildOutlineShadow(width)
}
