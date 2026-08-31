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
    expect(t('settings.ambassador.toastFeatured')).toBe('You will now appear on the homepage')
    expect(t('settings.ambassador.toastUnfeatured')).toBe('Removed from the homepage showcase')
    expect(t('settings.ambassador.toastFailed')).toBe('Failed to update showcase setting')
  })
})

describe('settings index page copy', () => {
  it('keeps the page heading and the profile card', () => {
    expect(t('settings.index.heading')).toBe('Settings')
    expect(t('settings.index.profileHeading')).toBe('Profile')
    expect(t('settings.index.usernameLabel')).toBe('Username')
    expect(t('settings.index.primaryPlatformLabel')).toBe('Primary Platform')
    expect(t('settings.index.primaryPlatformUnknown')).toBe('Unknown')
  })

  it('keeps the setup guide card', () => {
    expect(t('settings.index.setupGuideHeading')).toBe('Setup guide')
    expect(t('settings.index.setupGuideBody')).toBe(
      'Walk through overlay setup again: create an overlay, connect chat, pick a theme, and get the OBS link.'
    )
    expect(t('settings.index.setupGuideRestart')).toBe('Restart')
    expect(t('settings.index.setupGuideRestarting')).toBe('Restarting\u2026')
  })

  it('keeps the premium, devices and token cards', () => {
    expect(t('settings.index.premiumHeading')).toBe('Premium')
    expect(t('settings.index.premiumBody')).toBe(
      'Unlock premium features by backing All-Chat on Patreon.'
    )
    expect(t('settings.index.premiumManage')).toBe('Manage Premium')
    expect(t('settings.index.devicesHeading')).toBe('Paired devices')
    expect(t('settings.index.devicesBody')).toBe(
      'See and revoke the Stream Deck and StreamController control surfaces linked to your account. Each one is locked to a single overlay.'
    )
    expect(t('settings.index.devicesManage')).toBe('Manage devices')
    expect(t('settings.index.tokensHeading')).toBe('API tokens')
    expect(t('settings.index.tokensBody')).toBe(
      'Create and revoke personal access tokens. Use these where linking cannot reach \u2014 a headless capture box, a second PC, or a script.'
    )
    expect(t('settings.index.tokensManage')).toBe('Manage tokens')
  })

  it('keeps the data and privacy card', () => {
    // The render site spelled the ampersand &amp;. A catalog string is not
    // HTML, so it is the character.
    expect(t('settings.index.privacyHeading')).toBe('Data & Privacy')
    expect(t('settings.index.privacyBody')).toBe(
      'We keep data collection minimal and transparent. Review the policies below for details about how tokens, overlays, and chat metadata are processed.'
    )
    expect(t('settings.index.privacyPolicyLink')).toBe('Privacy Policy')
    expect(t('settings.index.termsLink')).toBe('Terms of Service')
  })

  it('keeps the Discord servers card', () => {
    expect(t('settings.index.discordHeading')).toBe('Discord')
    expect(t('settings.index.discordServersLoading')).toBe('Loading Discord servers')
    expect(t('settings.index.discordNoServer')).toBe('No Discord server connected.')
    expect(t('settings.index.discordConnectServer')).toBe('Connect Discord Server')
    expect(t('settings.index.discordConnectAnother')).toBe('Connect another server')
    expect(t('settings.index.discordDisconnect')).toBe('Disconnect')
    expect(t('settings.index.discordDisconnectTitle', { guild: 'My Server' })).toBe(
      'Disconnect My Server?'
    )
    expect(t('settings.index.discordDisconnectBody', { guild: 'My Server' })).toBe(
      'This will remove all Discord sources connected to My Server.'
    )
    expect(t('settings.index.discordDisconnectCancel')).toBe('Cancel')
    expect(t('settings.index.discordDisconnectConfirm')).toBe('Yes, disconnect')
  })

  it('keeps the Discord account link card', () => {
    expect(t('settings.index.discordAccountHeading')).toBe('Your Discord account')
    expect(t('settings.index.discordAccountLoading')).toBe('Loading your Discord account link')
    expect(t('settings.index.discordAccountLinked', { username: 'someone' })).toBe(
      'Linked as someone. Needed so moderators can act on your Discord servers.'
    )
    // The fallback the render site substituted when the username is unknown.
    expect(t('settings.index.discordAccountFallbackName')).toBe('your Discord account')
    expect(t('settings.index.discordAccountUnlinked')).toBe(
      'Not linked. Link it to let moderators you invite act on your Discord servers.'
    )
    expect(t('settings.index.discordAccountUnlink')).toBe('Unlink')
    expect(t('settings.index.discordAccountLink')).toBe('Link Discord account')
  })

  it('keeps the danger zone', () => {
    expect(t('settings.index.dangerHeading')).toBe('Danger Zone')
    expect(t('settings.index.dangerBody')).toBe(
      'Deleting your account removes all overlays, OAuth grants, and cached chat sources. This action is permanent and cannot be undone.'
    )
    expect(t('settings.index.deleteAccount')).toBe('Delete Account')
    expect(t('settings.index.deleteConfirmTitle')).toBe('Delete your account?')
    expect(t('settings.index.deleteConfirmBody')).toBe(
      'This permanently deletes your account and all overlays. This action cannot be undone.'
    )
    expect(t('settings.index.deleteCancel')).toBe('Cancel')
    expect(t('settings.index.deleteConfirm')).toBe('Yes, delete my account')
  })
})
