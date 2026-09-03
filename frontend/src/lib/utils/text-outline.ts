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
 * job: a stroke is centred on the glyph path, so half its width erodes the
 * glyph fill and it muddies gradient (transparent-fill) usernames. A layered
 * `text-shadow` paints behind the glyphs instead, so it thickens cleanly.
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

/**
 * Offset directions, in the order `buildOutlineShadow` emits them: the four
 * axis-aligned edges clockwise from east, then the four diagonals clockwise
 * from south-east. The order is fixed because `parseOutlineWidth` compares the
 * whole string, so a reordering would stop stored values round-tripping.
 *
 * Eight rather than four directions: at 2px and up a four-diagonal outline
 * leaves visible gaps along the horizontal and vertical edges of a glyph.
 */
const OUTLINE_DIRECTIONS: ReadonlyArray<readonly [x: number, y: number]> = [
  [1, 0],
  [0, 1],
  [-1, 0],
  [0, -1],
  [1, 1],
  [-1, 1],
  [-1, -1],
  [1, -1],
]

/**
 * The four-direction 1px outline the editor hard-coded before the width was
 * settable. Overlays saved with it are still out there, so `parseOutlineWidth`
 * recognises it as width 1 and the slider picks it up; nothing rewrites stored
 * settings. `buildOutlineShadow(1)` deliberately differs — it has eight offsets
 * — which is visually equivalent at 1px.
 */
export const LEGACY_OUTLINE_SHADOW =
  '1px 1px 0 rgba(0, 0, 0, 0.85), -1px 1px 0 rgba(0, 0, 0, 0.85), 1px -1px 0 rgba(0, 0, 0, 0.85), -1px -1px 0 rgba(0, 0, 0, 0.85)'

/** A layered eight-direction outline `text-shadow` at the given width. */
export function buildOutlineShadow(widthPx: number): string {
  return OUTLINE_DIRECTIONS.map(
    ([x, y]) => `${x * widthPx}px ${y * widthPx}px 0 ${OUTLINE_COLOR}`
  ).join(', ')
}

/**
 * The width of an outline `text-shadow`, or null when the value is anything
 * else — unset, empty, one of the soft/strong shadow presets, a theme's own
 * declaration, or a hand-written custom value. Only strings this module could
 * have produced (plus the legacy one) are outlines; the editor shows its
 * read-only "Custom" entry for the rest.
 */
export function parseOutlineWidth(value: string | undefined): number | null {
  if (value === undefined || value === '') return null
  if (value === LEGACY_OUTLINE_SHADOW) return MIN_OUTLINE_WIDTH_PX
  for (let width = MIN_OUTLINE_WIDTH_PX; width <= MAX_OUTLINE_WIDTH_PX; width++) {
    if (value === buildOutlineShadow(width)) return width
  }
  return null
}
