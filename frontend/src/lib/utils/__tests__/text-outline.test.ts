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
} from '../text-outline'

const SOFT_PRESET = '0 1px 2px rgba(0, 0, 0, 0.6)'
const STRONG_PRESET = '0 1px 2px rgba(0, 0, 0, 0.9), 0 2px 6px rgba(0, 0, 0, 0.7)'

const WIDTHS = [1, 2, 3, 4, 5, 6]

describe('buildOutlineShadow', () => {
  it('round-trips through parseOutlineWidth at every offered width', () => {
    for (const width of WIDTHS) {
      expect(parseOutlineWidth(buildOutlineShadow(width))).toBe(width)
    }
  })

  it('emits eight layers, one per compass direction', () => {
    const layers = buildOutlineShadow(2).split(',')
    expect(layers).toHaveLength(8)
  })

  it('uses no stroke-based outline technique (ADR-0044)', () => {
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
})

describe('parseOutlineWidth', () => {
  it('reads the legacy four-direction 1px outline as width 1', () => {
    expect(parseOutlineWidth(LEGACY_OUTLINE_SHADOW)).toBe(1)
  })

  it('returns null for values that are not outlines', () => {
    expect(parseOutlineWidth(undefined)).toBeNull()
    expect(parseOutlineWidth('')).toBeNull()
    expect(parseOutlineWidth(SOFT_PRESET)).toBeNull()
    expect(parseOutlineWidth(STRONG_PRESET)).toBeNull()
    expect(parseOutlineWidth('2px 2px 0 #ff00ff')).toBeNull()
  })
})
