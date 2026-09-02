import { test, expect, type Page } from '@playwright/test'

/**
 * Overlay monitor as an OBS/Streamlabs browser dock (/overlay/[id]/view?dock=1)
 *
 * A custom browser dock differs from a browser tab in two ways this covers:
 *
 * 1. It has its OWN cookie jar (a separate CEF profile), so the streamer's
 *    dashboard session is not there on first open. The monitor's usual answer
 *    to an anonymous visitor is `router.push('/')`, which in a chromeless
 *    ~320px panel renders the marketing homepage with no back button and no
 *    sign-in affordance. In dock mode it must render a sign-in panel instead
 *    and stay on the monitor URL.
 * 2. It is narrow. Two side-by-side columns are ~150px each there, so the
 *    ResizableSplit is replaced by a Chat | Activity tab switcher.
 *
 * Frontend-only (API mocked, no backend), matching overlay-view.spec.ts. The
 * per-message moderation behind these panels is covered by the component tests.
 */

const OVERLAY_ID = 'test-overlay-id'
const DOCK_URL = `/overlay/${OVERLAY_ID}/view?dock=1`

// A dock is typically pinned around 300-450px wide; 320 is the narrow end.
const DOCK_VIEWPORT = { width: 320, height: 720 }

const USER = {
  id: 'user-1',
  username: 'streamer',
  display_name: 'Streamer',
  auth_provider: 'twitch',
  onboarding_completed_at: '2026-01-01T00:00:00Z',
}

function json(body: unknown) {
  return { status: 200, contentType: 'application/json', body: JSON.stringify(body) }
}

/**
 * Playwright matches routes last-registered-first, so the catch-all is
 * registered first and the specific endpoints below override it. Without it,
 * unmocked calls reach a real backend or hang.
 */
async function mockApi(page: Page, { signedIn }: { signedIn: boolean }) {
  await page.route('**/api/v1/**', (route) =>
    route.fulfill({ status: 404, contentType: 'application/json', body: '{}' })
  )
  await page.route('**/api/v1/auth/me', (route) =>
    signedIn
      ? route.fulfill(json(USER))
      : route.fulfill({ status: 401, contentType: 'application/json', body: '{}' })
  )
}

test.describe('Overlay monitor in dock mode', () => {
  test('an anonymous dock offers sign-in instead of navigating away', async ({ page }) => {
    await page.setViewportSize(DOCK_VIEWPORT)
    await mockApi(page, { signedIn: false })

    await page.goto(DOCK_URL)

    // The panel explains the separate sign-in and offers a way to do it here.
    await expect(page.getByRole('button', { name: /sign in with twitch/i })).toBeVisible({
      timeout: 20_000,
    })
    // Still on the monitor: a dock that navigates home is the dead end this
    // whole mode exists to prevent, and it has no back button to recover with.
    expect(new URL(page.url()).pathname).toBe(`/overlay/${OVERLAY_ID}/view`)
    await expect(page.getByRole('heading', { level: 1 })).toHaveCount(0)
  })

  test('an anonymous monitor without the dock flag still redirects home', async ({ page }) => {
    await mockApi(page, { signedIn: false })

    await page.goto(`/overlay/${OVERLAY_ID}/view`)

    await expect(page).toHaveURL(/\/$/, { timeout: 20_000 })
  })

  test('a signed-in dock renders tabs and no resizable split', async ({ page }) => {
    await page.setViewportSize(DOCK_VIEWPORT)
    await mockApi(page, { signedIn: true })

    await page.goto(DOCK_URL)
    await expect(page.locator('#overlay-view-root')).toBeVisible({ timeout: 20_000 })

    // Chat first, Activity one click away…
    const chatTab = page.getByRole('tab', { name: 'Chat' })
    const activityTab = page.getByRole('tab', { name: 'Activity' })
    await expect(chatTab).toHaveAttribute('aria-selected', 'true')
    await expect(activityTab).toHaveAttribute('aria-selected', 'false')

    // …and no split, so neither panel is squeezed into ~150px.
    await expect(page.getByRole('separator', { name: /resize/i })).toHaveCount(0)

    await activityTab.click()
    await expect(activityTab).toHaveAttribute('aria-selected', 'true')
    // The choice survives a reload, which is the only way OBS reopens a dock.
    const stored = await page.evaluate(
      (id) => localStorage.getItem(`overlay-view-dock-tab-${id}`),
      OVERLAY_ID
    )
    expect(stored).toBe('activity')
  })

  test('a signed-in dock keeps its header to one row and hides the layout picker', async ({
    page,
  }) => {
    await page.setViewportSize(DOCK_VIEWPORT)
    await mockApi(page, { signedIn: true })

    await page.goto(DOCK_URL)
    // Direct child: ChatPanel renders a <header> of its own inside the panel.
    const header = page.locator('#overlay-view-root > header')
    await expect(header).toBeVisible({ timeout: 20_000 })

    // One row: the wide header wraps its ten controls, which at this width
    // leaves the panel with no room for chat.
    const headerHeight = await header.evaluate((el) => el.getBoundingClientRect().height)
    expect(headerHeight).toBeLessThan(60)

    // With no split there is no layout to pick, in the menu or out of it.
    await expect(page.getByRole('radiogroup', { name: 'Panel layout' })).toHaveCount(0)
    await page.getByRole('button', { name: 'Monitor controls' }).click()
    await expect(page.getByRole('radiogroup', { name: 'Panel layout' })).toHaveCount(0)
    // The controls that survive are in that one menu.
    await expect(page.getByRole('button', { name: 'Display settings' })).toBeVisible()
  })
})
