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
  alphaFromHex,
  hexWithAlpha,
  normalizeHex,
  stripAlpha,
  withLegacyOpacity,
} from '@/lib/utils/hex-alpha'

describe('normalizeHex', () => {
  it('expands 3- and 4-digit shorthand and lowercases', () => {
    expect(normalizeHex('#ABC')).toBe('#aabbcc')
    expect(normalizeHex('#abcd')).toBe('#aabbccdd')
  })

  it('keeps 6- and 8-digit values, adding the missing #', () => {
    expect(normalizeHex('1a1a2e')).toBe('#1a1a2e')
    expect(normalizeHex('#1A1A2E80')).toBe('#1a1a2e80')
  })

  it('returns undefined for non-hex input', () => {
    expect(normalizeHex('')).toBeUndefined()
    expect(normalizeHex('rgba(0,0,0,0.5)')).toBeUndefined()
    expect(normalizeHex('#12345')).toBeUndefined()
  })
})

describe('alphaFromHex', () => {
  it('reads the alpha channel of 8- and 4-digit hex', () => {
    expect(alphaFromHex('#1a1a2e00')).toBe(0)
    expect(alphaFromHex('#1a1a2eff')).toBe(1)
    expect(alphaFromHex('#0000')).toBe(0)
  })

  it('treats a color without an alpha channel as fully opaque', () => {
    expect(alphaFromHex('#1a1a2e')).toBe(1)
    expect(alphaFromHex('#abc')).toBe(1)
  })

  it('falls back to opaque for unparseable input', () => {
    expect(alphaFromHex('not-a-color')).toBe(1)
  })
})

describe('hexWithAlpha', () => {
  it('drops the alpha channel at full opacity so stored colors stay 6-digit', () => {
    expect(hexWithAlpha('#1a1a2e', 1)).toBe('#1a1a2e')
    expect(hexWithAlpha('#1a1a2e80', 1)).toBe('#1a1a2e')
  })

  it('appends the alpha channel below full opacity', () => {
    expect(hexWithAlpha('#1a1a2e', 0)).toBe('#1a1a2e00')
    expect(hexWithAlpha('#1a1a2e', 0.5)).toBe('#1a1a2e80')
  })

  it('replaces an existing alpha channel instead of appending', () => {
    expect(hexWithAlpha('#1a1a2eff', 0.5)).toBe('#1a1a2e80')
  })

  it('clamps out-of-range alpha', () => {
    expect(hexWithAlpha('#1a1a2e', 5)).toBe('#1a1a2e')
    expect(hexWithAlpha('#1a1a2e', -1)).toBe('#1a1a2e00')
    expect(hexWithAlpha('#1a1a2e', Number.NaN)).toBe('#1a1a2e')
  })

  it('round-trips every whole percent through alphaFromHex', () => {
    for (let percent = 0; percent <= 100; percent++) {
      const composed = hexWithAlpha('#1a1a2e', percent / 100)
      expect(Math.round(alphaFromHex(composed) * 100)).toBe(percent)
    }
  })

  it('returns non-hex input unchanged', () => {
    expect(hexWithAlpha('currentColor', 0.5)).toBe('currentColor')
  })
})

describe('stripAlpha', () => {
  it('removes the alpha channel', () => {
    expect(stripAlpha('#1a1a2e80')).toBe('#1a1a2e')
    expect(stripAlpha('#1a1a2e')).toBe('#1a1a2e')
  })

  it('returns non-hex input unchanged', () => {
    expect(stripAlpha('inherit')).toBe('inherit')
  })
})

describe('withLegacyOpacity', () => {
  it('folds a legacy sibling opacity field into the color', () => {
    expect(withLegacyOpacity('#1a1a2e', '0.5')).toBe('#1a1a2e80')
  })

  it('prefers an alpha channel already carried by the color', () => {
    expect(withLegacyOpacity('#1a1a2e00', '0.85')).toBe('#1a1a2e00')
  })

  it('leaves the color alone when no legacy opacity is stored', () => {
    expect(withLegacyOpacity('#1a1a2e')).toBe('#1a1a2e')
    expect(withLegacyOpacity('#1a1a2e', '')).toBe('#1a1a2e')
    expect(withLegacyOpacity('#1a1a2e', 'nonsense')).toBe('#1a1a2e')
  })
})
