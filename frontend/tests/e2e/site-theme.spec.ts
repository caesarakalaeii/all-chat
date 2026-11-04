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
 * The site light/dark theme (ADR-0087): the toggle in the nav switches the
 * whole site, the choice survives a reload and client-side navigation, and
 * the OBS overlay routes and the stream mockups never take it.
 */

const json = (body: unknown, status = 200) => ({
  status,
  contentType: 'application/json',
  body: JSON.stringify(body),
})

async function boot(page: Page) {
  await page.route('**/api/v1/**', (route) => route.fulfill(json({}, 404)))
  await page.addInitScript(() => localStorage.setItem('cookieBannerAcknowledged', 'true'))
}

const theme = (page: Page) => page.evaluate(() => document.documentElement.dataset.theme ?? 'dark')
const background = (page: Page, selector: string) =>
  page
    .locator(selector)
    .first()
    .evaluate((el) => getComputedStyle(el).backgroundColor)

test('the toggle on the terms page lights the whole site and is remembered', async ({ page }) => {
  await boot(page)
  await page.goto('/legal/terms')
  const toggle = page.getByRole('button', { name: 'Light mode' })
  await expect(toggle).toHaveAttribute('aria-pressed', 'false', { timeout: 20_000 })
  const darkPanel = await background(page, '.lanes-panel')

  await toggle.click()
  await expect(toggle).toHaveAttribute('aria-pressed', 'true')
  expect(await theme(page)).toBe('light')
  // The legal panel is lanes art direction: black in dark, white in light.
  expect(darkPanel).toBe('rgb(0, 0, 0)')
  await expect.poll(() => background(page, '.lanes-panel')).toBe('rgb(255, 255, 255)')

  await page.reload()
  await expect(page.getByRole('button', { name: 'Light mode' })).toHaveAttribute(
    'aria-pressed',
    'true'
  )
  expect(await theme(page)).toBe('light')

  // Client-side navigation keeps it.
  await page.getByRole('link', { name: 'Docs' }).first().click()
  await expect(page).toHaveURL(/\/docs/)
  expect(await theme(page)).toBe('light')

  await page.getByRole('button', { name: 'Light mode' }).click()
  expect(await theme(page)).toBe('dark')
})

test('an OBS overlay never takes the light theme', async ({ page }) => {
  await boot(page)
  await page.addInitScript(() => localStorage.setItem('allchat_theme', 'light'))
  await page.goto('/overlay/00000000-0000-4000-8000-000000000001')
  await page.waitForLoadState('domcontentloaded')
  expect(await theme(page)).toBe('dark')
})

test('stream mockups stay dark on a light homepage', async ({ page }) => {
  await boot(page)
  await page.addInitScript(() => localStorage.setItem('allchat_theme', 'light'))
  await page.goto('/')
  await expect.poll(() => theme(page), { timeout: 20_000 }).toBe('light')

  // The page itself is light...
  await expect.poll(() => background(page, '.lanes-home')).toBe('rgb(244, 243, 239)')
  // ...but the hero stage and the live-preview mockup keep their dark fill.
  expect(await background(page, '.lanes-stage')).toBe('rgb(5, 5, 8)')
  const frame = page.locator('.frame').first()
  await frame.scrollIntoViewIfNeeded()
  expect(
    await frame.evaluate((el) => getComputedStyle(el).getPropertyValue('--lanes-page').trim())
  ).toBe('#050508')
})
