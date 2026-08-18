import { test, expect } from '@playwright/test'
import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { visualSettingsToCss } from '../../src/lib/utils/visual-settings-to-css'
import type { VisualSettings } from '../../src/lib/types/visual-settings'

/**
 * A theme must be able to remove chrome the platform adds by default.
 *
 * The visual-customizer rules live in `@layer visual-customizer` and are
 * `!important`, which by the cascade-layers spec beats a theme's *unlayered*
 * `!important`. That is deliberate — a customizer setting has to win over a
 * theme. The bug is that each declaration also carries the PLATFORM DEFAULT as
 * its `var()` fallback, so when a control is UNSET the platform default still
 * outranks the theme. The Minimal family asks for `padding: 0` (no bubbles at
 * all) and got 12px on every row regardless — 24px of vertical space per
 * message that the theme explicitly tried to delete.
 *
 * Intended precedence: customizer setting > theme intent > platform default.
 * A theme states its intent with `--theme-*`; the customizer's `--chat-*` still
 * wins whenever it is set.
 */

const REPO = join(__dirname, '..', '..', '..')
const EVENTS_CSS = readFileSync(join(REPO, 'frontend/src/styles/events.css'), 'utf8')
const THEME_CSS = readFileSync(join(REPO, 'docs/overlay-themes/minimal-theme.css'), 'utf8').replace(
  /@import[^;]+;/g,
  ''
)

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

function fixture(surface: string, settings: Partial<VisualSettings>): string {
  return `<!doctype html><html><head><meta charset="utf-8">
<style>@layer base, design-system, marketplace-themes, user-overrides;</style>
<style>${TAILWIND}</style>
<style>${EVENTS_CSS}</style>
<style>${visualSettingsToCss(settings)}</style>
<style>${THEME_CSS}</style>
</head><body><div class="${surface} space-y-3 p-4">${ROW}${ROW}</div></body></html>`
}

async function measure(
  page: import('@playwright/test').Page,
  surface: string,
  settings: Partial<VisualSettings>
) {
  await page.setContent(fixture(surface, settings))
  return page.evaluate(() => {
    const rows = [...document.querySelectorAll('[data-row]')]
    const cs = getComputedStyle(rows[0] as HTMLElement)
    const r = rows.map((e) => e.getBoundingClientRect())
    return {
      paddingTop: cs.paddingTop,
      borderRadius: cs.borderRadius,
      backdropFilter: cs.backdropFilter,
      pitch: +(r[1].top - r[0].top).toFixed(1),
    }
  })
}

for (const surface of ['overlay-live-body', 'overlay-preview-body']) {
  test.describe(`theme intent vs platform default — ${surface}`, () => {
    test('an unset control lets the theme delete the bubble chrome', async ({ page }) => {
      const m = await measure(page, surface, {})
      // The Minimal theme is a no-bubble theme: no padding, no rounding, no blur.
      expect(m.paddingTop).toBe('0px')
      expect(m.borderRadius).toBe('0px')
      expect(m.backdropFilter === 'none' || m.backdropFilter === 'blur(0px)').toBe(true)
    })

    test('an explicitly set control still beats the theme', async ({ page }) => {
      const m = await measure(page, surface, { bubblePadding: '10px' })
      expect(m.paddingTop).toBe('10px')
    })

    test('theme-zeroed padding removes 24px of vertical space per message', async ({ page }) => {
      const themed = await measure(page, surface, { messageGap: '0px' })
      const forced = await measure(page, surface, { messageGap: '0px', bubblePadding: '12px' })
      expect(forced.pitch - themed.pitch).toBeCloseTo(24, 0)
    })
  })
}
