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

import { readFile } from 'node:fs/promises'
import { test, expect, type Page } from '@playwright/test'

/**
 * "Download my data" (DSGVO Art. 15/20). The privacy policy sends streamers to
 * /settings and viewers to /settings/viewer for a machine-readable export; each
 * button must fetch the export with that account's own credential and hand the
 * browser a JSON file. The API is mocked: the export's contents are pinned by
 * the auth-service integration tests, this pins the browser end of it.
 */

const USER = {
  id: 'user-1',
  username: 'tester',
  display_name: 'Tester',
  auth_provider: 'twitch',
  onboarding_completed_at: '2026-01-01T00:00:00Z',
}

const EXPORT = { format_version: 2, exported_at: '2026-11-02T12:00:00Z', overlays: [] }

function json(body: unknown) {
  return { status: 200, contentType: 'application/json', body: JSON.stringify(body) }
}

async function downloadFrom(page: Page) {
  const [download] = await Promise.all([
    page.waitForEvent('download'),
    page.getByRole('button', { name: 'Download my data' }).click(),
  ])
  const path = await download.path()
  return { name: download.suggestedFilename(), body: JSON.parse(await readFile(path, 'utf8')) }
}

test.beforeEach(async ({ page }) => {
  // Catch-all first — Playwright matches routes last-registered-first.
  await page.route('**/api/v1/**', (route) =>
    route.fulfill({ status: 404, contentType: 'application/json', body: '{}' })
  )
})

test('a streamer downloads their export from /settings', async ({ page }) => {
  await page.route('**/api/v1/auth/me', (route) => route.fulfill(json(USER)))
  let requested = false
  await page.route('**/api/v1/auth/me/data-export', (route) => {
    requested = true
    return route.fulfill(json({ ...EXPORT, user: { username: USER.username } }))
  })

  await page.goto('/settings')
  const file = await downloadFrom(page)

  expect(requested).toBe(true)
  expect(file.name).toMatch(/^allchat-data-export-\d{4}-\d{2}-\d{2}\.json$/)
  expect(file.body).toEqual({ ...EXPORT, user: { username: USER.username } })
})

test('a viewer downloads their export from /settings/viewer with the viewer token', async ({
  page,
}) => {
  const claims = { viewer_id: 'viewer-1', display_name: 'Viewer', platform: 'twitch', exp: 4e9 }
  const token = `x.${Buffer.from(JSON.stringify(claims)).toString('base64')}.x`
  await page.addInitScript((t) => localStorage.setItem('viewer_jwt_token', t), token)
  let authorization: string | undefined
  await page.route('**/api/v1/auth/viewer/data-export', (route) => {
    authorization = route.request().headers()['authorization']
    return route.fulfill(json(EXPORT))
  })

  await page.goto('/settings/viewer')
  await expect(page.getByRole('heading', { name: 'Your data' })).toBeVisible()
  const file = await downloadFrom(page)

  expect(authorization).toBe(`Bearer ${token}`)
  expect(file.name).toMatch(/^allchat-viewer-data-export-\d{4}-\d{2}-\d{2}\.json$/)
  expect(file.body).toEqual(EXPORT)
})
