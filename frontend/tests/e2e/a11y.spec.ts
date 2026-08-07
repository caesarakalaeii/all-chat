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
 * A11y smoke suite — axe-core over the main app pages (WCAG 2.2 AA gate).
 *
 * Auth is cookie/`/auth/me`-based, so authenticated pages are driven with
 * NETWORK-ROUTE mocks (page.route on /api/v1/*), not the legacy
 * localStorage['auth-store'] mock used by older specs — that store is no
 * longer read for auth decisions.
 *
 * Each test asserts a page-specific anchor is visible BEFORE scanning, so a
 * broken mock fails loudly instead of silently scanning an error page.
 *
 * Coverage gaps (deliberate, not silent): /dashboard/shares, /admin/*, and
 * /overlay/[id]/participate need heavier fixtures (share tokens, admin stats,
 * viewer session + websocket) and join the suite with the admin pass
 * (roadmap Batch 8) and the cross-cutting sweep (Batch 9). Modal-open states
 * follow with the modal consolidation batch.
 */

import { test, expect, type Page } from '@playwright/test'
import { expectNoNewA11yViolations } from './a11y-helpers'

const USER = {
  id: '11111111-1111-1111-1111-111111111111',
  username: 'a11y_smoke',
  display_name: 'A11y Smoke',
  auth_provider: 'twitch',
  is_admin: false,
  is_premium: false,
  is_beta_tester: false,
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
  // Completed, so the (upcoming) first-run setup guide never overlays the
  // pages this suite scans.
  onboarding_completed_at: '2026-01-02T00:00:00Z',
}

const OVERLAY = {
  id: '22222222-2222-2222-2222-222222222222',
  user_id: USER.id,
  name: 'A11y Test Overlay',
  description: '',
  is_active: true,
  is_public_for_viewers: false,
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
}

const SOURCES = [
  {
    id: 'source-1',
    overlay_id: OVERLAY.id,
    platform: 'twitch',
    channel_id: '123',
    channel_name: 'somechannel',
    is_active: true,
  },
]

const CONFIG = {
  id: 'config-1',
  overlay_id: OVERLAY.id,
  display_settings: {},
  filter_settings: {},
  visual_settings: {},
  enable_7tv: true,
  enable_bttv: false,
  enable_ffz: false,
  theme_id: null,
  custom_css: '',
}

/**
 * Register mocks for an authenticated session. Registration order matters:
 * Playwright matches routes LAST-registered-first, so the JSON-404 catch-all
 * goes first and specific endpoints override it. The catch-all keeps unknown
 * API calls from hanging the page or hitting a real backend.
 */
async function mockAuthedApi(page: Page, overrides: Record<string, unknown> = {}) {
  await page.route('**/api/v1/**', (route) =>
    route.fulfill({ status: 404, contentType: 'application/json', body: '{}' })
  )
  const json = (body: unknown) => ({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify(body),
  })
  await page.route('**/api/v1/auth/me', (route) => route.fulfill(json(USER)))
  await page.route('**/api/v1/auth/guilds', (route) => route.fulfill(json([])))
  await page.route('**/api/v1/payment/status', (route) =>
    route.fulfill(json({ connected: false, is_premium: false }))
  )
  for (const [pattern, body] of Object.entries(overrides)) {
    await page.route(pattern, (route) => route.fulfill(json(body)))
  }
}

test.describe('a11y smoke (axe, WCAG 2.2 AA)', () => {
  test.beforeEach(async ({ page }) => {
    // Must be set BEFORE navigation: the landing theme carousel reads
    // prefers-reduced-motion at mount (ThemeSwitcher) and otherwise keeps
    // auto-advancing via JS, changing the DOM between scans. CSS animations
    // are additionally frozen inside the scan helper.
    await page.emulateMedia({ reducedMotion: 'reduce' })
    // Monaco (editor CSS panel) loads from cdn.jsdelivr.net, which the dev
    // CSP rejects — the async failure surfaces at random points and flakes
    // the scan. Block it outright; the Monaco surface is audited separately
    // (roadmap Batch 6).
    await page.route('https://cdn.jsdelivr.net/**', (route) => route.abort())
  })

  test('landing page', async ({ page }, testInfo) => {
    await page.goto('/')
    await expect(page.getByRole('main')).toBeVisible({ timeout: 20_000 })
    await expectNoNewA11yViolations(page, 'landing', testInfo)
  })

  test('docs page', async ({ page }, testInfo) => {
    await page.goto('/docs')
    // No <main> landmark on this page yet (itself a Batch 7 finding) — anchor
    // on the h1 instead.
    await expect(page.getByRole('heading', { name: 'Streamer guide' })).toBeVisible({
      timeout: 20_000,
    })
    await expectNoNewA11yViolations(page, 'docs', testInfo)
  })

  test('upgrade page', async ({ page }, testInfo) => {
    await mockAuthedApi(page)
    await page.goto('/upgrade')
    await expect(page.getByRole('main')).toBeVisible({ timeout: 20_000 })
    await expectNoNewA11yViolations(page, 'upgrade', testInfo)
  })

  test('dashboard with overlays', async ({ page }, testInfo) => {
    await mockAuthedApi(page, {
      '**/api/v1/overlays': [OVERLAY],
      [`**/api/v1/overlays/${OVERLAY.id}/sources`]: SOURCES,
    })
    await page.goto('/dashboard')
    await expect(page.getByText(OVERLAY.name)).toBeVisible({ timeout: 20_000 })
    await expectNoNewA11yViolations(page, 'dashboard-populated', testInfo)
  })

  test('dashboard empty state', async ({ page }, testInfo) => {
    await mockAuthedApi(page, { '**/api/v1/overlays': [] })
    await page.goto('/dashboard')
    await expect(page.getByText('No overlays yet')).toBeVisible({ timeout: 20_000 })
    await expectNoNewA11yViolations(page, 'dashboard-empty', testInfo)
  })

  test('settings page', async ({ page }, testInfo) => {
    await mockAuthedApi(page)
    await page.goto('/settings')
    await expect(page.getByText(USER.username)).toBeVisible({ timeout: 20_000 })
    await expectNoNewA11yViolations(page, 'settings', testInfo)
  })

  test('overlay editor', async ({ page }, testInfo) => {
    await mockAuthedApi(page, {
      [`**/api/v1/overlays/${OVERLAY.id}`]: OVERLAY,
      [`**/api/v1/overlays/${OVERLAY.id}/sources`]: SOURCES,
      [`**/api/v1/overlays/${OVERLAY.id}/config`]: CONFIG,
    })
    await page.goto(`/overlays/${OVERLAY.id}`)
    await expect(page.getByText(OVERLAY.name)).toBeVisible({ timeout: 20_000 })
    await expectNoNewA11yViolations(page, 'overlay-editor', testInfo)
  })

  // The moderator's side of delegation (ADR-0048). Worth its own scan rather than
  // trusting the owner pages: it is the only surface a volunteer ever sees, and they
  // reach it from a link rather than by exploring the app.
  test('channels you moderate', async ({ page }, testInfo) => {
    await mockAuthedApi(page, {
      '**/api/v1/moderation/delegations': {
        delegations: [
          {
            grant_id: 'grant-1',
            overlay_id: OVERLAY.id,
            overlay_name: OVERLAY.name,
            owner_display_name: 'Some Streamer',
            status: 'active',
            actions: ['delete', 'timeout'],
            platforms: [{ platform: 'twitch', enabled: true, verification: 'unverified' }],
            available: true,
          },
        ],
      },
    })
    await page.goto('/moderate')
    await expect(page.getByRole('heading', { name: 'Channels you moderate' })).toBeVisible({
      timeout: 20_000,
    })
    await expectNoNewA11yViolations(page, 'moderate-delegations', testInfo)
  })

  test('moderation invite acceptance', async ({ page }, testInfo) => {
    await mockAuthedApi(page, {
      '**/api/v1/moderation/invites/preview': {
        overlay_name: OVERLAY.name,
        owner_display_name: 'Some Streamer',
        actions: ['delete', 'timeout'],
        platforms: [{ platform: 'twitch', enabled: true, verification: 'unverified' }],
        expires_at: '2026-08-14T10:00:00Z',
      },
    })
    await page.goto('/moderate/accept?token=a11y-smoke-token')
    await expect(page.getByRole('heading', { name: 'Moderation invite' })).toBeVisible({
      timeout: 20_000,
    })
    await expectNoNewA11yViolations(page, 'moderate-accept', testInfo)
  })
})
