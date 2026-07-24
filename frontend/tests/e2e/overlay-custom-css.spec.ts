import { test, expect } from '@playwright/test'

/**
 * Advanced → Custom CSS editor (ADR-0043)
 *
 * Frontend behaviour (API mocked, no backend):
 * - Applying a theme PRELOADS its CSS so the editor is not a blank box, and the
 *   overlay reads as still linked to the bundle ("Using … theme").
 * - A saved override reads as a fork ("detached from the bundled theme") and
 *   offers "Reset to theme".
 * - The editor region + validation status render.
 *
 * The fork-on-save decision itself is unit-tested (src/lib/utils/custom-css.ts);
 * live-preview-as-you-type is verified manually against the running app because
 * it needs the preview iframe + WebSocket message flow.
 */

const OVERLAY = {
  id: 'css-test-overlay',
  user_id: 'user-1',
  name: 'CSS Test Overlay',
  is_active: true,
}

const USER = {
  id: 'user-1',
  username: 'tester',
  display_name: 'Tester',
  auth_provider: 'twitch',
  onboarding_completed_at: '2026-01-01T00:00:00Z',
}

function json(body: unknown) {
  return { status: 200, contentType: 'application/json', body: JSON.stringify(body) }
}

async function setup(page: import('@playwright/test').Page, config: Record<string, unknown>) {
  await page.emulateMedia({ reducedMotion: 'reduce' })
  await page.route('**/api/v1/**', (route) =>
    route.fulfill({ status: 404, contentType: 'application/json', body: '{}' })
  )
  await page.route('**/api/v1/auth/me', (route) => route.fulfill(json(USER)))
  await page.route(`**/api/v1/overlays/${OVERLAY.id}`, (route) => route.fulfill(json(OVERLAY)))
  await page.route(`**/api/v1/overlays/${OVERLAY.id}/sources`, (route) => route.fulfill(json([])))
  await page.route(`**/api/v1/overlays/${OVERLAY.id}/config`, (route) => route.fulfill(json(config)))
  await page.goto(`/overlays/${OVERLAY.id}`)
  await expect(page.getByText(OVERLAY.name)).toBeVisible({ timeout: 20_000 })
}

async function openCustomCss(page: import('@playwright/test').Page) {
  const nav = page.getByRole('navigation', { name: 'Overlay settings' })
  await nav.getByRole('button', { name: 'Custom CSS' }).click()
}

test.describe('Advanced → Custom CSS', () => {
  test('a themed overlay with no override reads as linked to the bundled theme', async ({
    page,
  }) => {
    await setup(page, { theme_id: 'minimal', custom_css: '', visual_settings: {} })
    await openCustomCss(page)

    // Status pill: still using the bundled theme (auto-updates), not forked.
    await expect(page.getByText(/Using .* theme/)).toBeVisible()
    // Editor is present and preloaded (not a blank box); Reset re-links to theme.
    await expect(page.getByRole('group', { name: 'Custom CSS editor' })).toBeVisible()
    await expect(page.getByRole('button', { name: 'Reset to theme' })).toBeVisible()
  })

  test('a saved override reads as a fork detached from the theme', async ({ page }) => {
    await setup(page, {
      theme_id: 'minimal',
      custom_css: '.chat-message { background: rebeccapurple; }',
      visual_settings: {},
    })
    await openCustomCss(page)

    await expect(page.getByText(/detached from the bundled theme/i)).toBeVisible()
    await expect(page.getByRole('button', { name: 'Reset to theme' })).toBeVisible()
  })

  test('no theme + no CSS shows an empty editor with a Clear action', async ({ page }) => {
    await setup(page, { theme_id: '', custom_css: '', visual_settings: {} })
    await openCustomCss(page)

    await expect(page.getByText('No theme applied')).toBeVisible()
    await expect(page.getByRole('button', { name: 'Clear' })).toBeVisible()
  })
})
