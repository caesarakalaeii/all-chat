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
 * Viewer identities from the web (ADR-0034): an admin reaches a viewer who never
 * signed in and grants premium on the durable viewer, the Activity dialog reaches
 * the backend, and a signed-in viewer is asked to accept the terms.
 */
import { test, expect } from '@playwright/test'

const ADMIN = {
  id: '11111111-1111-1111-1111-111111111111',
  username: 'admin_e2e',
  display_name: 'Admin E2E',
  auth_provider: 'twitch',
  is_admin: true,
  is_premium: true,
  tos_accepted: true,
  onboarding_completed_at: '2026-01-02T00:00:00Z',
}

const SESSIONLESS = {
  session_id: null,
  viewer_id: '22222222-2222-4222-8222-222222222222',
  platform: 'youtube',
  platform_user_id: 'UCL3eFQtbthgTGal-ktJV5Vg',
  username: '',
  display_name: '',
  avatar_url: null,
  last_message_at: null,
  is_premium: false,
  premium_expires_at: null,
  user_id: null,
  is_banned: false,
  banned_at: null,
  banned_reason: null,
  created_at: '2026-11-01T00:00:00Z',
}

const SIGNED_IN = {
  ...SESSIONLESS,
  session_id: '33333333-3333-4333-8333-333333333333',
  viewer_id: '44444444-4444-4444-8444-444444444444',
  platform: 'twitch',
  platform_user_id: '1001',
  username: 'chatter',
  display_name: 'Chatter',
}

const json = (body: unknown, status = 200) => ({
  status,
  contentType: 'application/json',
  body: JSON.stringify(body),
})

test.beforeEach(async ({ page }) => {
  // Catch-all first — Playwright matches routes last-registered-first.
  await page.route('**/api/v1/**', (route) => route.fulfill(json({}, 404)))
  await page.addInitScript(() => localStorage.setItem('cookieBannerAcknowledged', 'true'))
})

test('an admin finds a viewer who never signed in and grants them premium', async ({ page }) => {
  await page.route('**/api/v1/auth/me', (route) => route.fulfill(json(ADMIN)))
  await page.route('**/api/v1/admin/viewers?*', (route) =>
    route.fulfill(json({ viewers: [SESSIONLESS, SIGNED_IN], total: 2, limit: 50, offset: 0 }))
  )
  const posted: string[] = []
  await page.route('**/api/v1/admin/viewers/**', (route) => {
    const url = new URL(route.request().url())
    posted.push(`${route.request().method()} ${url.pathname}`)
    return route.fulfill(json({ total_messages: 0, last_sent_at: null, streamers: [] }))
  })

  await page.goto('/admin/viewers?q=UCL3eFQtbthgTGal-ktJV5Vg')

  const table = page.getByRole('table')
  const row = table.getByRole('row').filter({ hasText: SESSIONLESS.platform_user_id })
  await expect(row).toBeVisible({ timeout: 20_000 })
  await expect(row.getByText('Not signed in')).toBeVisible()

  await row
    .getByRole('button', {
      name: `Free: change premium status for ${SESSIONLESS.platform_user_id}`,
    })
    .click()
  await page.getByRole('button', { name: 'Grant Premium' }).click()
  await expect
    .poll(() => posted)
    .toContain(`POST /api/v1/admin/viewers/identities/${SESSIONLESS.viewer_id}/premium`)

  // The signed-in viewer's Activity dialog asks the backend by session id.
  await table
    .getByRole('row')
    .filter({ hasText: SIGNED_IN.username })
    .getByRole('button', { name: 'Activity' })
    .click()
  await expect
    .poll(() => posted)
    .toContain(`GET /api/v1/admin/viewers/${SIGNED_IN.session_id}/activity`)
})

test('a signed-in viewer accepts the terms before using the site', async ({ page }) => {
  const claims = { viewer_id: 'viewer-1', display_name: 'Viewer', platform: 'twitch', exp: 4e9 }
  const token = `x.${Buffer.from(JSON.stringify(claims)).toString('base64')}.x`
  await page.addInitScript((t) => localStorage.setItem('viewer_jwt_token', t), token)

  let accepted = false
  await page.route('**/api/v1/auth/viewer/me', (route) =>
    route.fulfill(
      json({ id: 's-1', platform: 'twitch', username: 'viewer', tos_accepted: accepted })
    )
  )
  let authorization: string | undefined
  await page.route('**/api/v1/auth/viewer/me/tos-acceptance', (route) => {
    authorization = route.request().headers()['authorization']
    accepted = true
    return route.fulfill(json({ tos_accepted: true }))
  })

  await page.goto('/settings/viewer')

  const dialog = page.getByRole('alertdialog')
  await expect(dialog).toBeVisible({ timeout: 20_000 })
  await dialog.getByRole('checkbox').check()
  await dialog.getByRole('button', { name: 'Accept' }).click()

  await expect(dialog).toBeHidden()
  expect(authorization).toBe(`Bearer ${token}`)
  await page.reload()
  await expect(page.getByRole('heading', { name: 'Viewer Identity' }).first()).toBeVisible()
  await expect(page.getByRole('alertdialog')).toBeHidden()
})
