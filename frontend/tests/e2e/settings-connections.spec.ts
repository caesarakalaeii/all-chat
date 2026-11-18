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
 * Settings → Connected platforms, in the real page: the card lists what
 * GET /auth/me/connections returns, the sign-in platform has no disconnect,
 * and a disconnect is sent only after the confirmation and only for the chosen
 * platform. The API is mocked (same contract as a11y.spec.ts); the backend side
 * is covered against real PostgreSQL in auth-service's repository tests.
 */

import { test, expect, type Page } from '@playwright/test'

const USER = {
  id: 'user-1',
  username: 'tester',
  display_name: 'Tester',
  auth_provider: 'twitch',
  is_admin: false,
  is_premium: false,
  is_beta_tester: false,
  is_ambassador: false,
  onboarding_completed_at: '2026-01-01T00:00:00Z',
}

const CONNECTIONS = [
  { platform: 'twitch', accounts: ['tester'], is_login: true },
  { platform: 'youtube', accounts: ['UCtester'], is_login: false },
  { platform: 'kick', accounts: ['tester-kick'], is_login: false },
]

function json(body: unknown) {
  return { status: 200, contentType: 'application/json', body: JSON.stringify(body) }
}

async function mockApi(page: Page, deleted: string[]) {
  // Catch-all first: Playwright matches routes last-registered-first.
  await page.route('**/api/v1/**', (route) =>
    route.fulfill({ status: 404, contentType: 'application/json', body: '{}' })
  )
  await page.route('**/api/v1/auth/me', (route) => route.fulfill(json(USER)))
  await page.route('**/api/v1/auth/me/connections', (route) =>
    route.fulfill(json({ connections: CONNECTIONS }))
  )
  await page.route('**/api/v1/auth/me/connections/*', (route) => {
    if (route.request().method() !== 'DELETE') return route.fallback()
    deleted.push(new URL(route.request().url()).pathname)
    return route.fulfill(json({ message: 'Platform disconnected' }))
  })
}

test.describe('settings connected platforms', () => {
  test('disconnects a linked platform after confirmation', async ({ page }) => {
    const deleted: string[] = []
    await page.emulateMedia({ reducedMotion: 'reduce' })
    await mockApi(page, deleted)
    await page.goto('/settings')

    const card = page.locator('#connections')
    await expect(card.getByText('UCtester')).toBeVisible({ timeout: 20_000 })
    await expect(card.getByText(/You sign in with Twitch/)).toBeVisible()
    await expect(card.getByRole('button', { name: 'Disconnect Twitch' })).toHaveCount(0)

    await card.getByRole('button', { name: 'Disconnect YouTube' }).click()
    const dialog = page.getByRole('alertdialog')
    await expect(dialog.getByText('Disconnect YouTube?')).toBeVisible()

    // Cancelling sends nothing.
    await dialog.getByRole('button', { name: 'Cancel' }).click()
    await expect(dialog).toHaveCount(0)
    expect(deleted).toEqual([])

    await card.getByRole('button', { name: 'Disconnect YouTube' }).click()
    await page.getByRole('alertdialog').getByRole('button', { name: 'Yes, disconnect' }).click()

    await expect(card.getByText('UCtester')).toHaveCount(0)
    await expect(card.getByText('tester-kick')).toBeVisible()
    expect(deleted).toEqual(['/api/v1/auth/me/connections/youtube'])
  })
})
