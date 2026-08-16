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

import { describe, expect, it } from 'vitest'
import {
  chatBubbleStyle,
  hexToRgba,
  overlayContainerStyle,
} from '../visual-inline-styles'

describe('hexToRgba', () => {
  it('combines a 6-digit hex with a 0–1 opacity', () => {
    expect(hexToRgba('#1a1a2e', '0.85')).toBe('rgba(26, 26, 46, 0.85)')
  })

  it('expands a 3-digit hex and tolerates a missing leading #', () => {
    expect(hexToRgba('f00', '1')).toBe('rgba(255, 0, 0, 1)')
  })

  it('defaults opacity to 1 when absent and clamps out-of-range values', () => {
    expect(hexToRgba('#000000')).toBe('rgba(0, 0, 0, 1)')
    expect(hexToRgba('#000000', '5')).toBe('rgba(0, 0, 0, 1)')
    expect(hexToRgba('#ffffff', '-2')).toBe('rgba(255, 255, 255, 0)')
  })

  it('returns undefined for missing or invalid input so callers keep defaults', () => {
    expect(hexToRgba(undefined, '0.5')).toBeUndefined()
    expect(hexToRgba('not-a-color', '0.5')).toBeUndefined()
    expect(hexToRgba('', '0.5')).toBeUndefined()
  })

  it('reads the alpha channel of an 8-digit hex (ADR-0050)', () => {
    expect(hexToRgba('#1a1a2e00')).toBe('rgba(26, 26, 46, 0)')
    expect(hexToRgba('#ff0000ff')).toBe('rgba(255, 0, 0, 1)')
    // 0x80/255 rounds to 3 decimals rather than trailing 16 digits of float
    expect(hexToRgba('#ff000080')).toBe('rgba(255, 0, 0, 0.502)')
  })

  it('expands a 4-digit hex', () => {
    expect(hexToRgba('#f000')).toBe('rgba(255, 0, 0, 0)')
  })

  it('lets the color’s own alpha win over a legacy opacity field', () => {
    // A re-saved setting carries alpha in the color; a stale sibling
    // *BgOpacity must not dim it a second time.
    expect(hexToRgba('#1a1a2e00', '0.85')).toBe('rgba(26, 26, 46, 0)')
  })
})

describe('overlayContainerStyle', () => {
  it('is empty when nothing is configured (Tailwind defaults win)', () => {
    expect(overlayContainerStyle({})).toEqual({})
  })

  it('emits background + max width only for set values', () => {
    expect(
      overlayContainerStyle({ overlayBgColor: '#000000', overlayBgOpacity: '0.7' })
    ).toEqual({ backgroundColor: 'rgba(0, 0, 0, 0.7)' })
    expect(overlayContainerStyle({ maxWidth: '600px' })).toEqual({ maxWidth: '600px' })
  })
})

describe('chatBubbleStyle', () => {
  it('is empty when nothing is configured', () => {
    expect(chatBubbleStyle({})).toEqual({})
  })

  it('renders a fully transparent bubble from an alpha-carrying color', () => {
    expect(chatBubbleStyle({ bubbleBgColor: '#1a1a2e00' })).toEqual({
      backgroundColor: 'rgba(26, 26, 46, 0)',
    })
  })

  it('emits background + shadow only for set values', () => {
    expect(
      chatBubbleStyle({ bubbleBgColor: '#1a1a2e', bubbleBgOpacity: '0.85' })
    ).toEqual({ backgroundColor: 'rgba(26, 26, 46, 0.85)' })
    expect(chatBubbleStyle({ bubbleShadow: '0 0 8px red' })).toEqual({
      boxShadow: '0 0 8px red',
    })
  })
})
