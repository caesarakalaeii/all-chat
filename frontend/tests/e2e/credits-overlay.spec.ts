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
 * The credit roll OBS source: the roll scrolls up at the configured speed, and
 * enabled clips play behind it through Twitch's clip embed, one after another.
 * The public config and credit-roll reads are mocked, and the clip embed is
 * stubbed so the test never reaches Twitch.
 */

const OVERLAY_ID = 'credits-e2e-overlay'

const entry = (rank: number) => ({
  rank,
  user_id: `u${rank}`,
  display_name: `Supporter ${rank}`,
  avatar_url: '',
  platform: 'twitch',
  total_value: 10 * rank,
})

const clip = (id: string, title: string) => ({
  id,
  url: `https://clips.twitch.tv/${id}`,
  embed_url: `https://clips.twitch.tv/embed?clip=${id}`,
  title,
  view_count: 1,
  created_at: '2026-10-17T10:00:00Z',
  thumbnail_url: '',
  duration: 1,
})

async function boot(page: Page, config: Record<string, unknown>) {
  await page.route(`**/api/v1/overlays/public/${OVERLAY_ID}/creditroll`, (route) =>
    route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ theme: 'cinematic', background_opacity: 0.8, ...config }),
    })
  )
  await page.route(`**/api/v1/overlays/public/${OVERLAY_ID}/credit-roll`, (route) =>
    route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        overlay_id: OVERLAY_ID,
        session_id: 's1',
        session_started_at: '2026-10-17T10:00:00Z',
        session_duration_seconds: 17940,
        leaderboards: { gifts: [1, 2, 3, 4, 5].map(entry) },
        clips: [clip('ClipOne', 'First clip'), clip('ClipTwo', 'Second clip')],
        clips_is_fallback: false,
      }),
    })
  )
  await page.route('https://clips.twitch.tv/**', (route) =>
    route.fulfill({ status: 200, contentType: 'text/html', body: '<html></html>' })
  )
  await page.setViewportSize({ width: 1280, height: 720 })
  await page.goto(`/overlay/${OVERLAY_ID}/credits`)
}

const headingTop = (page: Page) =>
  page.getByRole('heading', { level: 1 }).evaluate((el) => el.getBoundingClientRect().top)

test('the roll starts below the screen and scrolls up', async ({ page }) => {
  await boot(page, { scroll_speed: 100, display_duration_seconds: 60 })
  await expect(page.getByRole('heading', { level: 1 })).toBeAttached()

  const first = await headingTop(page)
  expect(first).toBeGreaterThan(600)
  await expect.poll(() => headingTop(page)).toBeLessThan(first - 50)
})

test('clips play through the Twitch clip embed and advance after their duration', async ({
  page,
}) => {
  await boot(page, { scroll_speed: 50, display_duration_seconds: 60, clips_enabled: true })

  const player = page.locator('iframe[src^="https://clips.twitch.tv/embed"]')
  await expect(player).toHaveAttribute('src', /clip=ClipOne/)
  await expect(player).toHaveAttribute('src', /parent=localhost/)
  await expect(player).toHaveAttribute('src', /autoplay=true/)
  await expect(player).toHaveAttribute('src', /muted=true/)
  await expect(page.getByText('First clip')).toBeVisible()

  await expect(player).toHaveAttribute('src', /clip=ClipTwo/, { timeout: 10_000 })
  await expect(page.getByText('Second clip')).toBeVisible()
})
