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
 * The chat commands editor (/commands, ADR-0079, ADR-0080) against a stateful mock
 * of the engagement-service API: create a clip trigger from an example, see the
 * reply previewed, find it in the list, switch it off and delete it; create an
 * image command and see its image previewed.
 *
 * The chat side (a message arriving, the reply and the clip) needs live platform
 * chats and accounts, so it is covered by the Go tests of message-processor and
 * engagement-service rather than here.
 */

const USER = {
  id: '11111111-1111-1111-1111-111111111111',
  username: 'commands_e2e',
  display_name: 'Commands E2E',
  auth_provider: 'twitch',
  is_premium: false,
  onboarding_completed_at: '2026-01-02T00:00:00Z',
}

/** What the preview mock says the image template rendered to, served as a 1x1 PNG. */
const DOG = 'https://images.dog.example/akita.png'
const PIXEL =
  'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR4nGP4z8DwHwAFAAH/iZk9HQAAAABJRU5ErkJggg=='

const json = (body: unknown, status = 200) => ({
  status,
  contentType: 'application/json',
  body: JSON.stringify(body),
})

async function boot(page: Page) {
  const commands: Array<Record<string, unknown>> = []
  const writes: Array<{ method: string; body: Record<string, unknown> | null }> = []

  await page.route('**/api/v1/**', (route) => route.fulfill(json({}, 404)))
  await page.route('**/api/v1/auth/me', (route) => route.fulfill(json(USER)))
  await page.route('**/api/v1/engagement/chat-commands', async (route) => {
    const req = route.request()
    if (req.method() === 'POST') {
      const body = req.postDataJSON()
      writes.push({ method: 'POST', body })
      const created = { ...body, id: `cmd-${commands.length + 1}`, use_count: 0 }
      commands.push(created)
      return route.fulfill(json(created, 201))
    }
    return route.fulfill(
      json({
        commands,
        status: {
          is_premium: false,
          twitch_connected: true,
          twitch_can_send: true,
          twitch_can_clip: false,
          consent_overlay_id: '22222222-2222-2222-2222-222222222222',
        },
      })
    )
  })
  await page.route('**/api/v1/engagement/chat-commands/preview', async (route) => {
    const { response, image } = route.request().postDataJSON()
    const output = String(response)
      .replace('{user}', 'ExampleViewer')
      .replace('{clip}', 'https://clips.twitch.tv/ExampleClip')
    return route.fulfill(json({ output, image: image ? DOG : '', errors: [] }))
  })
  await page.route(DOG, (route) =>
    route.fulfill({ contentType: 'image/png', body: Buffer.from(PIXEL, 'base64') })
  )
  await page.route('**/api/v1/engagement/chat-commands/cmd-*', async (route) => {
    const req = route.request()
    const id = req.url().split('/').pop()
    const i = commands.findIndex((c) => c.id === id)
    if (req.method() === 'DELETE') {
      writes.push({ method: 'DELETE', body: null })
      commands.splice(i, 1)
      return route.fulfill({ status: 204 })
    }
    const body = req.postDataJSON()
    writes.push({ method: 'PUT', body })
    commands[i] = { ...commands[i], ...body }
    return route.fulfill(json(commands[i]))
  })

  await page.addInitScript(() => localStorage.setItem('cookieBannerAcknowledged', 'true'))
  await page.goto('/commands')
  await expect(page.getByRole('heading', { name: 'Chat commands', level: 1 })).toBeVisible({
    timeout: 20_000,
  })
  return { writes }
}

test('creates a clip trigger from an example, then switches it off and deletes it', async ({
  page,
}) => {
  const { writes } = await boot(page)

  // The clip grant is missing, so the panel offers it.
  await expect(page.getByRole('button', { name: 'Allow on Twitch' })).toBeVisible()

  await page.getByRole('button', { name: 'Clip on cue' }).click()
  const dialog = page.getByRole('dialog')
  await expect(dialog.getByLabel('Trigger', { exact: true })).toHaveValue('clip that')
  await expect(dialog.getByText('Clipped! https://clips.twitch.tv/ExampleClip')).toBeVisible()
  await dialog.getByRole('button', { name: 'Save' }).click()
  await expect(dialog).toBeHidden()

  expect(writes[0]).toEqual({
    method: 'POST',
    body: expect.objectContaining({
      kind: 'keyword',
      trigger: 'clip that',
      create_clip: true,
      min_role: 'everyone',
    }),
  })

  const row = page.getByRole('listitem').filter({ hasText: 'clip that' })
  await expect(row.getByText('Keyword')).toBeVisible()
  await expect(row.getByText('Clips')).toBeVisible()

  await row.getByRole('switch', { name: 'Enable clip that' }).click()
  await expect.poll(() => writes[writes.length - 1]?.body?.enabled).toBe(false)

  await row.getByRole('button', { name: 'Delete clip that' }).click()
  await page.getByRole('alertdialog').getByRole('button', { name: 'Delete' }).click()
  await expect(row).toHaveCount(0)
  expect(writes[writes.length - 1]?.method).toBe('DELETE')
})

test('creates an image command and previews the image it shows on stream', async ({ page }) => {
  const { writes } = await boot(page)

  await page.getByRole('button', { name: 'Random dog' }).click()
  const dialog = page.getByRole('dialog')
  await expect(dialog.getByLabel('Image on stream')).toHaveValue(/urlfetch https:\/\/dog\.ceo/)
  await expect(dialog.getByRole('img', { name: 'Image preview' })).toHaveAttribute('src', DOG)
  await expect(dialog.getByText('A dog for ExampleViewer!')).toBeVisible()
  await dialog.getByRole('button', { name: 'Save' }).click()
  await expect(dialog).toBeHidden()

  expect(writes[0]?.body).toEqual(
    expect.objectContaining({ trigger: '!dog', image_url: expect.stringContaining('urlfetch') })
  )
  const row = page.getByRole('listitem').filter({ hasText: '!dog' })
  await expect(row.getByText('Image', { exact: true })).toBeVisible()
})

test('has no horizontal scroll at phone width, with the editor open', async ({ page }) => {
  await page.setViewportSize({ width: 375, height: 667 })
  await boot(page)
  const overflow = () =>
    page.evaluate(() => document.documentElement.scrollWidth > window.innerWidth)
  expect(await overflow()).toBe(false)
  await page.getByRole('button', { name: 'New command' }).click()
  await expect(page.getByRole('dialog')).toBeVisible()
  expect(await overflow()).toBe(false)
})
