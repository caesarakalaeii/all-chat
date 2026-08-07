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
 * End-to-end cover for the moderator's side of delegation (ADR-0048): finding a channel
 * someone handed you, and redeeming the invite that handed it over.
 *
 * The moderation API is stubbed at the network boundary, so this exercises routing, the
 * auth guard and the real pages together — which the component tests cannot. The path
 * under test is the one with no alternative: `GET /api/v1/overlays` is owner-filtered, so
 * if this list breaks, an accepted grant becomes unreachable.
 */

import { test, expect } from '@playwright/test'

const OVERLAY_ID = 'aaaaaaaa-1111-1111-1111-111111111111'

const USER = {
  id: 'user-2',
  username: 'sarah',
  display_name: 'Sarah',
  auth_provider: 'twitch',
  onboarding_completed_at: '2026-01-01T00:00:00Z',
}

const DELEGATION = {
  grant_id: 'grant-1',
  overlay_id: OVERLAY_ID,
  overlay_name: 'Main Overlay',
  owner_display_name: 'SomeStreamer',
  status: 'active',
  actions: ['delete', 'timeout'],
  platforms: [{ platform: 'twitch', enabled: true, verification: 'unverified' }],
  available: true,
  accepted_at: '2026-08-01T11:00:00Z',
}

const INVITE_SECRET = 'e2e-invite-secret-value'

function json(body: unknown, status = 200) {
  return { status, contentType: 'application/json', body: JSON.stringify(body) }
}

test.describe('moderator — channels you moderate', () => {
  test.beforeEach(async ({ page }) => {
    await page.emulateMedia({ reducedMotion: 'reduce' })
    // Catch-all first — Playwright matches routes last-registered-first.
    await page.route('**/api/v1/**', (route) => route.fulfill(json({}, 404)))
    await page.route('**/api/v1/auth/me', (route) => route.fulfill(json(USER)))
  })

  test('lists a delegated channel and links to its monitor', async ({ page }) => {
    await page.route('**/api/v1/moderation/delegations', (route) =>
      route.fulfill(json({ delegations: [DELEGATION] }))
    )

    await page.goto('/moderate')

    await expect(page.getByRole('heading', { name: 'Channels you moderate' })).toBeVisible({
      timeout: 20_000,
    })
    await expect(page.getByText('Main Overlay')).toBeVisible()
    await expect(page.getByText('for SomeStreamer')).toBeVisible()
    await expect(page.getByRole('link', { name: /open chat monitor/i })).toHaveAttribute(
      'href',
      `/overlay/${OVERLAY_ID}/view`
    )
  })

  test('explains how to get a channel when there are none', async ({ page }) => {
    await page.route('**/api/v1/moderation/delegations', (route) =>
      route.fulfill(json({ delegations: [] }))
    )

    await page.goto('/moderate')

    await expect(page.getByRole('heading', { name: 'No channels yet' })).toBeVisible({
      timeout: 20_000,
    })
  })

  // Entitlement is keyed on the overlay OWNER, so a lapsed plan is the streamer's. A
  // volunteer must never be pointed at an upgrade page for a plan that is not theirs.
  test('names the streamer plan for an unavailable channel, with no upgrade link', async ({
    page,
  }) => {
    await page.route('**/api/v1/moderation/delegations', (route) =>
      route.fulfill(json({ delegations: [{ ...DELEGATION, available: false }] }))
    )

    await page.goto('/moderate')

    await expect(page.getByText(/SomeStreamer's plan does not include moderation/)).toBeVisible({
      timeout: 20_000,
    })
    await expect(page.getByRole('link', { name: /upgrade/i })).toHaveCount(0)
  })
})

test.describe('moderator — invite acceptance', () => {
  test.beforeEach(async ({ page }) => {
    await page.emulateMedia({ reducedMotion: 'reduce' })
    await page.route('**/api/v1/**', (route) => route.fulfill(json({}, 404)))
    await page.route('**/api/v1/auth/me', (route) => route.fulfill(json(USER)))
  })

  test('previews an invite and redeems it into the overlay it discloses', async ({ page }) => {
    const seenPaths: string[] = []
    await page.route('**/api/v1/moderation/invites/preview', (route) => {
      seenPaths.push(route.request().url())
      return route.fulfill(
        json({
          overlay_name: 'Main Overlay',
          owner_display_name: 'SomeStreamer',
          actions: ['delete', 'timeout'],
          platforms: [{ platform: 'twitch', enabled: true, verification: 'unverified' }],
          expires_at: '2026-08-14T10:00:00Z',
        })
      )
    })
    await page.route('**/api/v1/moderation/invites/accept', (route) => {
      seenPaths.push(route.request().url())
      return route.fulfill(
        json({
          grant_id: 'grant-1',
          overlay_id: OVERLAY_ID,
          overlay_name: 'Main Overlay',
          owner_display_name: 'SomeStreamer',
          actions: ['delete', 'timeout'],
          platforms: [],
        })
      )
    })

    await page.goto(`/moderate/accept?token=${INVITE_SECRET}`)

    await expect(page.getByText('Main Overlay')).toBeVisible({ timeout: 20_000 })
    await expect(page.getByText('Delete messages')).toBeVisible()

    await page.getByRole('button', { name: /accept and start moderating/i }).click()
    await expect(page).toHaveURL(new RegExp(`/overlay/${OVERLAY_ID}/view`))

    // The secret travels in the body on every call: a path parameter would be captured by
    // every access log and Referer header between the browser and the service.
    expect(seenPaths.length).toBeGreaterThan(0)
    for (const url of seenPaths) {
      expect(url).not.toContain(INVITE_SECRET)
    }
  })

  test('sends a dead invite somewhere it can act instead of a raw error', async ({ page }) => {
    await page.route('**/api/v1/moderation/invites/preview', (route) =>
      route.fulfill(json({ error: 'invite not found', code: 'invite_not_found' }, 404))
    )

    await page.goto(`/moderate/accept?token=${INVITE_SECRET}`)

    await expect(page.getByText(/already have been used/i)).toBeVisible({ timeout: 20_000 })
    await expect(page.getByRole('link', { name: /go to your channels/i })).toBeVisible()
  })
})
