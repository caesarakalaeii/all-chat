import { test, expect } from '@playwright/test'

/**
 * Overlay editor settings navigation (ADR-0042)
 *
 * The left nav replaced the stacked drawers: every section is a flat nav
 * entry, exactly one section renders at a time, settings search jumps to
 * anchored controls, and low-traffic controls sit behind a per-section
 * Advanced disclosure. These tests cover the exact failure modes from the
 * usability feedback that motivated the redesign — above all "I could not
 * find where to disable the badge".
 */

const OVERLAY = {
  id: 'nav-test-overlay',
  user_id: 'user-1',
  name: 'Nav Test Overlay',
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
  return {
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify(body),
  }
}

test.describe('editor settings navigation', () => {
  test.beforeEach(async ({ page }) => {
    await page.emulateMedia({ reducedMotion: 'reduce' })
    // Catch-all first — Playwright matches routes last-registered-first.
    await page.route('**/api/v1/**', (route) =>
      route.fulfill({ status: 404, contentType: 'application/json', body: '{}' })
    )
    await page.route('**/api/v1/auth/me', (route) => route.fulfill(json(USER)))
    await page.route(`**/api/v1/overlays/${OVERLAY.id}`, (route) => route.fulfill(json(OVERLAY)))
    await page.route(`**/api/v1/overlays/${OVERLAY.id}/sources`, (route) =>
      route.fulfill(json([]))
    )
    await page.route(`**/api/v1/overlays/${OVERLAY.id}/config`, (route) =>
      route.fulfill(json({}))
    )
    await page.goto(`/overlays/${OVERLAY.id}`)
    await expect(page.getByText(OVERLAY.name)).toBeVisible({ timeout: 20_000 })
  })

  test('defaults to Theme and switches sections one at a time', async ({ page }) => {
    const nav = page.getByRole('navigation', { name: 'Overlay settings' })
    await expect(page.getByRole('heading', { name: 'Theme', exact: true })).toBeVisible()

    await nav.getByRole('button', { name: 'Visibility' }).click()
    await expect(page.getByRole('heading', { name: 'Visibility' })).toBeVisible()
    await expect(nav.getByRole('button', { name: 'Visibility' })).toHaveAttribute(
      'aria-current',
      'true'
    )
    // One section at a time: the Theme heading is gone
    await expect(page.getByRole('heading', { name: 'Theme', exact: true })).toHaveCount(0)
  })

  test('search "badge" finds and jumps to the badge toggle (the original complaint)', async ({
    page,
  }) => {
    const search = page.getByRole('combobox', { name: 'Search settings' })
    await search.fill('badge')

    const listbox = page.getByRole('listbox', { name: 'Matching settings' })
    await expect(listbox).toBeVisible()
    // Both badge toggles are offered, with their breadcrumb
    await expect(listbox.getByRole('option', { name: /^Show badges/ })).toBeVisible()
    await expect(listbox.getByRole('option', { name: /^Show platform badge/ })).toBeVisible()

    await listbox.getByRole('option', { name: /^Show badges/ }).click()
    await expect(page.getByRole('heading', { name: 'Visibility' })).toBeVisible()
    await expect(page.locator('[data-setting-anchor="showBadges"]')).toBeVisible()
    // Search resets after the jump
    await expect(search).toHaveValue('')
  })

  test('keyboard: Enter selects the top result', async ({ page }) => {
    const search = page.getByRole('combobox', { name: 'Search settings' })
    await search.fill('fade')
    await search.press('Enter')
    await expect(page.getByRole('heading', { name: 'Messages' })).toBeVisible()
    await expect(page.locator('[data-setting-anchor="disableFade"]')).toBeVisible()
  })

  test('advanced controls are collapsed by default; search opens the disclosure', async ({
    page,
  }) => {
    const nav = page.getByRole('navigation', { name: 'Overlay settings' })
    await nav.getByRole('button', { name: 'Messages' }).click()
    await expect(page.getByText('Advanced (1)')).toBeVisible()
    await expect(page.getByText('7TV Emote Set', { exact: true })).toBeHidden()

    const search = page.getByRole('combobox', { name: 'Search settings' })
    await search.fill('7tv emote set')
    await page.getByRole('option', { name: /^7TV Emote Set/ }).click()
    await expect(page.getByText('7TV Emote Set', { exact: true })).toBeVisible()
  })

  test('preview backdrop picker recolors the pane and persists', async ({ page }) => {
    const panel = page.getByTestId('preview-panel')
    await page.getByRole('button', { name: 'Preview on chroma green' }).click()
    await expect(panel).toHaveCSS('background-color', 'rgb(0, 177, 64)')

    // The embed iframe must be transparent so the panel backdrop shows THROUGH
    // it — otherwise the pane stays black regardless of the picker. The root
    // element (html) paints the iframe canvas, so BOTH html and body must be
    // transparent; asserting body alone gave a false pass while html stayed
    // opaque (the actual regression).
    const frame = page.frameLocator('iframe[title="Overlay live preview"]')
    await expect(frame.locator('html')).toHaveCSS('background-color', 'rgba(0, 0, 0, 0)', {
      timeout: 20_000,
    })
    await expect(frame.locator('body')).toHaveCSS('background-color', 'rgba(0, 0, 0, 0)')

    await page.reload()
    await expect(page.getByTestId('preview-panel')).toHaveCSS(
      'background-color',
      'rgb(0, 177, 64)',
      { timeout: 20_000 }
    )
    // Reset restores the default app background
    await page.getByRole('button', { name: 'Preview on app background' }).click()
    await expect(page.getByTestId('preview-panel')).not.toHaveCSS(
      'background-color',
      'rgb(0, 177, 64)'
    )
  })

  test('text shadow preset reaches the live preview', async ({ page }) => {
    // Wait for the embed to render before changing settings: its message
    // listener registers on mount, and the editor re-syncs on EMBED_READY —
    // but a cold iframe load can otherwise outlast the assertion window.
    const previewBody = page
      .frameLocator('iframe[title="Overlay live preview"]')
      .locator('.overlay-preview-body')
    await expect(previewBody).toBeVisible({ timeout: 20_000 })

    // Findable by what it is FOR, not just its name
    const search = page.getByRole('combobox', { name: 'Search settings' })
    await search.fill('readability')
    await page.getByRole('option', { name: /^Text Shadow/ }).click()
    await expect(page.getByRole('heading', { name: 'Typography' })).toBeVisible()

    await page.getByLabel('Text Shadow').selectOption({ label: 'Soft shadow' })
    // The var travels via VISUAL_CSS_UPDATE into the embed and inherits from
    // the preview body container.
    await expect(previewBody).toHaveCSS('text-shadow', 'rgba(0, 0, 0, 0.6) 0px 1px 2px', {
      timeout: 15_000,
    })
  })

  test('last active section persists across reloads', async ({ page }) => {
    const nav = page.getByRole('navigation', { name: 'Overlay settings' })
    await nav.getByRole('button', { name: 'Sounds' }).click()
    await expect(page.getByRole('heading', { name: 'Notification Sounds' })).toBeVisible()

    await page.reload()
    await expect(page.getByRole('heading', { name: 'Notification Sounds' })).toBeVisible({
      timeout: 20_000,
    })
  })
})
