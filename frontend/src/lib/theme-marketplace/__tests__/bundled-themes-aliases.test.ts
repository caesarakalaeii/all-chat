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
import { BUNDLED_THEMES, getBundledTheme, THEME_ALIASES } from '../bundled-themes'

/**
 * Retiring a bundled theme is a data-integrity problem, not a file deletion.
 *
 * `overlay_configs.theme_id` stores the id, and theme CSS is resolved from the
 * bundle by that id at render time. An id that resolves to nothing renders an
 * overlay with NO theme CSS — so a retired id has to keep resolving (ADR-0053).
 * `minimal-theme-fixed` alone was the single most-used theme on the platform
 * when it was consolidated away, which is the blast radius these tests guard.
 */
describe('retired theme ids', () => {
  it('every alias target is a real bundled theme', () => {
    for (const [from, to] of Object.entries(THEME_ALIASES)) {
      expect(BUNDLED_THEMES.find((t) => t.id === to), `${from} -> ${to}`).toBeDefined()
    }
  })

  it('an alias is only for ids that are NOT bundled themselves', () => {
    // If both existed, the picker would offer a theme that silently renders as
    // another one.
    for (const from of Object.keys(THEME_ALIASES)) {
      expect(BUNDLED_THEMES.find((t) => t.id === from), from).toBeUndefined()
    }
  })

  it('resolves minimal-theme-fixed to the consolidated Minimal Clean theme', () => {
    const theme = getBundledTheme('minimal-theme-fixed')
    expect(theme?.id).toBe('minimal-theme')
    expect(theme?.css ?? '').not.toBe('')
  })

  it('aliases never chain (an alias target is never itself an alias)', () => {
    for (const to of Object.values(THEME_ALIASES)) {
      expect(THEME_ALIASES[to]).toBeUndefined()
    }
  })

  it('an unknown id still resolves to nothing', () => {
    expect(getBundledTheme('no-such-theme')).toBeUndefined()
  })
})
