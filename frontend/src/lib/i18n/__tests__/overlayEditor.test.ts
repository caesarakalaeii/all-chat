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
 * Copy lock for the overlay editor surface. See __tests__/dashboard.test.ts for
 * why the copy is pinned here rather than through a rendered-output diff.
 */

import { describe, expect, it } from 'vitest'

import { getTranslations } from '@/lib/i18n'

const t = getTranslations()

describe('theme marketplace copy', () => {
  it('keeps the two marketplace titles as whole sentences', () => {
    // The render site built these as `{cond ? 'Credit Roll' : ''} Theme
    // Marketplace`, which is the concatenation the catalog exists to remove: a
    // language that puts the qualifier after the noun cannot reassemble it.
    expect(t('overlayEditor.themeMarketplace.title')).toBe('Theme Marketplace')
    expect(t('overlayEditor.themeMarketplace.titleCreditRoll')).toBe(
      'Credit Roll Theme Marketplace'
    )
    expect(t('overlayEditor.themeMarketplace.description')).toBe(
      'Browse and apply custom CSS themes for your overlay'
    )
    expect(t('overlayEditor.themeMarketplace.descriptionCreditRoll')).toBe(
      'Browse and apply custom CSS themes for your credit roll'
    )
  })

  it('keeps the marketplace states', () => {
    expect(t('overlayEditor.themeMarketplace.loading')).toBe('Loading themes')
    expect(t('overlayEditor.themeMarketplace.loadingEllipsis')).toBe('Loading themes...')
    expect(t('overlayEditor.themeMarketplace.errorTitle')).toBe('Error Loading Themes')
    expect(t('overlayEditor.themeMarketplace.emptyTitle')).toBe('No themes found')
    expect(t('overlayEditor.themeMarketplace.emptyBody')).toBe('Try adjusting your filters')
  })

  it('keeps the result count as one sentence with both numbers in it', () => {
    // Was `Showing {filtered} of {total} themes` split across three text nodes.
    expect(t('overlayEditor.themeMarketplace.showingCount', { shown: 4, total: 12 })).toBe(
      'Showing 4 of 12 themes'
    )
  })

  it('keeps the marketplace controls', () => {
    expect(t('overlayEditor.themeMarketplace.applyTheme')).toBe('Apply Theme')
    expect(t('overlayEditor.themeMarketplace.clearFilters')).toBe('Clear Filters')
    expect(t('overlayEditor.themeMarketplace.searchLabel')).toBe('Search themes')
    expect(t('overlayEditor.themeMarketplace.searchPlaceholder')).toBe('Search themes...')
    expect(t('overlayEditor.themeMarketplace.sync')).toBe('Sync')
    expect(t('overlayEditor.themeMarketplace.syncTitleInline')).toBe(
      'Force refresh themes from GitHub (Admin)'
    )
    expect(t('overlayEditor.themeMarketplace.syncLabel')).toBe('Force refresh themes from GitHub')
    expect(t('overlayEditor.themeMarketplace.syncTitle')).toBe('Force refresh themes (Admin)')
    expect(t('overlayEditor.themeMarketplace.closeLabel')).toBe('Close theme marketplace')
  })
})

describe('credit roll preview copy', () => {
  it('keeps the sample credits the preview renders', () => {
    expect(t('overlayEditor.creditRollPreview.heading')).toBe('🎬 Stream Credits')
    expect(t('overlayEditor.creditRollPreview.subheading')).toBe('Thank you for your support!')
    expect(t('overlayEditor.creditRollPreview.leaderboardHeading')).toBe('Top Subscribers')
    expect(t('overlayEditor.creditRollPreview.footerHeading')).toBe('Thank you! ❤️')
    expect(t('overlayEditor.creditRollPreview.footerBody')).toBe('See you next stream!')
  })
})
