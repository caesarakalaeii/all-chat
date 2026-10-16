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
 * The goal overlay (ADR-0072, #955): the render route seeds the bar from the
 * public goal read, then moves it on every `type: "goal"` frame — a counted
 * event (bits) and a poll result (followers) look the same on the wire. The
 * gateway, the public config and the goal read are mocked; the frames are
 * exactly what the api-gateway forwards from overlay:{id}:alerts.
 */

const OVERLAY_ID = 'goal-e2e-overlay'

const goalFrame = (goal: string, current: number, target: number, title: string) =>
  JSON.stringify({
    type: 'goal',
    data: { type: 'goal', overlay_id: OVERLAY_ID, goal, current, target, title },
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
      body: JSON.stringify({ overlay_type: 'goal', display_settings: {}, sources: [] }),
    })
  )
  await page.route(`**/api/v1/overlays/public/${OVERLAY_ID}/goal`, (route) =>
    route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        goal: {
          metric: 'bits',
          platform: 'twitch',
          target: 1000,
          starting_value: 0,
          title: 'Bits goal',
        },
        configured: true,
        current: 100,
        status: '',
      }),
    })
  )
  await page.setViewportSize({ width: 800, height: 200 })
  await page.goto(`/overlay/${OVERLAY_ID}`)
  await expect.poll(() => sockets.length, { timeout: 15_000 }).toBeGreaterThan(0)
  return (frame: string) => sockets[sockets.length - 1].send(frame)
}

test.describe('Goal overlay', () => {
  test('the bar starts from the stored value and fills on a counted event', async ({ page }) => {
    const send = await boot(page)
    const bar = page.getByRole('progressbar', { name: 'Bits goal' })
    await expect(bar).toHaveAttribute('aria-valuenow', '100')
    // OBS-transparent: the page paints no background of its own.
    expect(await page.evaluate(() => getComputedStyle(document.body).backgroundColor)).toBe(
      'rgba(0, 0, 0, 0)'
    )

    send(goalFrame('bits', 600, 1000, 'Bits goal'))
    await expect(bar).toHaveAttribute('aria-valuenow', '600')
    await expect(page.getByText('600 / 1,000')).toBeVisible()
    await shoot(page, 'goal-after-event.png')
  })

  test('a poll result for a new goal switches the bar', async ({ page }) => {
    const send = await boot(page)
    send(goalFrame('followers', 321, 500, 'Road to 500'))
    const bar = page.getByRole('progressbar', { name: 'Road to 500' })
    await expect(bar).toHaveAttribute('aria-valuenow', '321')
    await expect(bar).toHaveAttribute('aria-valuemax', '500')
    await shoot(page, 'goal-after-poll.png')
  })
})

const EDITOR_ID = 'goal-editor-e2e'

/** The dashboard editor of a goal overlay, API and gateway mocked. */
async function bootEditor(page: Page) {
  const saved: Array<Record<string, unknown>> = []
  let goal = { metric: 'followers', platform: 'twitch', target: 100, starting_value: 0, title: '' }
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
        display_name: 'Streamer',
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
        name: 'Goal bar',
        overlay_type: 'goal',
        is_active: true,
      }),
    })
  )
  await page.route(`**/api/v1/overlays/${EDITOR_ID}/sources`, (route) =>
    route.fulfill({ status: 200, contentType: 'application/json', body: '[]' })
  )
  await page.route(`**/api/v1/overlays/public/${EDITOR_ID}/config`, (route) =>
    route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ overlay_type: 'goal', display_settings: {}, sources: [] }),
    })
  )
  const state = () => ({ goal, configured: true, current: 40, status: 'reconnect' })
  await page.route(`**/api/v1/overlays/public/${EDITOR_ID}/goal`, (route) =>
    route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(state()) })
  )
  await page.route(`**/api/v1/overlays/${EDITOR_ID}/goal`, async (route) => {
    if (route.request().method() === 'PUT') {
      const body = route.request().postDataJSON() as typeof goal
      saved.push(body)
      goal = { ...body }
    }
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(state()),
    })
  })
  await page.context().routeWebSocket('**/ws/overlay/**', (ws) => {
    ws.send(JSON.stringify({ type: 'connected', data: { overlay_id: EDITOR_ID } }))
  })
  await page.goto('/')
  await page.evaluate(() => {
    localStorage.setItem(
      'auth-store',
      JSON.stringify({
        state: { user: { id: 'u1', username: 'streamer' }, token: 'mock-jwt' },
        version: 0,
      })
    )
  })
  await page.goto(`/overlays/${EDITOR_ID}`)
  await expect(page.getByLabel('Target')).toBeVisible({ timeout: 30_000 })
  return saved
}

test.describe('Goal editor', () => {
  test('replaces the chat editor and hides the chat-only tools', async ({ page }) => {
    await bootEditor(page)
    const nav = page.getByRole('navigation', { name: 'Overlay settings' })
    await expect(nav.getByRole('button', { name: 'Goal' })).toBeVisible()
    await expect(nav.getByRole('button', { name: 'Typography' })).toHaveCount(0)
    for (const name of ['Monitor View', 'Event Settings', 'Credits']) {
      await expect(page.getByRole('button', { name, exact: true })).toHaveCount(0)
    }
    await expect(page.getByRole('button', { name: /dock URL/i })).toHaveCount(0)
    // The last poll could not read the count: the editor says why.
    await expect(page.getByRole('status').filter({ hasText: 'reconnect' })).toBeVisible()
    // The preview pane renders the goal bar itself, passive (no demand).
    const preview = page.frameLocator(`iframe[src$="/overlay/${EDITOR_ID}?passive=true"]`)
    await expect(preview.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '40')
    await page.screenshot({ path: test.info().outputPath('goal-editor.png') })
  })

  test('saves the chosen metric with its platform', async ({ page }) => {
    const saved = await bootEditor(page)
    await page.getByLabel('What the bar counts').click()
    await page.getByRole('option', { name: 'Twitch bits' }).click()
    await page.getByLabel('Target').fill('5000')
    await page.getByLabel('Title (optional)').fill('Bits for the new mic')
    await page.getByRole('button', { name: 'Save goal' }).click()
    await expect(page.getByRole('region', { name: 'Notifications' })).toContainText('Goal saved')
    expect(saved).toEqual([
      expect.objectContaining({
        metric: 'bits',
        platform: 'twitch',
        target: 5000,
        title: 'Bits for the new mic',
        reset_progress: false,
      }),
    ])
  })
})
