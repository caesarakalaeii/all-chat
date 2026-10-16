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
 * The list overlay (ADR-0072, #955): the render route seeds the board from
 * GET /overlays/:id/list/top, then re-ranks it on every `type: "list_update"`
 * frame the alert-processor debounces per board. Frames for boards the
 * overlay does not show are ignored.
 */

const OVERLAY_ID = 'list-e2e-overlay'

const listUpdate = (category: string, entries: Array<[string, number]>) =>
  JSON.stringify({
    type: 'list_update',
    data: {
      type: 'list_update',
      overlay_id: OVERLAY_ID,
      category,
      entries: entries.map(([name, amount], i) => ({
        rank: i + 1,
        name,
        platform: 'twitch',
        amount,
      })),
    },
    timestamp: '2026-10-16T12:00:00.000Z',
  })

/**
 * Screenshot artifact. The overlay is white art on a transparent page, so it
 * is shot over a stream-like dark backdrop, with transitions finished.
 */
async function shoot(page: Page, name: string) {
  await page.evaluate(() =>
    document.documentElement.style.setProperty('background', '#1f2430', 'important')
  )
  await page.screenshot({ path: test.info().outputPath(name), animations: 'disabled' })
}

async function boot(page: Page) {
  const sockets: Array<{ send: (s: string) => void }> = []
  await page.routeWebSocket('**/ws/overlay/**', (ws) => {
    sockets.push(ws)
    ws.send(JSON.stringify({ type: 'connected', data: { overlay_id: OVERLAY_ID } }))
  })
  await page.route(`**/api/v1/overlays/public/${OVERLAY_ID}/config`, (route) =>
    route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ overlay_type: 'list', display_settings: {}, sources: [] }),
    })
  )
  await page.route(`**/api/v1/overlays/${OVERLAY_ID}/list/top`, (route) =>
    route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        title: 'Top cheerers',
        category: 'bits',
        limit: 3,
        entries: [
          { rank: 1, name: 'EarlyBird', platform: 'twitch', amount: 500 },
          { rank: 2, name: 'Regular', platform: 'twitch', amount: 200 },
        ],
      }),
    })
  )
  await page.setViewportSize({ width: 600, height: 400 })
  await page.goto(`/overlay/${OVERLAY_ID}`)
  await expect.poll(() => sockets.length, { timeout: 15_000 }).toBeGreaterThan(0)
  return (frame: string) => sockets[sockets.length - 1].send(frame)
}

test.describe('List overlay', () => {
  test('ranks the stored board, then re-ranks on list_update', async ({ page }) => {
    const send = await boot(page)
    const rows = page.getByRole('list', { name: 'Top cheerers' }).getByRole('listitem')
    await expect(rows).toHaveCount(2)
    await expect(rows.first()).toContainText('EarlyBird')

    send(
      listUpdate('bits', [
        ['BigSpender', 5000],
        ['EarlyBird', 500],
        ['Regular', 200],
        ['Fourth', 10],
      ])
    )
    await expect(rows).toHaveCount(3) // cut to the configured limit
    await expect(rows.first()).toContainText('BigSpender')
    await expect(rows.first()).toContainText('5,000')
    await shoot(page, 'list-after-update.png')
  })

  test('ignores updates for a board it does not show', async ({ page }) => {
    const send = await boot(page)
    send(listUpdate('gifts', [['Gifter', 50]]))
    const rows = page.getByRole('list', { name: 'Top cheerers' }).getByRole('listitem')
    await expect(rows.first()).toContainText('EarlyBird')
    await expect(page.getByText('Gifter')).toHaveCount(0)
  })
})

const EDITOR_ID = 'list-editor-e2e'

test.describe('List editor', () => {
  test('saves the chosen board for a list overlay', async ({ page }) => {
    const saved: unknown[] = []
    await page.route('**/api/v1/**', (route) =>
      route.fulfill({ status: 404, contentType: 'application/json', body: '{}' })
    )
    await page.route('**/api/v1/auth/me', (route) =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          id: 'u1',
          username: 'streamer',
          is_premium: true,
          onboarding_completed_at: '2026-01-01T00:00:00Z',
        }),
      })
    )
    await page.route(`**/api/v1/overlays/${EDITOR_ID}`, (route) =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          id: EDITOR_ID,
          user_id: 'u1',
          name: 'Top fans',
          overlay_type: 'list',
        }),
      })
    )
    await page.route(`**/api/v1/overlays/${EDITOR_ID}/sources`, (route) =>
      route.fulfill({ status: 200, contentType: 'application/json', body: '[]' })
    )
    await page.route(`**/api/v1/overlays/${EDITOR_ID}/list`, async (route) => {
      if (route.request().method() === 'PUT') saved.push(route.request().postDataJSON())
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          list: { category: 'bits', limit: 10, title: '' },
          configured: false,
        }),
      })
    })
    await page.goto('/')
    await page.evaluate(() =>
      localStorage.setItem(
        'auth-store',
        JSON.stringify({
          state: { user: { id: 'u1', username: 'streamer' }, token: 'mock-jwt' },
          version: 0,
        })
      )
    )
    await page.goto(`/overlays/${EDITOR_ID}`)

    const nav = page.getByRole('navigation', { name: 'Overlay settings' })
    await expect(nav.getByRole('button', { name: 'Leaderboard' })).toBeVisible({ timeout: 30_000 })
    await expect(page.getByRole('button', { name: 'Monitor View', exact: true })).toHaveCount(0)
    await page.getByLabel('Ranked by').click()
    await page.getByRole('option', { name: 'Subs and memberships gifted' }).click()
    await page.getByLabel(/Ranks shown/).fill('5')
    await page.getByRole('button', { name: 'Save leaderboard' }).click()
    await expect(page.getByRole('region', { name: 'Notifications' })).toContainText(
      'Leaderboard saved'
    )
    expect(saved).toEqual([{ category: 'gifts', limit: 5, title: '' }])
    await page.screenshot({ path: test.info().outputPath('list-editor.png') })
  })
})
