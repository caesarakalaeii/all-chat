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
 * The credit roll editor previews the OBS roll beside its settings: an unsaved
 * text or size edit shows in the preview at once, the preview never fetches the
 * real roll (that fetch counts as a display), and saving sends the custom texts
 * and sizes.
 */

const OVERLAY_ID = 'credits-editor-e2e'

const json = (body: unknown) => ({
  status: 200,
  contentType: 'application/json',
  body: JSON.stringify(body),
})

async function boot(page: Page) {
  const saved: Array<Record<string, unknown>> = []
  let rollFetches = 0
  await page.route('**/api/v1/**', (route) =>
    route.fulfill({ status: 404, contentType: 'application/json', body: '{}' })
  )
  await page.route('**/api/v1/auth/me', (route) =>
    route.fulfill(
      json({ id: 'u1', username: 'streamer', onboarding_completed_at: '2026-01-01T00:00:00Z' })
    )
  )
  await page.route(`**/api/v1/overlays/${OVERLAY_ID}`, (route) =>
    route.fulfill(json({ id: OVERLAY_ID, user_id: 'u1', name: 'Main', overlay_type: 'chat' }))
  )
  await page.route(`**/api/v1/overlays/${OVERLAY_ID}/creditroll`, async (route) => {
    if (route.request().method() === 'POST') saved.push(route.request().postDataJSON())
    await route.fulfill(
      json({
        id: 'cfg',
        overlay_id: OVERLAY_ID,
        enabled: true,
        theme: 'cinematic',
        background_opacity: 0.8,
        texts: { subheading: 'Merci à tous !' },
        font_scales: {},
      })
    )
  })
  await page.route(`**/api/v1/overlays/public/${OVERLAY_ID}/credit-roll`, (route) => {
    rollFetches++
    return route.fulfill({ status: 500, body: '{}' })
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
  await page.setViewportSize({ width: 1440, height: 900 })
  await page.goto(`/overlays/${OVERLAY_ID}/credits`)
  const preview = page.frameLocator('iframe[src*="/credits?preview=true"]')
  return { preview, saved, rollFetches: () => rollFetches }
}

test('the preview shows saved and unsaved custom text and sizes, and saving sends them', async ({
  page,
}) => {
  const { preview, saved, rollFetches } = await boot(page)

  // Saved custom text and the sample supporters, without the real roll.
  await expect(preview.getByText('Merci à tous !')).toBeVisible({ timeout: 20_000 })
  await expect(preview.getByText('GiftGoblin')).toBeVisible()

  await page.getByLabel('🎬 Stream Credits').fill('Générique de fin')
  await expect(preview.getByRole('heading', { level: 1 })).toHaveText('Générique de fin')

  await page.getByLabel('Session: {date}').fill('Session du {day}/{month}/{year}')
  await expect(preview.getByText(/^Session du \d{2}\/\d{2}\/2026$/)).toBeVisible()

  const session = preview.getByText(/^Session du/)
  const before = await session.evaluate((el) => parseFloat(getComputedStyle(el).fontSize))
  const sessionSize = page.getByLabel('Session and duration')
  await sessionSize.focus()
  for (let i = 0; i < 10; i++) await sessionSize.press('ArrowRight')
  await expect(sessionSize).toHaveValue('200')
  await expect
    .poll(() => session.evaluate((el) => parseFloat(getComputedStyle(el).fontSize)))
    .toBeCloseTo(before * 2, 0)

  await page.getByRole('button', { name: 'Save Settings' }).click()
  await expect.poll(() => saved.length).toBe(1)
  expect(saved[0].texts).toEqual({
    subheading: 'Merci à tous !',
    heading: 'Générique de fin',
    session: 'Session du {day}/{month}/{year}',
  })
  expect(saved[0].font_scales).toEqual({ session: 200 })
  expect(rollFetches()).toBe(0)
})
