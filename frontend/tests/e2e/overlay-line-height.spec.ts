import { test, expect } from '@playwright/test'
import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { visualSettingsToCss } from '../../src/lib/utils/visual-settings-to-css'
import type { VisualSettings } from '../../src/lib/types/visual-settings'

/**
 * Line Height must be able to make chat lines TIGHTER, not just looser.
 *
 * Reported against the Minimal Clean theme: setting Line Height to 1 changed
 * nothing. The minimal family lays a message out inline ("username: message"),
 * so the whole row is one line box inside the block content container
 * (`.min-w-0.flex-1`). A line box is floored by its block container's *strut* —
 * and the strut takes its line-height from Tailwind's preflight (`1.5`), which
 * no customizer rule used to touch. So `--chat-line-height` shrank the inline
 * spans while the strut held the row at 24px: the control was one-way.
 *
 * The fixture drives the REAL events.css + the REAL bundled theme CSS (read from
 * source) so this asserts the shipped cascade rather than a restatement of it.
 */

const REPO = join(__dirname, '..', '..', '..')
const EVENTS_CSS = readFileSync(join(REPO, 'frontend/src/styles/events.css'), 'utf8')

/** Theme CSS as shipped; the @import is stripped so the test needs no network. */
function themeCss(id: string): string {
  return readFileSync(join(REPO, `docs/overlay-themes/${id}.css`), 'utf8').replace(
    /@import[^;]+;/g,
    ''
  )
}

/**
 * The Tailwind declarations the overlay actually depends on here. `line-height:
 * 1.5` on the root is Tailwind v4 preflight — it is the strut this test is about.
 */
const TAILWIND = `
  *, ::before, ::after { box-sizing: border-box; margin: 0; padding: 0; }
  html { line-height: 1.5; font-family: system-ui, sans-serif; }
  .p-3 { padding: 0.75rem; }
  .p-4 { padding: 1rem; }
  .mb-1 { margin-bottom: 0.25rem; }
  .flex { display: flex; }
  .flex-wrap { flex-wrap: wrap; }
  .items-start { align-items: flex-start; }
  .items-center { align-items: center; }
  .gap-2 { gap: 0.5rem; }
  .gap-3 { gap: 0.75rem; }
  .min-w-0 { min-width: 0; }
  .flex-1 { flex: 1 1 0%; }
  .text-sm { font-size: 0.875rem; line-height: 1.25rem; }
  .font-semibold { font-weight: 600; }
  .break-words { overflow-wrap: break-word; }
  .text-white { color: #fff; }
  .rounded-lg { border-radius: 0.5rem; }
  .space-y-3 > :not(:last-child) { margin-block-end: 0.75rem; }
`

/** Mirrors the live overlay's message markup (src/app/overlay/[id]/page.tsx). */
const ROW = `
<div class="chat-message rounded-lg p-3" data-row>
  <div class="flex items-start gap-3">
    <div class="min-w-0 flex-1">
      <div class="mb-1 flex flex-wrap items-center gap-2">
        <span class="chat-username text-sm font-semibold">RetroMod</span>
      </div>
      <div class="break-words text-white" style="font-size:16px">Welcome to the overlay!</div>
    </div>
  </div>
</div>`

function fixture(theme: string, settings: Partial<VisualSettings>): string {
  return `<!doctype html><html><head><meta charset="utf-8">
<style>@layer base, design-system, marketplace-themes, user-overrides;</style>
<style>${TAILWIND}</style>
<style>${EVENTS_CSS}</style>
<style>${visualSettingsToCss(settings)}</style>
<style>${themeCss(theme)}</style>
</head><body>
  <div class="min-h-screen p-4">
    <div class="overlay-live-body space-y-3">${ROW}${ROW}</div>
  </div>
</body></html>`
}

/** Distance from one message row's top to the next — the visible line distance. */
async function rowPitch(
  page: import('@playwright/test').Page,
  theme: string,
  lineHeight: string
): Promise<number> {
  await page.setContent(fixture(theme, { lineHeight, messageGap: '8px' }))
  return page.evaluate(() => {
    const [a, b] = [...document.querySelectorAll('[data-row]')].map((e) =>
      e.getBoundingClientRect()
    )
    return +(b.top - a.top).toFixed(2)
  })
}

for (const theme of ['minimal-theme']) {
  test.describe(`line height — ${theme}`, () => {
    test('tightening Line Height below the default actually tightens the rows', async ({
      page,
    }) => {
      const dflt = await rowPitch(page, theme, '1.5')
      const tight = await rowPitch(page, theme, '1')

      // The reported bug: these were equal, because the block strut held the
      // line box at 24px no matter how small the inline line-height got.
      expect(tight).toBeLessThan(dflt)
    })

    test('Line Height stays monotonic across the slider range', async ({ page }) => {
      // Sequential on purpose: every measurement re-renders the same page.
      const pitches: number[] = []
      for (const lh of ['1', '1.2', '1.5', '2', '2.5']) {
        pitches.push(await rowPitch(page, theme, lh))
      }
      for (let i = 1; i < pitches.length; i++) {
        expect(pitches[i]).toBeGreaterThan(pitches[i - 1])
      }
    })
  })
}
