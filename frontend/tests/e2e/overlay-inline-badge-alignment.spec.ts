import { test, expect } from '@playwright/test'
import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { visualSettingsToCss } from '../../src/lib/utils/visual-settings-to-css'
import { wrapThemeCss } from '../../src/lib/theme-marketplace/wrap-theme-css'

/**
 * In an inline-layout theme ("[icon] [badges] username: message" on one line)
 * the platform icon and badges must stay centred on the username at every
 * username size, not only at the 16px default.
 *
 * The inline themes make the username header `display: inline` and centre the
 * icon and the badge group with `vertical-align: middle`. `middle` resolves
 * against the x-height of the PARENT, which is that header — and the header
 * inherited the page's 16px, not the username's size. Reported at 26px
 * username/body/badge size: the icons sat on a 16px midline and hung ~9px below
 * the baseline instead of ~6px, visibly lower than the name beside them.
 *
 * The measure is the icon's offset from the username's centre, in ems of the
 * username size. At the default 16px that offset is the accepted look, so the
 * assertion is that it stays the same at larger sizes. Font-independent: the
 * fallback font changes the offset itself, never whether it is constant.
 */

const REPO = join(__dirname, '..', '..', '..')
const EVENTS_CSS = readFileSync(join(REPO, 'frontend/src/styles/events.css'), 'utf8')
const theme = (file: string) =>
  wrapThemeCss(
    readFileSync(join(REPO, 'docs/overlay-themes', file), 'utf8').replace(/@import[^;]+;/g, '')
  )

// Tailwind's own layers come first in the real app (globals.css imports
// tailwindcss before declaring its layers), so utilities rank BELOW the theme
// and the customizer. Unlayered, they would outrank both.
const PREFLIGHT = `
  *, ::before, ::after { box-sizing: border-box; margin: 0; padding: 0; }
  html { line-height: 1.5; font-family: system-ui, sans-serif; }
`

const TAILWIND = `
  .p-4 { padding: 1rem; }
  .mb-1 { margin-bottom: 0.25rem; }
  .flex { display: flex; }
  .inline-block { display: inline-block; }
  .flex-wrap { flex-wrap: wrap; }
  .items-start { align-items: flex-start; }
  .items-center { align-items: center; }
  .gap-1 { gap: 0.25rem; }
  .gap-2 { gap: 0.5rem; }
  .gap-3 { gap: 0.75rem; }
  .w-4 { width: 1rem; }
  .h-4 { height: 1rem; }
  .min-w-0 { min-width: 0; }
  .flex-1 { flex: 1 1 0%; }
  .text-sm { font-size: 0.875rem; line-height: 1.25rem; }
  .font-semibold { font-weight: 600; }
  .break-words { overflow-wrap: break-word; }
  .text-white { color: #fff; }
`

const BADGE =
  "data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 18 18'%3E%3Crect width='18' height='18' fill='%23e91916'/%3E%3C/svg%3E"

// Mirrors the live overlay's header markup (src/app/overlay/[id]/page.tsx).
const ROW = `
<div class="chat-message rounded-lg p-3">
  <div class="flex items-start gap-3">
    <div class="min-w-0 flex-1">
      <div class="mb-1 flex flex-wrap items-center gap-2">
        <span class="platform-badge platform-badge-icon flex items-center gap-0.5">
          <svg data-testid="icon" viewBox="0 0 24 24" class="inline-block w-4 h-4"><rect width="24" height="24" fill="#9146FF"/></svg>
        </span>
        <div class="flex items-center gap-1">
          <img data-testid="badge" src="${BADGE}" width="18" height="18" class="h-[1em] w-auto object-contain">
        </div>
        <span class="chat-username text-sm font-semibold" data-testid="username">Xeniatro</span>
      </div>
      <div class="break-words text-white">a</div>
    </div>
  </div>
</div>`

function fixture(surface: string, themeFile: string, px: number): string {
  const size = `${px}px`
  const settings = { usernameFontSize: size, fontSize: size, badgeSize: size }
  return `<!doctype html><html><head><meta charset="utf-8">
<style>@layer theme, base, components, utilities;
@layer base, design-system, marketplace-themes, user-overrides;</style>
<style>@layer base { ${PREFLIGHT} } @layer utilities { ${TAILWIND} }</style>
<style>${EVENTS_CSS}</style>
<style>${visualSettingsToCss(settings)}</style>
<style>${theme(themeFile)}</style>
</head><body><div class="${surface} space-y-3 p-4">${ROW}</div></body></html>`
}

/** Vertical offset of icon and badge centres below the username centre, in ems. */
async function offsets(
  page: import('@playwright/test').Page,
  surface: string,
  themeFile: string,
  px: number
) {
  await page.setContent(fixture(surface, themeFile, px))
  return page.evaluate((size) => {
    const mid = (id: string) => {
      const r = document.querySelector(`[data-testid="${id}"]`)!.getBoundingClientRect()
      return r.top + r.height / 2
    }
    const name = mid('username')
    return { icon: (mid('icon') - name) / size, badge: (mid('badge') - name) / size }
  }, px)
}

for (const surface of ['overlay-live-body', 'overlay-preview-body']) {
  for (const themeFile of ['minimal-theme.css', 'noita-minimal-theme.css']) {
    test(`${themeFile} on ${surface}: icon and badges stay centred on the username at any size`, async ({
      page,
    }) => {
      const base = await offsets(page, surface, themeFile, 16)
      for (const px of [26, 40]) {
        const o = await offsets(page, surface, themeFile, px)
        expect(o.icon, `platform icon at ${px}px vs 16px`).toBeCloseTo(base.icon, 1)
        expect(o.badge, `badge at ${px}px vs 16px`).toBeCloseTo(base.badge, 1)
      }
      // Coarse sanity bound on the reference itself. The exact offset depends on
      // the fallback font's metrics; the invariance above is the real guard.
      expect(Math.abs(base.icon)).toBeLessThan(0.15)
      expect(Math.abs(base.badge)).toBeLessThan(0.15)
    })
  }
}
