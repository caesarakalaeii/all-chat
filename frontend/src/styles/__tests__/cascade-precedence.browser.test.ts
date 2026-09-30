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

import { beforeEach, describe, expect, it } from 'vitest'
import { page } from '@vitest/browser/context'

/**
 * Real-Chromium guard for the overlay CSS precedence hierarchy:
 *
 *   1. Manual CSS (unlayered)
 *   2. GUI settings (`@layer visual-customizer`)
 *   3. Theme (`@layer marketplace-themes`, wrapped at injection)
 *
 * The unit suite (cascade-precedence.test.ts) pins the source shape: no
 * `!important` in the GUI chain, themes wrapped and stripped at injection.
 * This test pins the COMPUTED result, because the hierarchy is produced by
 * the interplay of stylesheets the unit tests never load together: the
 * Tailwind layer stack, events.css, the injected theme CSS and the injected
 * GUI CSS. A regression in any of them — a stray `!important` in the GUI
 * emitter, an unwrapped theme, a reordered `@layer` statement — only shows
 * up as a computed style. That exact class of drift (theme `!important`
 * beating everything, GUI inline styles losing to theme declarations) made
 * the Text Shadow, padding and background controls inert in OBS.
 *
 * The fixture reproduces the compiled app's style order: layer declarations
 * first, then Tailwind utilities, then the theme (as wrapThemeCss emits it),
 * then the GUI block (as visualSettingsToCss emits it), then the user's
 * manual CSS — unlayered and last, so it wins at normal weight with no
 * `!important` of its own.
 */

const FIXTURE_ID = 'cascade-precedence-fixture'

function mountFixture(): void {
  document.getElementById(FIXTURE_ID)?.remove()

  // Layer order statements exactly as the compiled overlay page emits them:
  // Tailwind's stack (index.css), then globals.css, then events.css.
  const style = document.createElement('style')
  style.textContent = `
@layer theme, base, components, utilities;
@layer base, design-system, marketplace-themes, user-overrides;
@layer base, design-system, marketplace-themes, user-overrides, visual-customizer;
@layer utilities { .cascade-row { font-weight: 400; background-color: #e2e8f0; } }
@layer marketplace-themes { .cascade-row { font-weight: 500; background-color: #aabbcc; } }
@layer visual-customizer { .cascade-row { font-weight: 700; background-color: #ff00ff; } }
.cascade-row.manual-win { font-weight: 800; }
.cascade-row.manual-normal { letter-spacing: 9px; }
.cascade-row.sentinel { padding: 0 !important; }
@layer visual-customizer { .cascade-row.sentinel { padding: 12px; } }
`

  const root = document.createElement('div')
  root.id = FIXTURE_ID
  root.append(style)
  root.insertAdjacentHTML(
    'beforeend',
    `
<div class="cascade-row" data-testid="cascade-gui-only">x</div>
<div class="cascade-row manual-win" data-testid="cascade-manual">x</div>
<div class="cascade-row manual-normal" data-testid="cascade-manual-normal">x</div>
<div class="cascade-row sentinel" data-testid="cascade-sentinel">x</div>
`
  )
  document.body.append(root)
}

beforeEach(mountFixture)

function styleOf(testId: string, property: string): string {
  const el = page.getByTestId(testId).element()
  return getComputedStyle(el).getPropertyValue(property)
}

describe('overlay CSS precedence hierarchy (real Chromium)', () => {
  it('GUI settings beat the theme layer and Tailwind utilities', () => {
    // Theme was wrapped into marketplace-themes (importants stripped), so
    // the GUI layer wins by LAYER ORDER at normal weight.
    expect(styleOf('cascade-gui-only', 'font-weight')).toBe('700')
    expect(styleOf('cascade-gui-only', 'background-color')).toBe('rgb(255, 0, 255)')
  })

  it('manual CSS beats GUI settings at normal weight, no !important needed', () => {
    expect(styleOf('cascade-manual', 'font-weight')).toBe('800')
    expect(styleOf('cascade-manual-normal', 'letter-spacing')).toBe('9px')
  })

  it('unlayered !important (sentinel reset in globals.css) still beats the GUI layer', () => {
    // The one deliberate unlayered-important in the app: globals.css resets
    // the auto-scroll sentinel. GUI rules key on rows, so they must lose to
    // it — events.css excludes the sentinel by selector, and this asserts
    // the same order from the other side.
    expect(styleOf('cascade-sentinel', 'padding-top')).toBe('0px')
  })
})
