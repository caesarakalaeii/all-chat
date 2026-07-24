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
  computeThemeCssDiff,
  reconstructEditorCss,
  DIFF_MARKER,
  FORK_MARKER,
} from '../theme-css-diff'

const THEME = `.chat-username {
  color: red;
  font-size: 16px;
  font-weight: 700;
}
.chat-message .break-words {
  color: white;
}
@keyframes fadeIn {
  from { opacity: 0; }
  to { opacity: 1; }
}`

describe('computeThemeCssDiff', () => {
  it('is "linked" when the editor equals the theme (untouched preload)', async () => {
    const r = await computeThemeCssDiff(THEME, THEME)
    expect(r.mode).toBe('linked')
    expect(r.stored).toBe('')
  })

  it('is "linked" for an empty editor', async () => {
    expect((await computeThemeCssDiff(THEME, '')).mode).toBe('linked')
  })

  it('is "diff" and stores ONLY the changed declaration (with !important)', async () => {
    const edited = THEME.replace('color: red;', 'color: gold;')
    const r = await computeThemeCssDiff(THEME, edited)
    expect(r.mode).toBe('diff')
    expect(r.stored.startsWith(DIFF_MARKER)).toBe(true)
    expect(r.stored).toContain('color: gold !important')
    // Untouched declarations are NOT stored — they keep tracking the bundle.
    expect(r.stored).not.toContain('font-size')
    expect(r.stored).not.toContain('font-weight')
  })

  it('is "diff" when a brand-new rule is added', async () => {
    const edited = THEME + '\n.chat-message { background: rebeccapurple; }'
    const r = await computeThemeCssDiff(THEME, edited)
    expect(r.mode).toBe('diff')
    expect(r.stored).toContain('.chat-message')
    expect(r.stored).toContain('background: rebeccapurple !important')
  })

  it('is "fork" when a declaration is deleted (layering cannot un-set it)', async () => {
    const edited = THEME.replace('  font-weight: 700;\n', '')
    const r = await computeThemeCssDiff(THEME, edited)
    expect(r.mode).toBe('fork')
    expect(r.stored.startsWith(FORK_MARKER)).toBe(true)
    expect(r.stored).toContain(edited)
  })

  it('is "fork" when a whole rule is removed', async () => {
    const edited = THEME.replace(/\.chat-message \.break-words \{[^}]*\}/, '')
    const r = await computeThemeCssDiff(THEME, edited)
    expect(r.mode).toBe('fork')
  })

  it('is "fork" when a theme @keyframes is removed', async () => {
    const edited = THEME.replace(/@keyframes fadeIn \{[\s\S]*?\}\s*\}/, '')
    const r = await computeThemeCssDiff(THEME, edited)
    expect(r.mode).toBe('fork')
  })

  it('treats hand-written CSS with no theme as a fork override', async () => {
    expect((await computeThemeCssDiff('', '.x { color: red; }')).mode).toBe('fork')
    expect((await computeThemeCssDiff('', '')).mode).toBe('linked')
  })
})

describe('reconstructEditorCss', () => {
  it('preloads the theme when nothing is stored (linked)', async () => {
    const r = await reconstructEditorCss(THEME, '')
    expect(r.mode).toBe('linked')
    expect(r.editor).toBe(THEME)
  })

  it('shows a fork verbatim', async () => {
    const forkStored = FORK_MARKER + '\n' + '.chat-username { color: pink; }'
    const r = await reconstructEditorCss(THEME, forkStored)
    expect(r.mode).toBe('fork')
    expect(r.editor).toBe('.chat-username { color: pink; }')
  })

  it('treats legacy (unmarked) custom_css as a verbatim fork', async () => {
    const legacy = '.chat-username { color: cyan; }'
    const r = await reconstructEditorCss(THEME, legacy)
    expect(r.mode).toBe('fork')
    expect(r.editor).toBe(legacy)
  })

  it('merges a stored diff back onto the current theme', async () => {
    const delta = DIFF_MARKER + '\n.chat-username {\n  color: gold !important;\n}\n'
    const r = await reconstructEditorCss(THEME, delta)
    expect(r.mode).toBe('diff')
    // User's change is present…
    expect(r.editor).toContain('color: gold')
    // …and the untouched theme declarations survive (so the user sees the whole theme).
    expect(r.editor).toContain('font-size: 16px')
    expect(r.editor).toContain('@keyframes fadeIn')
  })

  it('reflects a theme update on reload for a rule the user did not touch', async () => {
    // User changed only the username colour.
    const { stored } = await computeThemeCssDiff(THEME, THEME.replace('color: red;', 'color: gold;'))
    // We later ship a theme fix: message text becomes offwhite.
    const updatedTheme = THEME.replace('color: white;', 'color: offwhite;')
    const r = await reconstructEditorCss(updatedTheme, stored)
    expect(r.editor).toContain('color: gold') // user's change kept
    expect(r.editor).toContain('color: offwhite') // shipped theme fix flows in
  })
})

describe('diff → reconstruct → diff round-trip', () => {
  it('is stable for change + add edits', async () => {
    const edited = THEME.replace('color: red;', 'color: gold;') + '\n.foo { display: none; }'
    const first = await computeThemeCssDiff(THEME, edited)
    expect(first.mode).toBe('diff')

    const { editor } = await reconstructEditorCss(THEME, first.stored)
    const second = await computeThemeCssDiff(THEME, editor)
    expect(second.mode).toBe('diff')
    // Same changes survive the round-trip.
    expect(second.stored).toContain('color: gold !important')
    expect(second.stored).toContain('.foo')
    expect(second.stored).toContain('display: none !important')
    // Still no untouched declarations leak into storage.
    expect(second.stored).not.toContain('font-weight')
  })
})
