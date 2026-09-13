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

import { wrapThemeCss } from '../wrap-theme-css'

describe('wrapThemeCss', () => {
  it('returns empty string for empty/whitespace input', () => {
    expect(wrapThemeCss('')).toBe('')
    expect(wrapThemeCss('   \n  ')).toBe('')
  })

  it('wraps plain rules in @layer marketplace-themes', () => {
    const out = wrapThemeCss('.chat-username { color: red; }')
    expect(out).toBe('@layer marketplace-themes {\n.chat-username { color: red; }\n}')
  })

  it('strips every !important (layered-important would beat GUI-normal AND unlayered manual-important)', () => {
    const out = wrapThemeCss(
      '.a { color: red !important; }\n.b { padding: 0 !IMPORTANT ; }\n.c { margin: 1px!important; }'
    )
    expect(out).not.toContain('important')
    expect(out).toContain('.a { color: red; }')
    expect(out).toContain('.b { padding: 0 ; }')
    expect(out).toContain('.c { margin: 1px; }')
  })

  it('hoists @import above the layer body (grammar requires imports first)', () => {
    const out = wrapThemeCss(
      "@import url('/font-proxy/css?family=Test');\n.chat-username { color: red !important; }"
    )
    const importIdx = out.indexOf('@import')
    const layerIdx = out.indexOf('@layer')
    expect(importIdx).toBeGreaterThanOrEqual(0)
    expect(layerIdx).toBeGreaterThan(importIdx)
    expect(out).not.toContain('important')
  })

  it('hoists interleaved @import statements in source order', () => {
    const out = wrapThemeCss(
      "@import url('/a');\n.x { color: red; }\n@import url('/b');\n.y { color: blue; }"
    )
    expect(out.indexOf("url('/a')")).toBeLessThan(out.indexOf("url('/b')"))
    expect(out.indexOf("url('/b')")).toBeLessThan(out.indexOf('@layer'))
    // body keeps rule order
    expect(out.indexOf('.x')).toBeLessThan(out.indexOf('.y'))
  })

  it('keeps Google Fonts css2 URLs intact (weight-axis semicolons are part of the URL)', () => {
    const out = wrapThemeCss(
      "@import url('https://fonts.googleapis.com/css2?family=Poppins:wght@400;500;600&display=swap');\n.chat-username { color: red; }"
    )
    expect(out).toContain(
      "@import url('https://fonts.googleapis.com/css2?family=Poppins:wght@400;500;600&display=swap');"
    )
    expect(out).toContain('@layer marketplace-themes')
    // The import must be hoisted whole and nothing may leak into the layer
    // body as garbage tokens (the old `[^;]+` match truncated at the first
    // weight axis).
    const layerBody = out.slice(out.indexOf('@layer'))
    expect(out.indexOf('display=swap')).toBeLessThan(out.indexOf('@layer'))
    expect(layerBody).not.toContain('500;600')
    expect(layerBody).not.toContain('display=swap')
  })

  it('strips the spec-legal `! important` whitespace variant (layered-important would beat manual CSS)', () => {
    const out = wrapThemeCss('.d { margin: 1px ! important; }')
    expect(out).not.toContain('important')
    expect(out).toContain('.d { margin: 1px; }')
  })
})
