import { test, expect } from '@playwright/test'

/**
 * Overlay Observability View (/overlay/[id]/view)
 *
 * Frontend smoke tests (no backend, matching overlay-preview.spec.ts):
 * - Page renders the dashboard chrome (header, Chat + Activity panels)
 * - Solid background + own light/dark mode (ignores overlay themes)
 * - Light/dark toggle persists to localStorage
 * - Opens a public /ws/overlay/<id> WebSocket (no ?token=)
 * - Resizable split persists its ratio (desktop only)
 * - Links back to the OBS overlay
 *
 * Deep message-flow behavior (chat vs activity partitioning, moderation log)
 * is covered by unit/component tests; it requires the full stack to exercise
 * end-to-end and is verified manually via `make frontend-quick`.
 */

const OVERLAY_ID = 'test-overlay-id'
const VIEW_URL = `/overlay/${OVERLAY_ID}/view`

test.describe('Overlay Observability View', () => {
  test('renders the dashboard chrome', async ({ page }) => {
    await page.goto(VIEW_URL)
    await expect(page.locator('#overlay-view-root')).toBeVisible()
    await expect(page.getByText('Chat', { exact: true })).toBeVisible()
    await expect(page.getByText('Activity & Events')).toBeVisible()
    await expect(page.getByRole('link', { name: /OBS overlay/i })).toHaveAttribute(
      'href',
      `/overlay/${OVERLAY_ID}`
    )
  })

  test('has a solid (non-transparent) background', async ({ page }) => {
    await page.goto(VIEW_URL)
    const bg = await page.evaluate(() => getComputedStyle(document.body).backgroundColor)
    // Layout forces a solid dark body background; OBS overlay would be transparent.
    expect(bg).not.toBe('rgba(0, 0, 0, 0)')
    expect(bg).not.toBe('transparent')
  })

  test('toggles light/dark mode and persists the choice', async ({ page }) => {
    await page.goto(VIEW_URL)
    const root = page.locator('#overlay-view-root')
    await expect(root).not.toHaveClass(/\blight\b/)

    await page.getByRole('button', { name: /Light/i }).click()
    await expect(root).toHaveClass(/\blight\b/)
    await expect(page.getByRole('button', { name: /Dark/i })).toBeVisible()

    const stored = await page.evaluate(() => localStorage.getItem('overlay-view-theme'))
    expect(stored).toBe('light')
  })

  test('opens a public overlay WebSocket without a token', async ({ page }) => {
    const wsUrls: string[] = []
    page.on('websocket', (ws) => wsUrls.push(ws.url()))

    await page.goto(VIEW_URL)
    await page.waitForTimeout(2000)

    const overlayWs = wsUrls.find((u) => u.includes(`/ws/overlay/${OVERLAY_ID}`))
    expect(overlayWs).toBeTruthy()
    expect(overlayWs).not.toContain('token=')
  })

  test('persists the resizable split ratio (desktop)', async ({ page }) => {
    await page.goto(VIEW_URL)
    const separator = page.getByRole('separator', { name: /resize/i })

    // The divider is hidden on mobile viewports (stacked layout) — skip there.
    if (!(await separator.isVisible().catch(() => false))) {
      test.skip(true, 'divider hidden on mobile viewport')
      return
    }

    await separator.focus()
    await separator.press('ArrowRight')

    const ratio = await page.evaluate(
      (id) => localStorage.getItem(`overlay-view-split-${id}`),
      OVERLAY_ID
    )
    expect(ratio).not.toBeNull()
  })
})
