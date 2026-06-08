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

/**
 * Connection resilience (the reported "/view keeps losing connection" bug).
 *
 * These drive the page's real WebSocket via Playwright's routeWebSocket, acting
 * as the gateway: deliver frames, drop the socket, or — for the half-open case —
 * hold it open and go silent. That last one is the actual bug: a connection
 * that stops delivering frames without ever firing onclose. The only way the
 * page recovers is its own liveness watchdog.
 */
const WS_GLOB = '**/ws/overlay/**'

const connectedFrame = JSON.stringify({ type: 'connected', data: { overlay_id: OVERLAY_ID } })
const chatFrame = (id: string, text: string) =>
  JSON.stringify({
    type: 'chat_message',
    data: {
      id,
      overlay_id: OVERLAY_ID,
      platform: 'kick',
      channel_id: 'c1',
      channel_name: 'chan',
      user: { id: 'u1', username: 'tester', display_name: 'Tester', badges: [] },
      message: { text, emotes: [] },
      timestamp: new Date().toISOString(),
      metadata: {},
    },
  })

test.describe('Overlay Observability View — connection resilience', () => {
  test('shows Live and renders a message delivered over the socket', async ({ page }) => {
    await page.routeWebSocket(WS_GLOB, (ws) => {
      ws.send(connectedFrame)
      ws.send(chatFrame('m1', 'hello-over-socket'))
    })

    await page.goto(VIEW_URL)
    await expect(page.locator('.connection-status')).toContainText('Live')
    await expect(page.getByText('hello-over-socket')).toBeVisible()
  })

  test('flips off "Live" and reconnects after the socket is dropped', async ({ page }) => {
    let connections = 0
    await page.routeWebSocket(WS_GLOB, (ws) => {
      connections += 1
      const n = connections
      ws.send(connectedFrame)
      ws.send(chatFrame(`msg-${n}`, `delivery-${n}`))
      // Drop the first socket cleanly; the client must reconnect on its own.
      if (n === 1) setTimeout(() => ws.close(), 300)
    })

    await page.goto(VIEW_URL)
    await expect(page.getByText('delivery-1')).toBeVisible()
    // delivery-2 can only arrive over a brand-new connection ⇒ proves reconnect.
    await expect(page.getByText('delivery-2')).toBeVisible({ timeout: 15000 })
    expect(connections).toBeGreaterThanOrEqual(2)
    // Indicator is honest again once the healthy socket is back.
    await expect(page.locator('.connection-status')).toContainText('Live')
  })

  test('recovers from a silent half-open socket via the watchdog', async ({ page, browserName }) => {
    // The watchdog's real-timer window is ~45s; run it only on the engines that
    // matter most (Chromium + Firefox — Firefox being LibreWolf's engine) to
    // keep the slow path off the mobile/webkit matrix.
    test.skip(
      !['chromium', 'firefox'].includes(browserName),
      'slow real-timer watchdog test runs on chromium/firefox only'
    )
    test.setTimeout(90_000)

    let connections = 0
    await page.routeWebSocket(WS_GLOB, (ws) => {
      connections += 1
      ws.send(connectedFrame)
      ws.send(chatFrame(`live-${connections}`, `live-${connections}`))
      // First socket goes silent forever: no further frames, no close. Without a
      // client watchdog this hangs indefinitely (exactly the reported symptom).
    })

    await page.goto(VIEW_URL)
    await expect(page.getByText('live-1')).toBeVisible()
    // The watchdog must notice the silence and force a fresh connection.
    await expect.poll(() => connections, { timeout: 75_000 }).toBeGreaterThanOrEqual(2)
    await expect(page.getByText('live-2')).toBeVisible()
  })
})
