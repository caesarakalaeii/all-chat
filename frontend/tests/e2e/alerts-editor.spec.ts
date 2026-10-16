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

import { test, expect, type Page, type WebSocketRoute } from '@playwright/test'

/**
 * The alerts editor (issue #954): an alerts overlay's dashboard editor edits
 * per-event alert profiles, saves them with PUT /overlays/:id/alerts/profiles,
 * and the test alert plays the change in the editor's preview pane — the same
 * alert box OBS renders, fed over the overlay WebSocket. The API and the
 * gateway are mocked; the mocked test endpoint answers like api-gateway does,
 * with the saved profile resolved into the frame.
 */

const OVERLAY_ID = 'alerts-editor-e2e'

const EVENT_TYPES = ['subscription', 'bits']
const defaults = EVENT_TYPES.map((event_type) => ({
  event_type,
  text_template: '{username} just subscribed!',
  sound_url: '',
  image_url: '',
  duration_ms: 4000,
  volume: 100,
  enabled: true,
  sort_after: 0,
  custom_html: '',
  custom_css: '',
  custom_js: '',
}))

interface Harness {
  saved: Array<Record<string, unknown>>
}

async function boot(page: Page, { premium = true } = {}): Promise<Harness> {
  const harness: Harness = { saved: [] }
  let profiles = structuredClone(defaults)
  const sockets: WebSocketRoute[] = []

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
        is_premium: premium,
        onboarding_completed_at: '2026-01-01T00:00:00Z',
      }),
    })
  )
  await page.route(`**/api/v1/overlays/${OVERLAY_ID}`, (route) =>
    route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        id: OVERLAY_ID,
        user_id: 'u1',
        name: 'Alert box',
        overlay_type: 'alerts',
        is_active: true,
      }),
    })
  )
  await page.route(`**/api/v1/overlays/${OVERLAY_ID}/sources`, (route) =>
    route.fulfill({ status: 200, contentType: 'application/json', body: '[]' })
  )
  await page.route(`**/api/v1/overlays/public/${OVERLAY_ID}/config`, (route) =>
    route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ overlay_type: 'alerts', display_settings: {}, sources: [] }),
    })
  )
  await page.route(`**/api/v1/overlays/${OVERLAY_ID}/alerts/profiles`, async (route) => {
    if (route.request().method() === 'PUT') {
      const body = route.request().postDataJSON() as { profiles: typeof defaults }
      harness.saved.push(...body.profiles)
      profiles = profiles.map((p) => body.profiles.find((b) => b.event_type === p.event_type) ?? p)
    }
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ profiles }),
    })
  })
  // The gateway's test-alert path: resolve the saved profile into the frame.
  await page.route(`**/api/v1/overlays/${OVERLAY_ID}/alerts/test`, async (route) => {
    const { event_type } = route.request().postDataJSON() as { event_type: string }
    const profile = profiles.find((p) => p.event_type === event_type)!
    const alertId = `t-${Date.now()}`
    const frame = {
      type: 'alert',
      data: {
        alert_id: alertId,
        overlay_id: OVERLAY_ID,
        platform: 'twitch',
        event_type,
        event_data: {
          type: event_type,
          value: { amount: 1, currency: 'months', display_text: 'Tier 1' },
        },
        user: { id: 'all-chat-test', name: 'AllChatTester', avatar_url: '' },
        message: 'This is a test alert',
        occurred_at: new Date().toISOString(),
        profile: {
          ...profile,
          text: profile.text_template.split('{username}').join('AllChatTester'),
        },
      },
      timestamp: new Date().toISOString(),
    }
    sockets.forEach((ws) => ws.send(JSON.stringify(frame)))
    await route.fulfill({
      status: 202,
      contentType: 'application/json',
      body: JSON.stringify({ alert_id: alertId }),
    })
  })
  await page.context().routeWebSocket('**/ws/overlay/**', (ws) => {
    sockets.push(ws)
    ws.send(JSON.stringify({ type: 'connected', data: { overlay_id: OVERLAY_ID } }))
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
  await page.goto(`/overlays/${OVERLAY_ID}`)
  await expect(page.getByLabel('Alert text')).toBeVisible({ timeout: 30_000 })
  // The preview pane's alert box has connected its socket.
  await expect.poll(() => sockets.length, { timeout: 30_000 }).toBeGreaterThan(0)
  return harness
}

const preview = (page: Page) => page.frameLocator('iframe[src$="/preview/alerts"]')

test.describe('Alerts editor', () => {
  test('replaces the chat editor for an alerts overlay', async ({ page }) => {
    await boot(page)
    const nav = page.getByRole('navigation', { name: 'Overlay settings' })
    await expect(nav.getByRole('button', { name: 'Alerts' })).toBeVisible()
    await expect(nav.getByRole('button', { name: 'Typography' })).toHaveCount(0)
    await expect(page.getByRole('button', { name: 'Send test alert' })).toBeVisible()
  })

  test('free-tier edit, save, then the test alert shows the change', async ({ page }) => {
    const harness = await boot(page)

    await page.getByLabel('Alert text').fill('{username} joined the party!')
    await page.getByLabel('Duration (seconds)').fill('8')
    await page.getByRole('button', { name: 'Save alerts' }).click()
    await expect(page.getByRole('status').filter({ hasText: 'Alerts saved' })).toBeVisible()
    expect(harness.saved).toEqual([
      expect.objectContaining({
        event_type: 'subscription',
        text_template: '{username} joined the party!',
        duration_ms: 8000,
      }),
    ])

    await page.getByRole('button', { name: 'Send test alert' }).click()
    await expect(preview(page).getByRole('status')).toHaveText('AllChatTester joined the party!')
  })

  test('custom code renders inside the scripts-only sandbox', async ({ page }) => {
    await boot(page)

    await page.getByLabel('HTML').fill('<h1 id="custom" data-alert-text></h1><p id="js"></p>')
    await page.getByLabel('CSS').fill('h1 { color: rgb(255, 215, 0) }')
    await page
      .getByLabel('JavaScript')
      .fill(
        "onAlert(function (m) { document.getElementById('js').textContent = allchat.fill('JS saw {username}', m.event) })"
      )
    await page.getByRole('button', { name: 'Save alerts' }).click()
    await expect(page.getByRole('status').filter({ hasText: 'Alerts saved' })).toBeVisible()

    await page.getByRole('button', { name: 'Send test alert' }).click()
    const box = preview(page)
    await expect(box.locator('iframe[title="Alert"]')).toHaveAttribute('sandbox', 'allow-scripts')
    const sandbox = box.frameLocator('iframe[title="Alert"]')
    // The page's CSP applies to the srcdoc: this proves inline scripts run under it.
    await expect(sandbox.locator('#js')).toHaveText('JS saw AllChatTester')
    await expect(sandbox.locator('#custom')).toHaveText('AllChatTester just subscribed!')
    await expect(sandbox.locator('#custom')).toHaveCSS('color', 'rgb(255, 215, 0)')
  })

  test('without premium the editor is read-only behind the upgrade link', async ({ page }) => {
    await boot(page, { premium: false })
    await expect(page.getByLabel('Alert text')).toBeDisabled()
    await expect(page.getByText('Custom alert code is a Premium feature.')).toBeVisible()
    await expect(page.getByRole('button', { name: 'Save alerts' })).toBeDisabled()
    // Test alerts stay free.
    await expect(page.getByRole('button', { name: 'Send test alert' })).toBeEnabled()
  })
})
