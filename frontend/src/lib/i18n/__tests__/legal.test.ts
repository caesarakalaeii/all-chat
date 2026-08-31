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
 * Copy lock for the consent banner and the legal pages. See
 * __tests__/dashboard.test.ts for why the copy is pinned here rather than
 * through a rendered-output diff.
 *
 * This namespace is the one where byte-identical transcription matters beyond
 * review hygiene: these are the GDPR disclosures, and a reworded disclosure is
 * a different disclosure.
 */

import { describe, expect, it } from 'vitest'

import { getTranslations } from '@/lib/i18n'

const t = getTranslations()

describe('cookie banner chrome', () => {
  it('keeps the region and icon labels', () => {
    expect(t('legal.cookieBanner.regionLabel')).toBe('Cookie consent')
    expect(t('legal.cookieBanner.iconLabel')).toBe('Cookie')
  })

  it('keeps the heading and the two lead paragraphs whole', () => {
    expect(t('legal.cookieBanner.title')).toBe('Privacy & Data Storage')
    expect(t('legal.cookieBanner.storageBody')).toBe(
      'All-Chat uses {storage} to save your authentication tokens and keep you logged in. This is essential for the service to function properly.'
    )
    expect(t('legal.cookieBanner.storageEmphasis')).toBe('browser local storage')
    expect(t('legal.cookieBanner.noTrackingBody')).toBe(
      '{emphasis} We do not share your data with third parties for advertising purposes.'
    )
    expect(t('legal.cookieBanner.noTrackingEmphasis')).toBe(
      'We use no tracking cookies, and our usage analytics are cookieless and store no personal data.'
    )
  })

  it('keeps the actions and the footer', () => {
    expect(t('legal.cookieBanner.acknowledge')).toBe('I Understand')
    expect(t('legal.cookieBanner.learnMore')).toBe('Learn More')
    expect(t('legal.cookieBanner.footer')).toBe(
      'Your data is stored locally in your browser and transmitted securely via HTTPS'
    )
  })
})

describe('cookie banner disclosure list', () => {
  it('keeps the summary', () => {
    expect(t('legal.cookieBanner.detailsSummary')).toBe('What data do we store locally?')
  })

  it('keeps each stored-data row whole', () => {
    // The bolded label opens each row and the rest of the row continues the
    // sentence, so the row is one string with a {label} placeholder. Splitting
    // at the hyphen would strand a fragment a translator cannot reorder.
    expect(t('legal.cookieBanner.tokensRow')).toBe(
      '{label} - Required to keep you logged in and authenticate with streaming platforms (Twitch, YouTube, TikTok, Kick)'
    )
    expect(t('legal.cookieBanner.tokensLabel')).toBe('Authentication tokens')
    expect(t('legal.cookieBanner.preferencesRow')).toBe(
      '{label} - Your overlay configurations and settings'
    )
    expect(t('legal.cookieBanner.preferencesLabel')).toBe('User preferences')
    expect(t('legal.cookieBanner.noTrackingRow')).toBe(
      "{label} - We don't track your browsing behavior"
    )
    expect(t('legal.cookieBanner.noTrackingLabel')).toBe('No tracking cookies')
    expect(t('legal.cookieBanner.noAdsRow')).toBe(
      "{label} - We don't serve ads or share data with advertisers"
    )
    expect(t('legal.cookieBanner.noAdsLabel')).toBe('No advertising cookies')
    expect(t('legal.cookieBanner.analyticsRow')).toBe(
      '{label} - We measure aggregate usage with self-hosted Umami. It sets no cookies, stores no personal identifier, and does not track public overlays'
    )
    expect(t('legal.cookieBanner.analyticsLabel')).toBe('Cookieless analytics')
  })

  it('keeps the fonts and third-party notes whole', () => {
    expect(t('legal.cookieBanner.fontsNote')).toBe(
      '{label} All fonts (including ones originally distributed by Google Fonts) are self-hosted on our infrastructure – your IP address is never sent to Google.'
    )
    expect(t('legal.cookieBanner.fontsLabel')).toBe('Fonts:')
    // Four emphasised runs in one sentence, one of them a link, so the sentence
    // stays whole and each run is its own placeholder.
    expect(t('legal.cookieBanner.thirdPartyNote')).toBe(
      '{label} Dashboard pages may load fallback avatars from {avatars} and themes from the {github}. These requests transmit your IP address to the respective providers. See our {privacy} for details.'
    )
    expect(t('legal.cookieBanner.thirdPartyLabel')).toBe('Third-party resources:')
    expect(t('legal.cookieBanner.thirdPartyAvatars')).toBe('UI Avatars')
    expect(t('legal.cookieBanner.thirdPartyGithub')).toBe('GitHub API')
  })

  it('keeps the agreement paragraph whole', () => {
    expect(t('legal.cookieBanner.agreement')).toBe(
      'By using All-Chat, you agree to this essential data storage. For more details, please read our {privacy} and {terms}.'
    )
  })

  it('keeps the two legal document names', () => {
    // Referenced from the banner in three places and from the landing footer,
    // but the footer owns its own copies under marketing.footer.* because the
    // banner's are prose links inside a sentence, not nav labels.
    expect(t('legal.cookieBanner.privacyPolicy')).toBe('Privacy Policy')
    expect(t('legal.cookieBanner.termsOfService')).toBe('Terms of Service')
  })
})
