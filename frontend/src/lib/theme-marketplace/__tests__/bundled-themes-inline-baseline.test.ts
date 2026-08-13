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

/**
 * An inline-layout theme must not blockify the username header.
 *
 * Themes that render "username: message" on one line make the message text
 * `display: inline`. If the header that carries the username is `inline-flex`,
 * the username becomes a flex item inside its own line box instead of sharing
 * the inline text baseline, so it sits a couple of pixels higher than the
 * message beside it. `display: inline` on the header puts both on the same
 * baseline; the platform icon and badges keep their horizontal spacing from
 * their own `margin-right`/`vertical-align`, not from the flex `gap`.
 *
 * Fixed once in minimal-theme.css (97e935b9) but never ported to the two other
 * themes that had copied the same header block — which is why this is a guard
 * over every bundled theme rather than a one-line fix.
 */

function stripComments(css: string): string {
  return css.replace(/\/\*[\s\S]*?\*\//g, '')
}

/** Declaration bodies of every rule whose selector contains `needle`. */
function ruleBodies(css: string, needle: string): string[] {
  const bodies: string[] = []
  const rule = /([^{}]+)\{([^{}]*)\}/g
  let match: RegExpExecArray | null
  while ((match = rule.exec(css)) !== null) {
    if (match[1].includes(needle)) bodies.push(match[2])
  }
  return bodies
}

/** Literal `display` keyword of a declaration body, ignoring `var(...)` forms. */
function literalDisplay(body: string): string | null {
  const match = /display\s*:\s*([a-z-]+)\s*(?:!important)?\s*;/i.exec(body)
  const value = match?.[1].toLowerCase()
  return !value || value === 'var' ? null : value
}

/** The header element that carries the platform badge, user badges and username. */
const USERNAME_HEADER = '.flex.items-center.gap-2'

describe('bundled theme inline-layout baseline', () => {
  const themes = getBundledThemes()

  it.each(themes.map((t) => [t.id, t] as const))(
    'theme %s keeps the username on the message baseline',
    (_id, theme) => {
      const css = stripComments(theme.css)

      // An inline-layout theme is one that runs the message text inline.
      const rendersMessageInline = ruleBodies(css, '.break-words')
        .filter((body) => literalDisplay(body) !== null)
        .some((body) => literalDisplay(body) === 'inline')
      if (!rendersMessageInline) return

      const headerDisplays = ruleBodies(css, USERNAME_HEADER)
        .map(literalDisplay)
        .filter((value): value is string => value !== null)

      const blockified = headerDisplays.filter((d) => d === 'flex' || d === 'inline-flex')
      expect(
        blockified,
        `This theme renders the message inline but sets '${USERNAME_HEADER}' to ` +
          `${blockified.join('/')}. That makes the username a flex item in its own ` +
          `line box, so it renders ~2px above the message text next to it. Use ` +
          `'display: inline' — the badges keep their spacing via margin-right.`
      ).toEqual([])
    }
  )
})
