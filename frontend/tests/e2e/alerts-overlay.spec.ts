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
 * The alert-box overlay (ADR-0068, #953): an alerts overlay's render route
 * plays `type: "alert"` frames one at a time, dedups by alert_id, and renders
 * nothing between alerts. The gateway and the public config are mocked; the
 * frame is exactly what api-gateway forwards for POST /overlays/:id/alerts/test.
 */

const OVERLAY_ID = 'alerts-e2e-overlay'

const alertFrame = (id: string, text: string, durationMs = 1500) =>
  JSON.stringify({
    type: 'alert',
    data: {
      alert_id: id,
      overlay_id: OVERLAY_ID,
      platform: 'twitch',
      event_type: 'bits',
      event_data: {
        type: 'bits',
        tier: 'high',
        value: { amount: 100, currency: 'bits', display_text: '100 bits' },
        duration: 8,
        is_update: false,
        metadata: { test: true },
      },
      user: { id: 'e2e', name: 'E2ECheck', avatar_url: '' },
      message: '',
      occurred_at: '2026-10-16T12:00:00.000Z',
      profile: {
        event_type: 'bits',
        text_template: '{username} cheered {amount}!',
        sound_url: '',
        image_url: '',
        duration_ms: durationMs,
        volume: 100,
        enabled: true,
        sort_after: 0,
        text,
      },
    },
    timestamp: '2026-10-16T12:00:00.000Z',
  })

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
      body: JSON.stringify({ overlay_type: 'alerts', display_settings: {}, sources: [] }),
    })
  )
  await page.setViewportSize({ width: 800, height: 450 })
  await page.goto(`/overlay/${OVERLAY_ID}`)
  await expect.poll(() => sockets.length, { timeout: 15_000 }).toBeGreaterThan(0)
  return (frame: string) => sockets[sockets.length - 1].send(frame)
}

test.describe('Alerts overlay', () => {
  test('a test alert appears, then the overlay is empty again', async ({ page }) => {
    const send = await boot(page)
    await expect(page.getByRole('status')).toHaveCount(0)

    send(alertFrame('a1', 'E2ECheck cheered 100 bits!'))

    await expect(page.getByRole('status')).toHaveText('E2ECheck cheered 100 bits!')
    // OBS-transparent: the page paints no background of its own.
    expect(await page.evaluate(() => getComputedStyle(document.body).backgroundColor)).toBe(
      'rgba(0, 0, 0, 0)'
    )
    await expect(page.getByRole('status')).toHaveCount(0, { timeout: 5_000 })
  })

  test('plays queued alerts in order and drops a redelivered one', async ({ page }) => {
    const send = await boot(page)

    send(alertFrame('a1', 'first alert', 2000))
    send(alertFrame('a1', 'first alert', 2000))
    send(alertFrame('a2', 'second alert', 1000))

    await expect(page.getByRole('status')).toHaveText('first alert')
    // One 2s play of a1, not two: a replayed duplicate would push a2 past 4s.
    await expect(page.getByRole('status')).toHaveText('second alert', { timeout: 3_000 })
    await expect(page.getByRole('status')).toHaveCount(0, { timeout: 3_000 })
  })
})
