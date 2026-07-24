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
  isCustomCssForked,
  persistedCustomCss,
  markerSeverityToLevel,
} from '../custom-css'

const THEME = '.chat-username { color: gold; }'

describe('isCustomCssForked', () => {
  it('is false for an empty editor (no theme)', () => {
    expect(isCustomCssForked('', '')).toBe(false)
  })

  it('is false when the editor is only whitespace', () => {
    expect(isCustomCssForked('   \n  ', '')).toBe(false)
  })

  it('is false when the editor still equals the pristine theme (preloaded, untouched)', () => {
    expect(isCustomCssForked(THEME, THEME)).toBe(false)
  })

  it('is true when the editor diverges from the pristine theme', () => {
    expect(isCustomCssForked(THEME + '\n.x{}', THEME)).toBe(true)
  })

  it('is true for hand-written CSS with no theme applied', () => {
    expect(isCustomCssForked('.x { color: red; }', '')).toBe(true)
  })
})

describe('persistedCustomCss', () => {
  it('persists nothing when the theme is untouched (keeps the overlay linked to the bundle)', () => {
    expect(persistedCustomCss(THEME, THEME)).toBe('')
  })

  it('persists nothing for an empty editor', () => {
    expect(persistedCustomCss('', '')).toBe('')
    expect(persistedCustomCss('', THEME)).toBe('')
  })

  it('persists the edited CSS verbatim once it diverges from the theme (fork)', () => {
    const edited = THEME + '\n.chat-message { background: blue; }'
    expect(persistedCustomCss(edited, THEME)).toBe(edited)
  })

  it('persists hand-written CSS when there is no theme', () => {
    expect(persistedCustomCss('.x { color: red; }', '')).toBe('.x { color: red; }')
  })
})

describe('markerSeverityToLevel', () => {
  it('maps Monaco Error(8) → error', () => {
    expect(markerSeverityToLevel(8)).toBe('error')
  })

  it('maps Monaco Warning(4) → warning', () => {
    expect(markerSeverityToLevel(4)).toBe('warning')
  })

  it('maps Monaco Info(2) and Hint(1) → info', () => {
    expect(markerSeverityToLevel(2)).toBe('info')
    expect(markerSeverityToLevel(1)).toBe('info')
  })
})
