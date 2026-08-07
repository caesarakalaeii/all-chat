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
 * End-to-end cover for the owner-side Moderators panel (ADR-0048), driving the real
 * editor page through the real left nav with the moderation API stubbed at the network
 * boundary — so this exercises routing, the registry entry and the panel together,
 * which the component tests cannot.
 */

import { test, expect } from '@playwright/test'

const OVERLAY = {
  id: 'aaaaaaaa-1111-1111-1111-111111111111',
  name: 'Main Overlay',
  is_active: true,
}

const USER = {
  id: 'user-1',
  username: 'tester',
  display_name: 'Tester',
  auth_provider: 'twitch',
  onboarding_completed_at: '2026-01-01T00:00:00Z',
}

const GRANT = {
  id: 'grant-1',
  status: 'active',
  moderator_user_id: 'user-2',
  display_name: 'Sarah',
  actions: ['delete', 'timeout'],
  platforms: [{ platform: 'twitch', enabled: true, verification: 'verified' }],
  created_at: '2026-08-01T10:00:00Z',
  accepted_at: '2026-08-01T11:00:00Z',
}

const INVITE_SECRET = 'e2e-invite-secret-value'

function json(body: unknown, status = 200) {
  return { status, contentType: 'application/json', body: JSON.stringify(body) }
}

const MODERATORS_URL = `**/api/v1/moderation/overlays/${OVERLAY.id}/moderators`

test.describe('overlay editor — Moderators panel', () => {
  test.beforeEach(async ({ page }) => {
    await page.emulateMedia({ reducedMotion: 'reduce' })
    // Catch-all first — Playwright matches routes last-registered-first.
    await page.route('**/api/v1/**', (route) => route.fulfill(json({}, 404)))
    await page.route('**/api/v1/auth/me', (route) => route.fulfill(json(USER)))
    await page.route(`**/api/v1/overlays/${OVERLAY.id}`, (route) => route.fulfill(json(OVERLAY)))
    await page.route(`**/api/v1/overlays/${OVERLAY.id}/sources`, (route) => route.fulfill(json([])))
    await page.route(`**/api/v1/overlays/${OVERLAY.id}/config`, (route) => route.fulfill(json({})))
  })

  async function openModerators(page: import('@playwright/test').Page) {
    await page.goto(`/overlays/${OVERLAY.id}`)
    await expect(page.getByText(OVERLAY.name)).toBeVisible({ timeout: 20_000 })
    await page
      .getByRole('navigation', { name: 'Overlay settings' })
      .getByRole('button', { name: 'Moderators' })
      .click()
    await expect(page.getByRole('heading', { name: 'Moderators' })).toBeVisible()
  }

  test('is reachable from the settings nav and lists the current moderators', async ({ page }) => {
    await page.route(MODERATORS_URL, (route) =>
      route.fulfill(json({ moderators: [GRANT], cap: 10, used: 1 }))
    )
    await openModerators(page)

    await expect(page.getByText('Sarah')).toBeVisible()
    await expect(page.getByText('1 of 10 used')).toBeVisible()
  })

  test('is findable through settings search, which is what the nav is for', async ({ page }) => {
    await page.route(MODERATORS_URL, (route) =>
      route.fulfill(json({ moderators: [], cap: 10, used: 0 }))
    )
    await page.goto(`/overlays/${OVERLAY.id}`)
    await expect(page.getByText(OVERLAY.name)).toBeVisible({ timeout: 20_000 })

    const search = page.getByRole('combobox', { name: 'Search settings' })
    await search.fill('mod')
    const listbox = page.getByRole('listbox', { name: 'Matching settings' })
    await expect(listbox).toBeVisible()
    await listbox
      .getByRole('option', { name: /Invite a moderator/ })
      .first()
      .click()

    await expect(page.getByRole('heading', { name: 'Moderators' })).toBeVisible()
  })

  test('creates an invite, reveals the secret once, and does not replay it', async ({ page }) => {
    let created = false
    await page.route(MODERATORS_URL, (route) => {
      if (route.request().method() === 'POST') {
        created = true
        return route.fulfill(
          json({
            grant_id: 'grant-9',
            invite_token: INVITE_SECRET,
            expires_at: '2026-08-14T10:00:00Z',
            actions: ['delete', 'timeout'],
            platforms: ['twitch'],
          })
        )
      }
      // After creation the roster carries a pending invite — but never the secret.
      return route.fulfill(
        json(
          created
            ? {
                moderators: [
                  {
                    ...GRANT,
                    id: 'grant-9',
                    status: 'pending',
                    display_name: undefined,
                    moderator_user_id: undefined,
                    invitee_label: 'Sarah',
                    invite_expires_at: '2026-08-14T10:00:00Z',
                  },
                ],
                cap: 10,
                used: 1,
              }
            : { moderators: [], cap: 10, used: 0 }
        )
      )
    })
    await openModerators(page)

    await page.getByRole('button', { name: 'Invite a moderator' }).click()
    await page.getByLabel(/who is this for/i).fill('Sarah')
    await page.getByRole('checkbox', { name: 'Twitch' }).check()
    await page.getByRole('button', { name: 'Create invite' }).click()

    // Shown once, with copy that does not promise a second chance.
    await expect(page.getByText(INVITE_SECRET)).toBeVisible()
    await expect(page.getByText(/won't be shown again/i)).toBeVisible()

    await page.getByRole('button', { name: 'Done' }).click()
    await expect(page.getByText(INVITE_SECRET)).toHaveCount(0)
    await expect(page.getByText('Invite pending')).toBeVisible()
    // The roster is the only source now, and it has no way to expose the secret.
    await expect(page.locator('body')).not.toContainText(INVITE_SECRET)
  })

  test('will not offer an invite once the overlay is at its cap', async ({ page }) => {
    await page.route(MODERATORS_URL, (route) =>
      route.fulfill(
        json({
          moderators: Array.from({ length: 10 }, (_, i) => ({ ...GRANT, id: `g${i}` })),
          cap: 10,
          used: 10,
        })
      )
    )
    await openModerators(page)

    await expect(page.getByRole('button', { name: 'Invite a moderator' })).toBeDisabled()
    await expect(page.getByText(/at its limit of 10 moderators/i)).toBeVisible()
  })

  test('removes a moderator only after confirmation', async ({ page }) => {
    let deleteCalls = 0
    await page.route(`${MODERATORS_URL}/grant-1`, (route) => {
      deleteCalls += 1
      return route.fulfill(json({ revoked: true }))
    })
    await page.route(MODERATORS_URL, (route) =>
      route.fulfill(
        json(
          deleteCalls > 0
            ? { moderators: [], cap: 10, used: 0 }
            : { moderators: [GRANT], cap: 10, used: 1 }
        )
      )
    )
    await openModerators(page)

    // The row's own control, not the text: the confirm dialog's title also names Sarah.
    const removeSarah = page.getByRole('button', { name: 'Remove Sarah' })
    await removeSarah.click()
    await expect(page.getByRole('alertdialog')).toBeVisible()

    // Backing out must leave the grant alone — no request, still on the roster.
    await page.getByRole('button', { name: 'Cancel' }).click()
    await expect(removeSarah).toBeVisible()
    expect(deleteCalls).toBe(0)

    await removeSarah.click()
    await page.getByRole('button', { name: 'Remove', exact: true }).click()
    await expect(page.getByText(/no one moderates this overlay yet/i)).toBeVisible()
    expect(deleteCalls).toBe(1)
  })

  // The gate refusal has to be actionable and stay put, not vanish as a toast.
  test('shows an inline upgrade path when the premium gate refuses the invite', async ({
    page,
  }) => {
    await page.route(MODERATORS_URL, (route) => {
      if (route.request().method() === 'POST') {
        return route.fulfill(
          json(
            {
              error: 'delegating moderation requires All-Chat premium',
              code: 'delegation_unavailable',
              upgrade_url: '/upgrade',
            },
            403
          )
        )
      }
      return route.fulfill(json({ moderators: [GRANT], cap: 10, used: 1 }))
    })
    await openModerators(page)

    await page.getByRole('button', { name: 'Invite a moderator' }).click()
    await page.getByRole('button', { name: 'Create invite' }).click()

    const upgrade = page.getByRole('link', { name: /upgrade to invite moderators/i })
    await expect(upgrade).toBeVisible()
    await expect(upgrade).toHaveAttribute('href', '/upgrade')

    // Revocation is never gated server-side; the UI must not gate it either.
    await page.getByRole('button', { name: 'Cancel' }).click()
    await expect(page.getByRole('button', { name: 'Remove Sarah' })).toBeEnabled()
    await expect(page.getByRole('button', { name: 'Remove all' })).toBeEnabled()
  })
})
