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
  parseOutlineWidth,
  LEGACY_OUTLINE_SHADOW,
  MIN_OUTLINE_WIDTH_PX,
  MAX_OUTLINE_WIDTH_PX,
  OUTLINE_COLOR,
} from '../text-outline'

const SOFT_PRESET = '0 1px 2px rgba(0, 0, 0, 0.6)'
const STRONG_PRESET = '0 1px 2px rgba(0, 0, 0, 0.9), 0 2px 6px rgba(0, 0, 0, 0.7)'

// Spelled out here rather than imported: this is the value real overlays have
// stored in their visual settings, so the test has to fail if the module's
// constant ever drifts away from it. Asserting parseOutlineWidth against the
// imported constant would only prove the module agrees with itself.
const LEGACY_OUTLINE_AS_STORED =
  '1px 1px 0 rgba(0, 0, 0, 0.85), -1px 1px 0 rgba(0, 0, 0, 0.85), 1px -1px 0 rgba(0, 0, 0, 0.85), -1px -1px 0 rgba(0, 0, 0, 0.85)'

const WIDTHS = [1, 2, 3, 4, 5, 6]

describe('buildOutlineShadow', () => {
  it('round-trips through parseOutlineWidth at every offered width', () => {
    for (const width of WIDTHS) {
      expect(parseOutlineWidth(buildOutlineShadow(width))).toBe(width)
    }
  })

  // Four axis-aligned offsets plus four diagonals. Counted by matching the
  // offset pairs rather than splitting on ',' because the rgba() colour of each
  // layer contains commas of its own.
  it('emits eight layers, one per compass direction', () => {
    const shadow = buildOutlineShadow(2)
    const offsets = shadow.match(/-?\d+px -?\d+px 0 /g) ?? []
    expect(offsets).toHaveLength(8)
    expect(new Set(offsets).size).toBe(8)
  })

  // Never hyphenate after "stroke" in this file: design-tokens.test.ts scans
  // src/ with the real Tailwind scanner, and `stroke-<word>` in prose is picked
  // up as a dead `stroke-*` utility.
  it('uses no stroke to draw the outline (ADR-0044)', () => {
    for (const width of WIDTHS) {
      const shadow = buildOutlineShadow(width)
      expect(shadow).not.toMatch(/-webkit-text-stroke/)
      expect(shadow).not.toMatch(/paint-order\s*:\s*stroke/i)
    }
  })

  it('offers widths spanning the advertised slider range', () => {
    expect(MIN_OUTLINE_WIDTH_PX).toBe(1)
    expect(MAX_OUTLINE_WIDTH_PX).toBe(6)
  })

  // Pins the colour as well as the layout: switching widths must not shift hue,
  // and an outline is only legible if it is actually near-opaque black.
  it('paints every layer in the original preset black', () => {
    expect(buildOutlineShadow(1)).toBe(
      '1px 0px 0 rgba(0, 0, 0, 0.85), 0px 1px 0 rgba(0, 0, 0, 0.85), -1px 0px 0 rgba(0, 0, 0, 0.85), 0px -1px 0 rgba(0, 0, 0, 0.85), 1px 1px 0 rgba(0, 0, 0, 0.85), -1px 1px 0 rgba(0, 0, 0, 0.85), -1px -1px 0 rgba(0, 0, 0, 0.85), 1px -1px 0 rgba(0, 0, 0, 0.85)'
    )
    expect(OUTLINE_COLOR).toBe('rgba(0, 0, 0, 0.85)')
  })
})

describe('parseOutlineWidth', () => {
  it('reads the legacy four-direction 1px outline as width 1', () => {
    expect(parseOutlineWidth(LEGACY_OUTLINE_AS_STORED)).toBe(1)
    expect(LEGACY_OUTLINE_SHADOW).toBe(LEGACY_OUTLINE_AS_STORED)
  })

  it('returns null for values that are not outlines', () => {
    expect(parseOutlineWidth(undefined)).toBeNull()
    expect(parseOutlineWidth('')).toBeNull()
    expect(parseOutlineWidth(SOFT_PRESET)).toBeNull()
    expect(parseOutlineWidth(STRONG_PRESET)).toBeNull()
    expect(parseOutlineWidth('2px 2px 0 #ff00ff')).toBeNull()
  })
})
