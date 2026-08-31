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
 * Copy lock for the settings surface. See __tests__/dashboard.test.ts for why
 * the copy is pinned here rather than through a rendered-output diff.
 */

import { describe, expect, it } from 'vitest'

import { getTranslations } from '@/lib/i18n'

const t = getTranslations()

describe('ambassador settings copy', () => {
  it('keeps the card copy, typographic apostrophe and quotes included', () => {
    // The render site spelled these &rsquo;/&ldquo;/&rdquo;. A catalog string is
    // not HTML, so they are the characters themselves.
    expect(t('settings.ambassador.heading')).toBe('Ambassador')
    expect(t('settings.ambassador.body')).toBe(
      'You\u2019re an All-Chat ambassador. Choose whether to be featured on the public homepage.'
    )
    expect(t('settings.ambassador.featureToggle')).toBe('Feature me on the homepage')
    expect(t('settings.ambassador.cardReads', { tagline: 'Streams every Friday' })).toBe(
      'Your card reads: \u201cStreams every Friday\u201d'
    )
  })

  it('keeps the showcase toggle toasts', () => {
    expect(t('settings.ambassador.toasts.featured')).toBe('You will now appear on the homepage')
    expect(t('settings.ambassador.toasts.unfeatured')).toBe('Removed from the homepage showcase')
    expect(t('settings.ambassador.toasts.failed')).toBe('Failed to update showcase setting')
  })
})
