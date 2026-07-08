import { test, expect } from '@playwright/test'

/**
 * Theme readability checker
 *
 * Renders every theme in the home-page theme switcher and asserts the message
 * text keeps a usable contrast ratio against its (composited) background. This
 * catches the "white-on-white" / same-colour class of theme bug automatically,
 * instead of eyeballing each theme by hand.
 *
 * It runs against the real app (Playwright's webServer), so the full CSS cascade
 * is in effect — including the events.css visual-customizer @layer whose layered
 * `!important` on `--chat-message-color` is exactly what made a couple of themes
 * render invisible text. Message text is theme-controlled, so it's the element we
 * gate; usernames are viewer-defined colours and intentionally not checked.
 *
 * Run just this check with: `npm run test:themes`.
 */

// WCAG-ish floor. White-on-white is ~1:1; a genuinely readable message clears this
// comfortably. Kept at 3.0 so stylistic-but-legible themes don't false-positive.
const MIN_MESSAGE_CONTRAST = 3.0

type RGBA = { r: number; g: number; b: number; a: number }
type Worst = { ratio: number; color: string; bg: string }

/**
 * Runs in the page. For the currently-active preview, returns the worst message-
 * text contrast ratio: text colour vs the background composited up the ancestor
 * chain (solid backgroundColors + averaged gradient colour-stops) over the
 * preview's black base. Type annotations are stripped when Playwright serialises
 * this into the browser.
 */
function measureActivePreview(): { worst: Worst | null } {
  const parse = (c: string | null | undefined): RGBA | null => {
    const m = c ? c.match(/rgba?\(([^)]+)\)/) : null
    if (!m) return null
    const p = m[1].split(',').map((s) => parseFloat(s))
    return { r: p[0], g: p[1], b: p[2], a: p[3] === undefined ? 1 : p[3] }
  }
  const over = (fg: RGBA, bg: RGBA): RGBA => {
    const a = fg.a + bg.a * (1 - fg.a)
    if (a === 0) return { r: 0, g: 0, b: 0, a: 0 }
    return {
      r: (fg.r * fg.a + bg.r * bg.a * (1 - fg.a)) / a,
      g: (fg.g * fg.a + bg.g * bg.a * (1 - fg.a)) / a,
      b: (fg.b * fg.a + bg.b * bg.a * (1 - fg.a)) / a,
      a,
    }
  }
  const layer = (el: Element): RGBA => {
    const cs = getComputedStyle(el)
    let result: RGBA = parse(cs.backgroundColor) ?? { r: 0, g: 0, b: 0, a: 0 }
    if (cs.backgroundImage && cs.backgroundImage !== 'none') {
      const stops = Array.from(cs.backgroundImage.matchAll(/rgba?\([^)]+\)/g))
        .map((mm) => parse(mm[0]))
        .filter((x): x is RGBA => x !== null)
      if (stops.length > 0) {
        const s = stops.reduce<RGBA>(
          (acc, c) => ({ r: acc.r + c.r, g: acc.g + c.g, b: acc.b + c.b, a: acc.a + c.a }),
          { r: 0, g: 0, b: 0, a: 0 }
        )
        const grad: RGBA = {
          r: s.r / stops.length,
          g: s.g / stops.length,
          b: s.b / stops.length,
          a: s.a / stops.length,
        }
        result = over(grad, result)
      }
    }
    return result
  }
  const effBg = (el: Element): RGBA => {
    const chain: Element[] = []
    let n: Element | null = el
    while (n && n.nodeType === 1) {
      chain.push(n)
      n = n.parentElement
    }
    let base: RGBA = { r: 0, g: 0, b: 0, a: 1 } // preview's black base
    for (let i = chain.length - 1; i >= 0; i--) {
      const lc = layer(chain[i])
      if (lc.a > 0) base = over(lc, base)
    }
    return base
  }
  const lum = (c: RGBA): number => {
    const f = (v: number): number => {
      const x = v / 255
      return x <= 0.03928 ? x / 12.92 : Math.pow((x + 0.055) / 1.055, 2.4)
    }
    return 0.2126 * f(c.r) + 0.7152 * f(c.g) + 0.0722 * f(c.b)
  }
  const ratio = (a: RGBA, b: RGBA): number => {
    const l1 = lum(a)
    const l2 = lum(b)
    return (Math.max(l1, l2) + 0.05) / (Math.min(l1, l2) + 0.05)
  }
  const fmt = (c: RGBA): string => `rgb(${Math.round(c.r)},${Math.round(c.g)},${Math.round(c.b)})`

  const preview = document.querySelector('[data-theme-preview]')
  if (!preview) return { worst: null }
  const msgs = Array.from(preview.querySelectorAll('.break-words'))
  let worst: Worst | null = null
  for (const el of msgs) {
    const cs = getComputedStyle(el)
    const color: RGBA = parse(cs.color) ?? { r: 0, g: 0, b: 0, a: 1 }
    const bg = effBg(el)
    const r = ratio(over(color, bg), bg)
    if (!worst || r < worst.ratio) worst = { ratio: r, color: fmt(color), bg: fmt(bg) }
  }
  return { worst }
}

test.describe('Theme readability', () => {
  test('every homepage theme has readable message text', async ({ page }) => {
    await page.goto('/')
    const group = page.getByRole('group', { name: 'Preview a theme' })
    await expect(group).toBeVisible()

    const pills = group.getByRole('button')
    const count = await pills.count()
    expect(count).toBeGreaterThan(0)

    const failures: string[] = []
    const summary: string[] = []

    for (let i = 0; i < count; i++) {
      const pill = pills.nth(i)
      const label = ((await pill.textContent()) ?? `#${i}`).trim()

      await pill.click()
      await expect(pill).toHaveAttribute('aria-pressed', 'true')

      // Wait until the active theme's scoped <style> is actually applied.
      const themeId = await page.locator('[data-theme-preview]').getAttribute('data-theme-preview')
      if (themeId) {
        await page.waitForFunction((id: string) => {
          const s = document.querySelector(`style[data-theme-id="${id}"]`)
          return !!s && !!s.textContent && s.textContent.includes(`theme-preview-${id}`)
        }, themeId)
      }

      const { worst } = await page.evaluate(measureActivePreview)
      if (!worst) {
        failures.push(`${label}: no message text found to measure`)
        continue
      }
      summary.push(`  ${label.padEnd(16)} ${worst.ratio.toFixed(2)}:1  (${worst.color} on ${worst.bg})`)
      if (worst.ratio < MIN_MESSAGE_CONTRAST) {
        failures.push(
          `${label}: message contrast ${worst.ratio.toFixed(2)}:1 — text ${worst.color} on ${worst.bg}`
        )
      }
    }

    console.log(`\nTheme message-text contrast (floor ${MIN_MESSAGE_CONTRAST}:1):\n${summary.join('\n')}\n`)
    expect(failures, `\nLow-contrast themes found:\n${failures.join('\n')}\n`).toEqual([])
  })
})
