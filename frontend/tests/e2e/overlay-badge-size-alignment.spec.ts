import { test, expect } from '@playwright/test'

/**
 * Badge size must not drift the name→message gap (regression for the reported
 * "when I change the badge size, the badge + name line shifts away from the
 * message"). The header row uses `align-items: center`, so a larger badge used
 * to grow the row and float the username up, away from the message. events.css
 * clamps each badge's *layout* box to the username line via symmetric block
 * margins so the visible badge overflows that box centred without changing the
 * row height.
 *
 * This drives a self-contained fixture that mirrors the `.overlay-live-body`
 * header markup + the clamp rules from src/styles/events.css (kept in sync with
 * that file). We measure the vertical gap between the username and the message
 * across badge sizes and assert it stays effectively constant.
 */

const FIXTURE = `<!doctype html>
<html>
<head><meta charset="utf-8"><style>
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body { font-family: system-ui, sans-serif; padding: 20px; }
  /* Minimal Tailwind-equivalents used by the real header row */
  .name-row { display: flex; flex-wrap: wrap; align-items: center; gap: 0.5rem; margin-bottom: 0.25rem; }
  .content { min-width: 0; flex: 1 1 0%; }
  .chat-username { font-size: var(--chat-username-font-size, 0.875rem); line-height: 1.25rem; font-weight: 600; }
  .break-words { font-size: 1rem; line-height: 1.5; }
  .platform-badge-icon { display: inline-flex; align-items: center; }
  .flex.gap-1 { display: flex; align-items: center; gap: 0.25rem; }

  /* --- Clamp rules copied verbatim from src/styles/events.css (.overlay-live-body). --- */
  .overlay-live-body .flex.gap-1 img {
    width: var(--chat-badge-size, 1rem) !important;
    height: var(--chat-badge-size, 1rem) !important;
    margin-top: calc((var(--chat-username-font-size, 0.875rem) - var(--chat-badge-size, 1rem)) / 2) !important;
    margin-bottom: calc((var(--chat-username-font-size, 0.875rem) - var(--chat-badge-size, 1rem)) / 2) !important;
  }
  .overlay-live-body .platform-badge-icon svg,
  .overlay-live-body .platform-badge-icon img {
    width: var(--chat-badge-size, 1rem) !important;
    height: var(--chat-badge-size, 1rem) !important;
    margin-top: calc((var(--chat-username-font-size, 0.875rem) - var(--chat-badge-size, 1rem)) / 2) !important;
    margin-bottom: calc((var(--chat-username-font-size, 0.875rem) - var(--chat-badge-size, 1rem)) / 2) !important;
  }
</style></head>
<body>
  <div class="overlay-live-body" style="--chat-username-font-size: 14px;">
    <div>
      <div class="content">
        <div class="name-row">
          <span class="platform-badge platform-badge-icon">
            <svg data-testid="badge" viewBox="0 0 24 24" width="16" height="16"><rect width="24" height="24" fill="#999"/></svg>
          </span>
          <span class="flex gap-1"><img data-testid="userbadge" src="data:image/gif;base64,R0lGODlhAQABAAAAACH5BAEKAAEALAAAAAABAAEAAAICTAEAOw==" width="16" height="16"/></span>
          <span class="chat-username" data-testid="username">Username</span>
        </div>
        <div class="break-words" data-testid="message">This is a chat message.</div>
      </div>
    </div>
  </div>
</body>
</html>`

async function gap(page: import('@playwright/test').Page, badgePx: number): Promise<number> {
  return page.evaluate((bp) => {
    const root = document.querySelector('.overlay-live-body') as HTMLElement
    root.style.setProperty('--chat-badge-size', `${bp}px`)
    // Force layout, then measure the gap from the username baseline area to the message.
    const username = document.querySelector('[data-testid="username"]')!.getBoundingClientRect()
    const message = document.querySelector('[data-testid="message"]')!.getBoundingClientRect()
    return message.top - username.bottom
  }, badgePx)
}

test.describe('badge size alignment', () => {
  test('name→message gap stays constant as badge size grows', async ({ page }) => {
    await page.setContent(FIXTURE)

    const small = await gap(page, 12)
    const dflt = await gap(page, 18)
    const large = await gap(page, 32)
    const huge = await gap(page, 48)

    // All four gaps must agree within a small tolerance: the badge overflows its
    // clamped box instead of pushing the message down. (Pre-fix, gap(48) was
    // ~17px larger than gap(12) because align-items:center floated the name up.)
    const gaps = [small, dflt, large, huge]
    const spread = Math.max(...gaps) - Math.min(...gaps)
    expect(spread).toBeLessThan(2)

    // And specifically: a big badge does not increase the gap vs the default.
    expect(Math.abs(large - dflt)).toBeLessThan(1.5)
    expect(Math.abs(huge - dflt)).toBeLessThan(1.5)
  })

  test('the badge still visibly scales with badge size', async ({ page }) => {
    await page.setContent(FIXTURE)
    const sizeAt = (bp: number) =>
      page.evaluate((b) => {
        const root = document.querySelector('.overlay-live-body') as HTMLElement
        root.style.setProperty('--chat-badge-size', `${b}px`)
        return document.querySelector('[data-testid="badge"]')!.getBoundingClientRect().height
      }, bp)
    expect(await sizeAt(16)).toBeCloseTo(16, 0)
    expect(await sizeAt(32)).toBeCloseTo(32, 0)
  })
})
