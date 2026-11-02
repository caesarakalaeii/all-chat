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

import { test, expect, type Page } from '@playwright/test'

/**
 * The paged admin lists (ADR-0086): users, overlays and sources load one page
 * at a time and fetch the next as the list scrolls; search and filters are
 * sent to the server instead of filtering a full list in the browser. Runs
 * against a mock of the admin list endpoints that pages like the real ones.
 */

const ADMIN = {
  id: '11111111-1111-1111-1111-111111111111',
  username: 'admin_e2e',
  display_name: 'Admin E2E',
  auth_provider: 'twitch',
  is_admin: true,
  is_premium: true,
  onboarding_completed_at: '2026-01-02T00:00:00Z',
}

const TOTAL = 120

const json = (body: unknown, status = 200) => ({
  status,
  contentType: 'application/json',
  body: JSON.stringify(body),
})

const uuid = (i: number) => `00000000-0000-4000-8000-${String(i).padStart(12, '0')}`

const USERS = Array.from({ length: TOTAL }, (_, i) => ({
  id: uuid(i),
  username: `streamer_${i}`,
  display_name: `Streamer ${i}`,
  auth_provider: 'twitch',
  profile_image_url: '',
  created_at: '2026-10-01T00:00:00Z',
  twitch_id: String(1000 + i),
  is_premium: i % 10 === 0,
  is_beta_tester: false,
  is_ambassador: false,
  is_banned: false,
  overlays_count: 1,
  sources_count: 1,
}))

const OVERLAYS = USERS.map((u, i) => ({
  id: uuid(1000 + i),
  name: `Overlay ${i}`,
  overlay_type: 'chat',
  user_id: u.id,
  owner_username: u.username,
  owner_display_name: u.display_name,
  created_at: '2026-10-01T00:00:00Z',
  updated_at: '2026-10-01T00:00:00Z',
  sources_count: 1,
}))

const SOURCES = OVERLAYS.map((o, i) => ({
  id: uuid(2000 + i),
  overlay_id: o.id,
  overlay_name: o.name,
  platform: i % 2 === 0 ? 'twitch' : 'youtube',
  channel_id: `chan-${i}`,
  channel_name: `Channel ${i}`,
  is_active: true,
  created_at: '2026-10-01T00:00:00Z',
  user_id: o.user_id,
  owner_username: o.owner_username,
}))

/** Serves `rows` under `key` like the admin list endpoints: q matching, limit/offset paging and a total. */
function paged<T>(key: string, rows: T[], match: (row: T, url: URL) => boolean) {
  return (url: URL) => {
    const hits = rows.filter((r) => match(r, url))
    const limit = Number(url.searchParams.get('limit') || 50)
    const offset = Number(url.searchParams.get('offset') || 0)
    return { [key]: hits.slice(offset, offset + limit), total: hits.length }
  }
}

const q = (url: URL) => url.searchParams.get('q')?.toLowerCase() ?? ''

async function boot(page: Page) {
  const requests: URL[] = []
  const serve = (pattern: string, answer: (url: URL) => unknown) =>
    page.route(pattern, (route) => {
      const url = new URL(route.request().url())
      requests.push(url)
      return route.fulfill(json(answer(url)))
    })

  await page.route('**/api/v1/**', (route) => route.fulfill(json({}, 404)))
  await page.route('**/api/v1/auth/me', (route) => route.fulfill(json(ADMIN)))
  await serve('**/api/v1/admin/users?*', (url) => {
    const hits = USERS.filter((u) => u.username.includes(q(url)) || u.id.includes(q(url)))
    const premium = hits.filter((u) => u.is_premium)
    const tab = url.searchParams.get('filter') === 'premium' ? premium : hits
    // Tab counts follow the search, not the selected tab, like the real endpoint.
    const counts = {
      all: hits.length,
      active: hits.length,
      banned: 0,
      premium: premium.length,
      beta: 0,
      unconfigured: 0,
    }
    return { ...paged('users', tab, () => true)(url), counts }
  })
  await serve(
    '**/api/v1/admin/overlays?*',
    paged('overlays', OVERLAYS, (o, url) => o.name.toLowerCase().includes(q(url)))
  )
  await serve('**/api/v1/admin/overlays/active', () => [])
  await serve(
    '**/api/v1/admin/sources?*',
    paged('sources', SOURCES, (s, url) => {
      const platform = url.searchParams.get('platform')
      return (!platform || s.platform === platform) && s.channel_name.toLowerCase().includes(q(url))
    })
  )
  await serve('**/api/v1/admin/stats', () => ({ total_sources: { twitch: 60, youtube: 60 } }))
  await serve('**/api/v1/admin/viewers?*', () => ({ viewers: [], total: 0 }))

  await page.addInitScript(() => localStorage.setItem('cookieBannerAcknowledged', 'true'))
  return requests
}

/** Scrolls the last row of `list` into view, which pulls in the next page. */
async function scrollToEnd(list: ReturnType<Page['locator']>) {
  await list.getByRole('button', { name: 'Load more' }).scrollIntoViewIfNeeded()
}

test('users load a page at a time and search on the server', async ({ page }) => {
  const requests = await boot(page)
  await page.goto('/admin/users')

  const user = /^Streamer \d+/
  const list = page.getByRole('list').filter({ has: page.getByRole('button', { name: user }) })
  const rows = list.getByRole('button', { name: user })
  await expect(rows).toHaveCount(50, { timeout: 20_000 })
  await expect(page.getByRole('heading', { name: 'All Users (120)' })).toBeVisible()
  await expect(page.getByRole('tab', { name: 'Premium (12)' })).toBeVisible()

  await scrollToEnd(list)
  await expect(rows).toHaveCount(100)
  await scrollToEnd(list)
  await expect(rows).toHaveCount(TOTAL)
  await expect(list.getByRole('button', { name: 'Load more' })).toHaveCount(0)

  await page.getByPlaceholder(/search/i).fill('streamer_7')
  await expect(rows).toHaveCount(11) // streamer_7 and streamer_70..79
  await expect(page.getByRole('tab', { name: 'Premium (1)' })).toBeVisible()
  expect(
    requests.some(
      (u) => u.pathname.endsWith('/admin/users') && u.searchParams.get('q') === 'streamer_7'
    )
  ).toBe(true)

  await page.getByRole('tab', { name: /^Premium/ }).click()
  await expect(rows).toHaveCount(1) // streamer_70
  expect(requests.some((u) => u.searchParams.get('filter') === 'premium')).toBe(true)
})

test('a ?user= deep link selects that user', async ({ page }) => {
  await boot(page)
  await page.goto(`/admin/users?user=${uuid(42)}`)
  await expect(page.getByRole('heading', { name: 'Streamer 42', level: 3 })).toBeVisible({
    timeout: 20_000,
  })
  await expect(page.getByPlaceholder(/search/i)).toHaveValue('streamer_42')
})

test('overlays load a page at a time', async ({ page }) => {
  await boot(page)
  await page.goto('/admin/overlays')

  const list = page.getByRole('list').filter({ has: page.getByText('Overlay 0', { exact: true }) })
  const rows = list.getByRole('button', { name: /^Overlay \d+/ })
  await expect(rows).toHaveCount(50, { timeout: 20_000 })
  await expect(page.getByRole('heading', { name: 'All Overlays (50 of 120)' })).toBeVisible()

  await scrollToEnd(list)
  await expect(rows).toHaveCount(100)
})

test('sources page through a platform deep link, with totals from the stats', async ({ page }) => {
  const requests = await boot(page)
  await page.goto('/admin/sources?platform=twitch')

  const table = page.getByRole('table')
  const rows = table.getByRole('row').filter({ has: page.getByRole('cell') })
  await expect(rows).toHaveCount(50, { timeout: 20_000 })
  expect(
    requests.every(
      (u) => !u.pathname.endsWith('/admin/sources') || u.searchParams.get('platform') === 'twitch'
    )
  ).toBe(true)

  await page.getByRole('button', { name: 'Load more' }).scrollIntoViewIfNeeded()
  await expect(rows).toHaveCount(60)
  await expect(page.getByRole('button', { name: 'Load more' })).toHaveCount(0)
})

test('global search asks each list for a few server-side matches', async ({ page }) => {
  const requests = await boot(page)
  await page.goto('/admin/search?q=streamer_7')

  await expect(page.getByRole('link', { name: /Streamer 7\b/ }).first()).toBeVisible({
    timeout: 20_000,
  })
  const userSearch = requests.find((u) => u.pathname.endsWith('/admin/users'))
  expect(userSearch?.searchParams.get('q')).toBe('streamer_7')
  expect(userSearch?.searchParams.get('limit')).toBe('8')
})
