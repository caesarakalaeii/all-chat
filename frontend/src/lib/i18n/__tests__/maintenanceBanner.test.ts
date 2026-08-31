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
 * Copy lock for the maintenance announcements. See __tests__/dashboard.test.ts
 * for why the copy is pinned here rather than through a rendered-output diff.
 */

import { describe, expect, it } from 'vitest'

import { getTranslations } from '@/lib/i18n'

const t = getTranslations()

describe('maintenance info popover copy', () => {
  it('keeps the state headings, which carry no trailing colon', () => {
    expect(t('maintenanceBanner.popoverActiveHeading')).toBe('Maintenance in progress')
    expect(t('maintenanceBanner.popoverScheduledHeading')).toBe('Scheduled maintenance')
  })

  it('keeps the two window-time sentences whole', () => {
    expect(t('maintenanceBanner.popoverExpectedCompletion', { endsAt: 'Mar 3, 09:00' })).toBe(
      'Expected completion: Mar 3, 09:00'
    )
    expect(
      t('maintenanceBanner.popoverRange', { startsAt: 'Mar 3, 08:00', endsAt: 'Mar 3, 09:00' })
    ).toBe('Mar 3, 08:00 to Mar 3, 09:00')
  })
})
