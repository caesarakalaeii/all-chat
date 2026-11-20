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

import { expect, test } from '@playwright/test'

// The homepage language switch (ADR-0078) end to end: the catalog is fetched
// on demand, the page re-renders in it, and a reload restores the choice.
for (const [code, headline] of [
  ['fr', 'Tous les chats.'],
  ['es', 'Todos los chats.'],
] as const) {
  test(`the homepage switches to ${code} and remembers it`, async ({ page }) => {
    await page.goto('/')
    const picker = page.getByRole('combobox', { name: 'Language' })
    await picker.selectOption(code)

    await expect(page.locator('html')).toHaveAttribute('lang', code)
    await expect(page.getByRole('heading', { level: 1 })).toContainText(headline, {
      ignoreCase: true,
    })

    await page.reload()
    await expect(page.locator('html')).toHaveAttribute('lang', code)
    await expect(page.locator('select.lanes-lang')).toHaveValue(code)
  })
}
