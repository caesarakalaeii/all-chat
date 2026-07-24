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
import { getBundledThemes } from '../bundled-themes'

// Strip /* ... */ comments so we only assert on *active* declarations — the
// theme sources legitimately mention "-webkit-text-stroke" in explanatory
// comments and in commented-out optional snippets.
function stripComments(css: string): string {
  return css.replace(/\/\*[\s\S]*?\*\//g, '')
}

describe('bundled theme text legibility (ADR-0044)', () => {
  const themes = getBundledThemes()

  it('bundles at least one theme', () => {
    expect(themes.length).toBeGreaterThan(0)
  })

  it.each(themes.map((t) => [t.id, t] as const))(
    'theme %s uses no legacy -webkit-text-stroke outline in active CSS',
    (_id, theme) => {
      const active = stripComments(theme.css)
      expect(active).not.toMatch(/-webkit-text-stroke/)
    }
  )

  it.each(themes.map((t) => [t.id, t] as const))(
    'theme %s uses no legacy "paint-order: stroke" outline in active CSS',
    (_id, theme) => {
      const active = stripComments(theme.css)
      expect(active).not.toMatch(/paint-order\s*:\s*stroke/i)
    }
  )
})
