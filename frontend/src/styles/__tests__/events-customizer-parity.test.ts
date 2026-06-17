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

import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

/**
 * Regression guard for the "preview changes, live overlay doesn't" class of bug.
 *
 * The visual customizer injects `--chat-*` custom properties on `:root` for both
 * the preview/embed pages and the live OBS overlay (`/overlay/[id]`). Historically
 * the *consuming* rules were scoped only to `.overlay-preview-body`, so every
 * setting (emote scale, typography, colors, sizing, bubbles) silently did nothing
 * on the actual overlay — it only ever changed the preview.
 *
 * This test asserts that every `--chat-*` variable consumed under the
 * `.overlay-preview-body` scope is also honored by the live overlay: either via a
 * matching `.overlay-live-body` rule, or via one of the documented exceptions the
 * live overlay handles in React / structurally.
 */

const CSS = readFileSync(
  fileURLToPath(new URL('../events.css', import.meta.url)),
  'utf8'
)

/** Variables the live overlay deliberately does NOT consume via CSS. */
const LIVE_EXCEPTIONS = new Set<string>([
  // Visibility toggles: the live overlay conditionally renders these in React
  // (showAvatars/showBadges/showUsername/showTimestamps state), so no CSS needed.
  '--chat-show-avatars',
  '--chat-show-badges',
  '--chat-show-username',
  '--chat-show-timestamps',
  // Body font-size: applied inline in React (visual `fontSize` ?? legacy
  // `display.font_size`) so it doesn't clobber the legacy display-settings path.
  '--chat-font-size',
  // Overlay padding: the live overlay owns its outer padding (`p-4`); this is not
  // a user-facing customizer control.
  '--chat-overlay-padding',
])

/** Extract the body of the first `@layer visual-customizer { ... }` block that
 *  contains the given scope selector, then collect the `--chat-*` vars used. */
function varsConsumedUnderScope(scope: string): Set<string> {
  const vars = new Set<string>()
  // Match each rule whose selector list mentions the scope, capture its decls.
  const ruleRe = new RegExp(`${scope.replace('.', '\\.')}[^{]*\\{([^}]*)\\}`, 'g')
  let m: RegExpExecArray | null
  while ((m = ruleRe.exec(CSS)) !== null) {
    const decls = m[1]
    const varRe = /var\((--chat-[a-z-]+)/g
    let v: RegExpExecArray | null
    while ((v = varRe.exec(decls)) !== null) {
      vars.add(v[1])
    }
  }
  return vars
}

describe('events.css visual-customizer scope parity', () => {
  it('defines a live-overlay scope', () => {
    expect(CSS).toContain('.overlay-live-body')
  })

  it('honors every preview-consumed --chat-* variable on the live overlay', () => {
    const preview = varsConsumedUnderScope('.overlay-preview-body')
    const live = varsConsumedUnderScope('.overlay-live-body')

    expect(preview.size).toBeGreaterThan(0)

    const unhandled = [...preview].filter(
      (v) => !live.has(v) && !LIVE_EXCEPTIONS.has(v)
    )

    expect(
      unhandled,
      `These customizer variables apply in the preview but are not honored by ` +
        `the live overlay. Add a rule under \`.overlay-live-body\` in events.css, ` +
        `or document a React/structural exception in this test's LIVE_EXCEPTIONS.`
    ).toEqual([])
  })

  it('keeps emote scale wired up on the live overlay', () => {
    const live = varsConsumedUnderScope('.overlay-live-body')
    expect(live.has('--chat-emote-scale')).toBe(true)
  })

  /**
   * Regression guard for #438 (emotes overlap at larger sizes). Emote scale must
   * grow the layout box (height), NOT `transform: scale()` — a transform grows
   * the emote visually without reserving space, so scaled emotes overlap text and
   * wrap-lines. And the line box must be floored at the emote height so a tall
   * emote can't bleed into adjacent lines.
   */
  it('scales emotes via the layout box, never transform: scale()', () => {
    expect(CSS).not.toMatch(/transform:\s*scale\(var\(--chat-emote-scale/)
    // both scopes size the emote height by the scale factor
    const heightScaleRules = CSS.match(
      /height:\s*calc\(1\.4em \* var\(--chat-emote-scale, 1\)\)/g
    )
    expect(heightScaleRules?.length).toBe(2)
  })

  it('floors the message line-height at the scaled emote height', () => {
    const floorRules = CSS.match(
      /line-height:\s*max\(\s*calc\(var\(--chat-line-height, 1\.5\) \* 1em\),\s*calc\(1\.4em \* var\(--chat-emote-scale, 1\)\)\s*\)/g
    )
    expect(floorRules?.length).toBe(2)
  })
})
