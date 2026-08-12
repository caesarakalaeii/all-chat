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
 * Every bundled theme must style events.
 *
 * A theme that styles only chat leaves events on the platform default — a
 * scaled-up card with a gold/purple gradient, a glowing border, a bouncing
 * entrance and a 36px emoji. Dropped into a minimal or retro overlay that reads
 * as a different product, which is exactly the complaint this guard exists to
 * prevent from coming back with the next theme someone adds.
 *
 * Authoring guide: docs/overlay-themes/AUTHORING-EVENTS.md
 */

// Comments are stripped so the guard can't be satisfied by a theme that merely
// *mentions* events in prose (same technique as bundled-themes-outline).
function stripComments(css: string): string {
  return css.replace(/\/\*[\s\S]*?\*\//g, '')
}

/** The default chrome a theme has to take a position on, one way or another. */
const CALMING_TOKENS = [
  '--event-scale',
  '--event-animation',
  '--event-bg',
  '--event-border-width',
  '--event-min-height',
]

describe('bundled theme event styling', () => {
  const themes = getBundledThemes()

  it('bundles at least one theme', () => {
    expect(themes.length).toBeGreaterThan(0)
  })

  it.each(themes.map((t) => [t.id, t] as const))('theme %s styles event rows', (_id, theme) => {
    const active = stripComments(theme.css)
    expect(
      /\.event-message|\.event-tier-|\.event-type-|--event-/.test(active),
      `Add an EVENTS section to this theme — see docs/overlay-themes/AUTHORING-EVENTS.md. ` +
        `Without one, subs/raids/redemptions render in the platform default styling ` +
        `(gold gradient card, glow, bounce) regardless of the theme.`
    ).toBe(true)
  })

  it.each(themes.map((t) => [t.id, t] as const))(
    'theme %s takes a position on the default event chrome',
    (_id, theme) => {
      const active = stripComments(theme.css)
      const handled =
        CALMING_TOKENS.some((token) => active.includes(token)) ||
        // A theme may instead override the chrome outright with its own rules.
        /\.event-message[^{]*\{[^}]*(background|border|box-shadow|animation|transform)/.test(active)
      expect(
        handled,
        `This theme names events but never touches the default card chrome ` +
          `(background/border/glow/animation/scale). Set the --event-* tokens or ` +
          `override the rules — see docs/overlay-themes/AUTHORING-EVENTS.md.`
      ).toBe(true)
    }
  )

  it.each(themes.map((t) => [t.id, t] as const))(
    'theme %s does not repaint the viewer-owned username colour on events',
    (_id, theme) => {
      const active = stripComments(theme.css)
      // `.chat-username` is the chatter's own colour on chat AND event rows.
      const repaints = /\.event-message[^{]*\.chat-username[^{]*\{[^}]*[^-]color\s*:/.test(active)
      expect(
        repaints,
        `Name colours belong to the viewer — style size/weight/font/outline, not color.`
      ).toBe(false)
    }
  )
})
