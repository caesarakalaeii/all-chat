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
import { joinPreviewCss, scopeCustomCss } from '../scope-css'
import { getBundledThemes } from '../bundled-themes'

const SCOPE = '.theme-preview-x'
const BODY = '.theme-preview-x .theme-preview-body'

describe('scopeCustomCss', () => {
  it('prefixes plain selectors with the preview scope', () => {
    expect(scopeCustomCss('.chat-message { color: red; }', SCOPE, BODY)).toContain(
      `${SCOPE} .chat-message {`
    )
  })

  it('rewrites :root and body to the preview scope', () => {
    const out = scopeCustomCss(':root { --x: 1; } body { background: blue; }', SCOPE, BODY)
    expect(out).toContain(`${SCOPE} {`)
    expect(out).toContain(`${BODY} {`)
  })

  it('leaves keyframe steps alone', () => {
    const out = scopeCustomCss(
      '@keyframes k { from { opacity: 0 } to { opacity: 1 } }',
      SCOPE,
      BODY
    )
    expect(out).not.toContain(`${SCOPE} from`)
    expect(out).not.toContain(`${SCOPE} to`)
  })

  /**
   * Regression guard. Selector detection treats everything between `}` and `{`
   * as a selector list and splits it on commas, so a comment above a rule was
   * absorbed into that rule's selector: a comment containing a comma (most of
   * them do) split into fragments and the rule that followed came out WITHOUT
   * the scope prefix. Theme CSS then escaped the preview container and restyled
   * the surrounding page — and in the theme-contrast harness, every other
   * theme's preview.
   */
  it('does not let a comma in a comment leak the next rule out of the scope', () => {
    const css = `/* tokens first: no bounce, no glow, no scale */
.event-message { --event-scale: 1; }`
    const out = scopeCustomCss(css, SCOPE, BODY)
    // Every occurrence of the rule must be the scoped one — no bare copy left.
    const total = out.match(/\.event-message\s*\{/g)?.length ?? 0
    const scoped =
      out.match(new RegExp(`\\.theme-preview-x \\.event-message\\s*\\{`, 'g'))?.length ?? 0
    expect(total).toBe(1)
    expect(scoped).toBe(1)
  })

  it('scopes every rule of every bundled theme', () => {
    for (const theme of getBundledThemes()) {
      const out = scopeCustomCss(theme.css, SCOPE, BODY)
      // Strip at-rule bodies' opening braces we don't scope (@media/@keyframes/@font-face/@import)
      const unscoped = out
        .split('\n')
        .filter((line) => /^\s*[.#\w[][^{}@]*\{\s*$/.test(line))
        .filter((line) => !line.includes(SCOPE))
        .filter((line) => !/^\s*(from|to|\d+%)/.test(line))
      expect(unscoped, `${theme.id} has rules that escape the preview scope`).toEqual([])
    }
  })
})

describe('joinPreviewCss', () => {
  it('hoists a user @import above the theme layer body (live-overlay parity)', () => {
    const theme = "@layer marketplace-themes {\n.chat-username { color: red; }\n}"
    const custom = "@import url('https://fonts.googleapis.com/css2?family=Caveat:wght@700&display=swap');\n.chat-username { text-shadow: none; }"
    const out = joinPreviewCss(theme, custom)
    const importIdx = out.indexOf('@import')
    const layerIdx = out.indexOf('@layer')
    expect(importIdx).toBeGreaterThanOrEqual(0)
    // The import must precede the layer body or the CSS parser drops it.
    expect(layerIdx).toBeGreaterThan(importIdx)
    expect(out).toContain('.chat-username { text-shadow: none; }')
  })

  it('keeps theme imports before user imports and drops empty tiers', () => {
    const theme = "@import url('/font-proxy/css?family=Theme');\n@layer marketplace-themes {\n.x { color: red; }\n}"
    const out = joinPreviewCss(theme, '')
    expect(out.indexOf("url('/font-proxy/css?family=Theme')")).toBeLessThan(
      out.indexOf('@layer')
    )
    expect(out).toContain('.x { color: red; }')
  })
})
