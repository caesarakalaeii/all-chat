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
 * enabled clips play behind it one after another, as the clip's MP4 when
 * Twitch's web GQL hands out a signed URL (ADR-0076) and through the muted clip
 * embed otherwise. The public config and credit-roll reads, the GQL lookup and
 * the clip embed are mocked, so the test never reaches Twitch.
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

const CLIP_MEDIA = 'https://clip-media.test'

/** Answers the clip lookup the way gql.twitch.tv does, or fails it. */
async function mockClipLookup(page: Page, ok: boolean) {
  await page.route('https://gql.twitch.tv/gql', (route) => {
    if (!ok) return route.fulfill({ status: 500, body: '' })
    const { variables } = route.request().postDataJSON()
    return route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        data: {
          clip: {
            playbackAccessToken: { signature: 'sig123', value: '{"clip_slug":"x"}' },
            videoQualities: [
              { quality: '1080', sourceURL: `${CLIP_MEDIA}/${variables.slug}-1080.mp4` },
              { quality: '720', sourceURL: `${CLIP_MEDIA}/${variables.slug}-720.mp4` },
            ],
          },
        },
      }),
    })
  })
  // Never answered: the MP4 stays loading, so only the test ends a clip.
  await page.route(`${CLIP_MEDIA}/**`, () => {})
}

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

test('clips play as the MP4 from Twitch, with sound unless muted, and advance when one ends', async ({
  page,
}) => {
  await mockClipLookup(page, true)
  await boot(page, { clips_enabled: true, clips_muted: false })

  const video = page.locator('video')
  await expect(video).toHaveAttribute('src', /ClipOne-1080\.mp4\?sig=sig123&token=/)
  // Starts muted and takes the streamer's audio choice once the clip can play.
  const mutedOnceLoaded = () =>
    video.evaluate((v: HTMLVideoElement) => {
      v.dispatchEvent(new Event('loadeddata'))
      return v.muted
    })
  expect(await mutedOnceLoaded()).toBe(false)
  await expect(page.locator('iframe')).toHaveCount(0)
  await expect(page.getByText('First clip')).toBeVisible()

  await video.evaluate((v) => v.dispatchEvent(new Event('ended')))
  await expect(video).toHaveAttribute('src', /ClipTwo-1080\.mp4/)
  await expect(page.getByText('Second clip')).toBeVisible()
})

test('clips fall back to the muted clip embed when the lookup fails', async ({ page }) => {
  await mockClipLookup(page, false)
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
