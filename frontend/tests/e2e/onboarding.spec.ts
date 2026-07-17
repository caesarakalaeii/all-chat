/**
 * This file is part of All-Chat.
 * Copyright (C) 2026 caesarakalaeii
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program. If not, see <https://www.gnu.org/licenses/>.
 */

/**
 * First-run setup guide (onboarding) E2E — network-route mocks only (auth is
 * cookie/`/auth/me`-based; the legacy localStorage auth mock is dead).
 */

import { test, expect, type Page } from '@playwright/test'
import { expectNoNewA11yViolations } from './a11y-helpers'

interface MockUser {
  id: string
  username: string
  display_name: string
  auth_provider: string
  is_admin: boolean
  is_premium: boolean
  is_beta_tester: boolean
  created_at: string
  updated_at: string
  onboarding_completed_at: string | null
  impersonating?: boolean
  impersonated_by?: string
}

const FRESH_USER: MockUser = {
  id: '11111111-1111-1111-1111-111111111111',
  username: 'fresh_streamer',
  display_name: 'Fresh Streamer',
  auth_provider: 'twitch',
  is_admin: false,
  is_premium: false,
  is_beta_tester: false,
  created_at: '2026-07-17T00:00:00Z',
  updated_at: '2026-07-17T00:00:00Z',
  onboarding_completed_at: null,
}

const OVERLAY = {
  id: '22222222-2222-2222-2222-222222222222',
  user_id: FRESH_USER.id,
  name: 'My Stream',
  description: '',
  is_active: true,
  is_public_for_viewers: false,
  created_at: '2026-07-17T00:00:00Z',
  updated_at: '2026-07-17T00:00:00Z',
}

const SOURCE = {
  id: 'source-1',
  overlay_id: OVERLAY.id,
  platform: 'tiktok',
  channel_id: 'somebody',
  channel_name: 'somebody',
  is_active: true,
}

const CONFIG = {
  id: 'config-1',
  overlay_id: OVERLAY.id,
  display_settings: {},
  filter_settings: {},
  visual_settings: {},
  enable_7tv: true,
  enable_bttv: false,
  enable_ffz: false,
  theme_id: null as string | null,
  custom_css: '',
}

async function mockApi(
  page: Page,
  {
    user = FRESH_USER,
    overlays = [] as (typeof OVERLAY)[],
    sources = [] as (typeof SOURCE)[],
    themeId = null as string | null,
  } = {}
) {
  const json = (body: unknown) => ({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify(body),
  })
  // Catch-all first — Playwright matches routes last-registered-first.
  await page.route('**/api/v1/**', (route) =>
    route.fulfill({ status: 404, contentType: 'application/json', body: '{}' })
  )
  await page.route('**/api/v1/auth/me', (route) => route.fulfill(json(user)))
  await page.route('**/api/v1/auth/guilds', (route) => route.fulfill(json([])))
  await page.route('**/api/v1/overlays', (route) => route.fulfill(json(overlays)))
  await page.route(`**/api/v1/overlays/${OVERLAY.id}`, (route) => route.fulfill(json(OVERLAY)))
  await page.route(`**/api/v1/overlays/${OVERLAY.id}/sources`, (route) =>
    route.fulfill(json(sources))
  )
  await page.route(`**/api/v1/overlays/${OVERLAY.id}/config`, (route) =>
    route.fulfill(json({ ...CONFIG, theme_id: themeId }))
  )
}

test.describe('first-run setup guide', () => {
  test.beforeEach(async ({ page }) => {
    await page.emulateMedia({ reducedMotion: 'reduce' })
    await page.route('https://cdn.jsdelivr.net/**', (route) => route.abort())
  })

  test('auto-opens for a fresh user on the dashboard', async ({ page }, testInfo) => {
    await mockApi(page)
    await page.goto('/dashboard')
    const guide = page.getByRole('region', { name: 'Setup guide' })
    await expect(guide).toBeVisible({ timeout: 20_000 })
    await expect(guide.getByText('0 of 4 steps done')).toBeVisible()
    await expect(guide.getByRole('button', { name: 'Create' })).toBeVisible()
    // The wizard surface itself must scan clean — it is new code with no
    // baseline entries.
    await expectNoNewA11yViolations(page, 'dashboard-empty', testInfo)
  })

  test('does not open when onboarding is already completed', async ({ page }) => {
    await mockApi(page, {
      user: { ...FRESH_USER, onboarding_completed_at: '2026-01-01T00:00:00Z' },
    })
    await page.goto('/dashboard')
    await expect(page.getByText('No overlays yet')).toBeVisible({ timeout: 20_000 })
    await expect(page.getByRole('region', { name: 'Setup guide' })).toHaveCount(0)
  })

  test('does not open for existing users with overlays (backfill-miss guard)', async ({ page }) => {
    await mockApi(page, { overlays: [OVERLAY], sources: [SOURCE] })
    await page.goto('/dashboard')
    await expect(page.getByText(OVERLAY.name)).toBeVisible({ timeout: 20_000 })
    await expect(page.getByRole('region', { name: 'Setup guide' })).toHaveCount(0)
  })

  test('does not open while impersonating', async ({ page }) => {
    await mockApi(page, {
      user: { ...FRESH_USER, impersonating: true, impersonated_by: 'admin-1' },
    })
    await page.goto('/dashboard')
    await expect(page.getByText('No overlays yet')).toBeVisible({ timeout: 20_000 })
    await expect(page.getByRole('region', { name: 'Setup guide' })).toHaveCount(0)
  })

  test('editor derives step completion and spotlights the active section', async ({
    page,
  }, testInfo) => {
    // Straight to the editor via a HARD navigation — exactly what the OAuth
    // add-source round-trip does. The editor re-arms the guide from the
    // server flag; the in-memory store did not survive and must not need to.
    await mockApi(page, { overlays: [], sources: [SOURCE], themeId: null })
    await page.goto(`/overlays/${OVERLAY.id}`)
    const guide = page.getByRole('region', { name: 'Setup guide' })
    await expect(guide).toBeVisible({ timeout: 20_000 })
    // Overlay exists (editor surface) + source exists → steps 1-2 done,
    // choose_theme active.
    await expect(guide.getByText('2 of 4 steps done')).toBeVisible()
    await expect(guide.getByRole('button', { name: 'Show me' })).toBeVisible()
    // The theme section is force-opened by the spotlight.
    const themeTrigger = page.getByRole('button', { name: 'Theme', exact: true })
    await expect(themeTrigger).toHaveAttribute('aria-expanded', 'true')
    await expectNoNewA11yViolations(page, 'overlay-editor', testInfo)
  })

  test('dismiss confirms, persists, and hides the guide', async ({ page }) => {
    await mockApi(page)
    let patchBody: unknown = null
    await page.route('**/api/v1/auth/me/onboarding', async (route) => {
      patchBody = route.request().postDataJSON()
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ onboarding_completed_at: '2026-07-17T12:00:00Z' }),
      })
    })
    await page.goto('/dashboard')
    const guide = page.getByRole('region', { name: 'Setup guide' })
    await expect(guide).toBeVisible({ timeout: 20_000 })
    await guide.getByRole('button', { name: 'Dismiss setup guide' }).click()
    await page.getByRole('button', { name: 'Hide guide' }).click()
    await expect(page.getByRole('region', { name: 'Setup guide' })).toHaveCount(0)
    await expect.poll(() => patchBody).toEqual({ completed: true })
  })

  test('settings restart re-arms the guide and returns to the dashboard', async ({ page }) => {
    await mockApi(page, {
      user: { ...FRESH_USER, onboarding_completed_at: '2026-01-01T00:00:00Z' },
    })
    let patchBody: unknown = null
    await page.route('**/api/v1/auth/me/onboarding', async (route) => {
      patchBody = route.request().postDataJSON()
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ onboarding_completed_at: null }),
      })
    })
    await page.goto('/settings')
    await expect(page.getByRole('heading', { name: 'Setup guide' })).toBeVisible({
      timeout: 20_000,
    })
    await page.getByRole('button', { name: 'Restart' }).click()
    await expect(page).toHaveURL(/\/dashboard/, { timeout: 20_000 })
    await expect(page.getByRole('region', { name: 'Setup guide' })).toBeVisible()
    await expect.poll(() => patchBody).toEqual({ completed: false })
  })
})
