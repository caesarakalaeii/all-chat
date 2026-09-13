/**
 * This file is part of All-Chat.
 * Copyright (C) 2026 caesarakalaeii
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published
 * by the Free Software Foundation, either version 3 of the License, or
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

import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

import { wrapThemeCss } from '@/lib/theme-marketplace/wrap-theme-css'
import type { VisualSettings } from '@/lib/types/visual-settings'
import { visualSettingsToCss } from '@/lib/utils/visual-settings-to-css'

/**
 * Guard for the overlay CSS precedence hierarchy:
 *
 *   1. Manual CSS (unlayered)
 *   2. GUI settings (`@layer visual-customizer`)
 *   3. Theme (`@layer marketplace-themes`, wrapped at injection)
 *
 * The GUI tier gets its rank from the layer order alone: it sits in the top
 * computed layer, and themes are stripped of their `!important` flags at
 * injection (wrap-theme-css.ts). The moment any declaration in the GUI chain
 * regains an `!important`, a layered-important declaration beats even the
 * user's unlayered normal-weight manual CSS, and the "manual CSS cannot
 * override the GUI" symptom returns. So the whole chain must stay at normal
 * weight.
 */

const EVENTS_CSS = readFileSync(fileURLToPath(new URL('../events.css', import.meta.url)), 'utf8')

/** Extract every `@layer visual-customizer { ... }` body (brace-depth aware). */
function visualCustomizerBodies(css: string): string[] {
  const bodies: string[] = []
  const re = /@layer visual-customizer\s*\{/g
  let m: RegExpExecArray | null
  while ((m = re.exec(css)) !== null) {
    let depth = 1
    let i = m.index + m[0].length
    while (i < css.length && depth > 0) {
      if (css[i] === '{') depth++
      else if (css[i] === '}') depth--
      i++
    }
    // Comments may legitimately mention `!important` (explaining why it is
    // absent); only declarations matter.
    bodies.push(css.slice(m.index + m[0].length, i - 1).replace(/\/\*[\s\S]*?\*\//g, ''))
  }
  return bodies
}
describe('cascade precedence: GUI chain stays at normal weight', () => {
  const FULL: VisualSettings = {
    textShadow: '1px 1px 0 #000, -1px 1px 0 #000',
    bubbleShadow: '0 2px 6px rgba(0, 0, 0, 0.5)',
    twitchAccent: '#9146ff',
    bubblePalette: ['#111111', '#222222'],
    bubbleBgColor: '#111111',
    bubbleBgOpacity: '0.9',
    fontFamily: 'Inter',
    fontSize: '16px',
  }

  it('visualSettingsToCss emits no !important for the full override set', () => {
    const css = visualSettingsToCss(FULL)
    expect(css).not.toContain('!important')
  })

  it('visualSettingsToCss emits no !important for the empty palette edge case', () => {
    const css = visualSettingsToCss({ bubblePalette: [] })
    expect(css).not.toContain('!important')
  })

  it('events.css declares visual-customizer above every tier it must beat', () => {
    // The explicit order statement: everything the GUI must outrank has to be
    // listed to its left (globals.css prepends the Tailwind stack, so this
    // line only names the overlay-specific tiers).
    expect(EVENTS_CSS).toMatch(
      /@layer base, design-system, marketplace-themes, visual-customizer, user-overrides;/
    )
  })

  it('events.css visual-customizer blocks carry no !important', () => {
    const bodies = visualCustomizerBodies(EVENTS_CSS)
    expect(bodies.length).toBeGreaterThan(0)
    for (const body of bodies) {
      expect(body).not.toContain('!important')
    }
  })

  it('wrapThemeCss strips !important from theme CSS and layers it below the GUI tier', () => {
    const wrapped = wrapThemeCss('.a { color: red !important; }')
    expect(wrapped).toContain('@layer marketplace-themes')
    expect(wrapped).not.toContain('!important')
  })
})
