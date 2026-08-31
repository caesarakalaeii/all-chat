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

describe('editor chrome copy', () => {
  it('keeps the advanced disclosure count inside one key', () => {
    // Was `Advanced ({count})` — a text node, an expression and a closing
    // parenthesis. The parentheses are punctuation a language may drop.
    expect(t('overlayEditor.nav.advancedCount', { count: 4 })).toBe('Advanced (4)')
  })

  it('keeps the editor navigation and preview labels', () => {
    expect(t('overlayEditor.nav.settingsLabel')).toBe('Overlay settings')
    expect(t('overlayEditor.previewBackdrop.heading')).toBe('Backdrop')
    expect(t('overlayEditor.previewBackdrop.appBackground')).toBe('Preview on app background')
    expect(t('overlayEditor.previewBackdrop.lightBackground')).toBe('Preview on light background')
    expect(t('overlayEditor.previewBackdrop.chromaGreen')).toBe('Preview on chroma green')
    expect(t('overlayEditor.previewBackdrop.customColor')).toBe('Custom preview background color')
  })

  it('keeps the settings search copy, typographic quotes included', () => {
    expect(t('overlayEditor.settingsSearch.label')).toBe('Search settings')
    expect(t('overlayEditor.settingsSearch.placeholder')).toBe(
      'Search settings\u2026 (e.g. badge, fade, banned words)'
    )
    expect(t('overlayEditor.settingsSearch.clearLabel')).toBe('Clear search')
    expect(t('overlayEditor.settingsSearch.resultsLabel')).toBe('Matching settings')
    expect(t('overlayEditor.settingsSearch.noResults', { query: 'badge' })).toBe(
      'No settings match \u201cbadge\u201d'
    )
  })
})

describe('appearance control copy', () => {
  it('keeps the background group labels', () => {
    expect(t('overlayEditor.appearance.background.overlayHeading')).toBe('Overlay background')
    expect(t('overlayEditor.appearance.background.overlayColor')).toBe('Overlay background')
    expect(t('overlayEditor.appearance.background.bubbleHeading')).toBe('Bubble background')
    expect(t('overlayEditor.appearance.background.bubbleColor')).toBe('Bubble background')
    expect(t('overlayEditor.appearance.background.borderColor')).toBe('Border color')
    expect(t('overlayEditor.appearance.background.borderRadius')).toBe('Border radius')
    expect(t('overlayEditor.appearance.background.borderWidth')).toBe('Border width')
    expect(t('overlayEditor.appearance.background.padding')).toBe('Padding')
    expect(t('overlayEditor.appearance.background.messageGap')).toBe('Message gap')
    expect(t('overlayEditor.appearance.background.backdropBlur')).toBe('Backdrop blur')
  })

  it('keeps the colors group labels', () => {
    expect(t('overlayEditor.appearance.colors.message')).toBe('Message color')
    expect(t('overlayEditor.appearance.colors.username')).toBe('Username color')
    expect(t('overlayEditor.appearance.colors.timestamp')).toBe('Timestamp color')
  })

  it('keeps the sizing group labels and the emote-scale caveat', () => {
    expect(t('overlayEditor.appearance.sizing.avatarSize')).toBe('Avatar size')
    expect(t('overlayEditor.appearance.sizing.badgeSize')).toBe('Badge size')
    expect(t('overlayEditor.appearance.sizing.emoteScale')).toBe('Emote scale')
    expect(t('overlayEditor.appearance.sizing.emoteScaleNote')).toBe(
      'Emote scale applies to third-party emotes (7TV, BTTV, FFZ). Standard emoji are not affected.'
    )
  })

  it('keeps the per-event size modifier label', () => {
    expect(t('overlayEditor.appearance.events.sizeModifier')).toBe('Size modifier')
  })

  it('names the colour picker controls after the swatch they belong to', () => {
    // All three took the control's own `label` prop, so the placeholder carries
    // it through rather than the catalog holding one key per swatch.
    expect(t('overlayEditor.appearance.colorPicker.swatchLabel', { label: 'Border color' })).toBe(
      'Pick color for Border color'
    )
    expect(t('overlayEditor.appearance.colorPicker.popoverTitle', { label: 'Border color' })).toBe(
      'Color for Border color'
    )
    expect(t('overlayEditor.appearance.colorPicker.hexLabel', { label: 'Border color' })).toBe(
      'Hex value for Border color'
    )
  })

  it('keeps the font picker labels and group headings', () => {
    expect(t('overlayEditor.appearance.fontPicker.openLabel')).toBe('Open font picker')
    expect(t('overlayEditor.appearance.fontPicker.empty')).toBe('No fonts found')
    expect(t('overlayEditor.appearance.fontPicker.systemGroup')).toBe('System Fonts')
    expect(t('overlayEditor.appearance.fontPicker.googleGroup')).toBe('Google Fonts')
  })
})
