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
    expect(t('overlayEditor.background.overlayHeading')).toBe('Overlay background')
    expect(t('overlayEditor.background.overlayColor')).toBe('Overlay background')
    expect(t('overlayEditor.background.bubbleHeading')).toBe('Bubble background')
    expect(t('overlayEditor.background.bubbleColor')).toBe('Bubble background')
    expect(t('overlayEditor.background.borderColor')).toBe('Border color')
    expect(t('overlayEditor.background.borderRadius')).toBe('Border radius')
    expect(t('overlayEditor.background.borderWidth')).toBe('Border width')
    expect(t('overlayEditor.background.padding')).toBe('Padding')
    expect(t('overlayEditor.background.messageGap')).toBe('Message gap')
    expect(t('overlayEditor.background.backdropBlur')).toBe('Backdrop blur')
  })

  it('keeps the colors group labels', () => {
    expect(t('overlayEditor.colors.message')).toBe('Message color')
    expect(t('overlayEditor.colors.username')).toBe('Username color')
    expect(t('overlayEditor.colors.timestamp')).toBe('Timestamp color')
  })

  it('keeps the sizing group labels and the emote-scale caveat', () => {
    expect(t('overlayEditor.sizing.avatarSize')).toBe('Avatar size')
    expect(t('overlayEditor.sizing.badgeSize')).toBe('Badge size')
    expect(t('overlayEditor.sizing.emoteScale')).toBe('Emote scale')
    expect(t('overlayEditor.sizing.emoteScaleNote')).toBe(
      'Emote scale applies to third-party emotes (7TV, BTTV, FFZ). Standard emoji are not affected.'
    )
  })

  it('keeps the per-event size modifier label', () => {
    expect(t('overlayEditor.events.sizeModifier')).toBe('Size modifier')
  })

  it('names the colour picker controls after the swatch they belong to', () => {
    // All three took the control's own `label` prop, so the placeholder carries
    // it through rather than the catalog holding one key per swatch.
    expect(t('overlayEditor.colorPicker.swatchLabel', { label: 'Border color' })).toBe(
      'Pick color for Border color'
    )
    expect(t('overlayEditor.colorPicker.popoverTitle', { label: 'Border color' })).toBe(
      'Color for Border color'
    )
    expect(t('overlayEditor.colorPicker.hexLabel', { label: 'Border color' })).toBe(
      'Hex value for Border color'
    )
  })

  it('keeps the font picker labels and group headings', () => {
    expect(t('overlayEditor.fontPicker.openLabel')).toBe('Open font picker')
    expect(t('overlayEditor.fontPicker.empty')).toBe('No fonts found')
    expect(t('overlayEditor.fontPicker.systemGroup')).toBe('System Fonts')
    expect(t('overlayEditor.fontPicker.googleGroup')).toBe('Google Fonts')
  })
})

describe('typography and visibility group copy', () => {
  it('keeps the font family field labels', () => {
    // Label and aria-label were byte-identical at every one of the three
    // sites, so they share one key each rather than doubling up.
    expect(t('overlayEditor.typography.bodyFont')).toBe('Body Font')
    expect(t('overlayEditor.typography.usernameFont')).toBe('Username Font')
    expect(t('overlayEditor.typography.timestampFont')).toBe('Timestamp Font')
  })

  it('keeps the font picker placeholder and default accessible name', () => {
    expect(t('overlayEditor.fontPicker.placeholder')).toBe('Select font…')
    expect(t('overlayEditor.fontPicker.defaultLabel')).toBe('Font family')
  })

  it('keeps the font weight options in numeric order', () => {
    expect(t('overlayEditor.typography.fontWeight')).toBe('Font Weight')
    expect(t('overlayEditor.typography.fontWeightPlaceholder')).toBe('Select weight…')
    expect(t('overlayEditor.typography.fontWeight100')).toBe('100 Thin')
    expect(t('overlayEditor.typography.fontWeight300')).toBe('300 Light')
    expect(t('overlayEditor.typography.fontWeight400')).toBe('400 Regular')
    expect(t('overlayEditor.typography.fontWeight500')).toBe('500 Medium')
    expect(t('overlayEditor.typography.fontWeight600')).toBe('600 SemiBold')
    expect(t('overlayEditor.typography.fontWeight700')).toBe('700 Bold')
    expect(t('overlayEditor.typography.fontWeight800')).toBe('800 ExtraBold')
    expect(t('overlayEditor.typography.fontWeight900')).toBe('900 Black')
  })

  it('keeps the font size fields and their pixel unit', () => {
    expect(t('overlayEditor.typography.bodySize')).toBe('Body Size')
    expect(t('overlayEditor.typography.usernameSize')).toBe('Username Size')
    expect(t('overlayEditor.typography.timestampSize')).toBe('Timestamp Size')
    // Rendered as the accessible description beside each number input, so it is
    // read out and therefore copy, not a CSS unit token.
    expect(t('overlayEditor.typography.pixelUnit')).toBe('px')
  })

  it('keeps the text shadow presets', () => {
    expect(t('overlayEditor.typography.textShadow')).toBe('Text Shadow')
    expect(t('overlayEditor.typography.textShadowNone')).toBe('None (default)')
    expect(t('overlayEditor.typography.textShadowSoft')).toBe('Soft shadow')
    expect(t('overlayEditor.typography.textShadowStrong')).toBe('Strong shadow')
    expect(t('overlayEditor.typography.textShadowOutline')).toBe('Outline')
    expect(t('overlayEditor.typography.textShadowCustom')).toBe('Custom')
    expect(t('overlayEditor.typography.textShadowNote')).toBe(
      'Keeps chat readable over bright gameplay. Try it with a light preview backdrop.'
    )
  })

  it('keeps the advanced typography sliders', () => {
    expect(t('overlayEditor.typography.lineHeight')).toBe('Line Height')
    expect(t('overlayEditor.typography.letterSpacing')).toBe('Letter Spacing')
  })

  it('keeps the visibility toggles', () => {
    expect(t('overlayEditor.visibility.showAvatars')).toBe('Show avatars')
    expect(t('overlayEditor.visibility.showBadges')).toBe('Show badges')
    expect(t('overlayEditor.visibility.showTimestamps')).toBe('Show timestamps')
    expect(t('overlayEditor.visibility.showEmotes')).toBe('Show emotes')
    expect(t('overlayEditor.visibility.showUsername')).toBe('Show username')
    expect(t('overlayEditor.visibility.showPlatformBadge')).toBe('Show platform badge')
    expect(t('overlayEditor.visibility.showPlatformIndicators')).toBe('Show platform indicators')
    expect(t('overlayEditor.visibility.showPronouns')).toBe('Show pronouns')
  })

  it('keeps the badge and pronoun placement options', () => {
    expect(t('overlayEditor.visibility.position')).toBe('Position')
    expect(t('overlayEditor.visibility.style')).toBe('Style')
    expect(t('overlayEditor.visibility.beforeUsername')).toBe('Before username')
    expect(t('overlayEditor.visibility.afterUsername')).toBe('After username')
    expect(t('overlayEditor.visibility.styleText')).toBe('Text')
    expect(t('overlayEditor.visibility.styleIcon')).toBe('Icon')
    expect(t('overlayEditor.visibility.pronounPillColor')).toBe('Pill color')
  })

  it('keeps the event visibility rows', () => {
    expect(t('overlayEditor.events.showSuperChat')).toBe('Super Chat')
    expect(t('overlayEditor.events.showSubscriptions')).toBe('Subscriptions')
    expect(t('overlayEditor.events.showRaids')).toBe('Raids')
    expect(t('overlayEditor.events.showBits')).toBe('Bits')
    expect(t('overlayEditor.events.showMembershipGift')).toBe('Membership Gift')
  })
})
