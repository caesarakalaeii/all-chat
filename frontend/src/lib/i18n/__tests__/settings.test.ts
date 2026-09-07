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

describe('viewer identity page copy', () => {
  it('keeps the unauthenticated sign-in card', () => {
    expect(t('settings.viewer.heading')).toBe('Viewer Identity')
    expect(t('settings.viewer.subheading')).toBe(
      'Customize how your name appears across all overlays'
    )
    expect(t('settings.viewer.signInHeading')).toBe('Sign in to manage your viewer identity')
    expect(t('settings.viewer.signInBody')).toBe(
      'Connect your streaming platform account to set a custom name color and manage your viewer identity.'
    )
    // One key per platform rather than a {platform} placeholder: the button copy
    // is per-platform in the source and a placeholder would change nothing here
    // while hiding which buttons exist.
    expect(t('settings.viewer.signInTwitch')).toBe('Sign in with Twitch')
    expect(t('settings.viewer.signInYoutube')).toBe('Sign in with YouTube')
    expect(t('settings.viewer.signInKick')).toBe('Sign in with Kick')
  })

  it('keeps the profile summary', () => {
    expect(t('settings.viewer.profileHeading')).toBe('Profile')
    // Shown in place of a missing display name and username.
    expect(t('settings.viewer.viewerFallbackName')).toBe('Viewer')
  })

  it('keeps the name colour card', () => {
    expect(t('settings.viewer.nameColorHeading')).toBe('Name Color')
    expect(t('settings.viewer.nameColorBody')).toBe(
      'Set a custom color or gradient for your name on overlays'
    )
    expect(t('settings.viewer.solidTab')).toBe('Solid Color')
    expect(t('settings.viewer.gradientTab')).toBe('Gradient')
    expect(t('settings.viewer.premiumPill')).toBe('Premium')
    expect(t('settings.viewer.gradientUpsell')).toBe(
      'Gradient names are a viewer premium cosmetic.'
    )
    expect(t('settings.viewer.unlockPremium')).toBe('Unlock viewer premium')
    expect(t('settings.viewer.colorPickerLabel')).toBe('Name color picker')
    expect(t('settings.viewer.colorHexLabel')).toBe('Name color hex value')
    expect(t('settings.viewer.savedFeedback')).toBe('Saved ✓')
    expect(t('settings.viewer.autoSaveNote')).toBe('Changes save automatically')
    expect(t('settings.viewer.previewLabel')).toBe('Preview')
    expect(t('settings.viewer.previewMessage')).toBe('Hello world!')
  })

  it('keeps the gradient editor controls', () => {
    expect(t('settings.viewer.removeStopLabel', { index: '2' })).toBe('Remove stop 2')
    expect(t('settings.viewer.addStop')).toBe('+ Add stop')
    expect(t('settings.viewer.angleLabel')).toBe('Angle')
    expect(t('settings.viewer.angleDegreesLabel')).toBe('Angle in degrees')
    expect(t('settings.viewer.saveGradient')).toBe('Save gradient')
    expect(t('settings.viewer.savingGradient')).toBe('Saving…')
  })

  it('keeps the avatar cosmetics card', () => {
    expect(t('settings.viewer.cosmeticsHeading')).toBe('Avatar Cosmetics')
    expect(t('settings.viewer.cosmeticsBody')).toBe('Choose a frame and flair for your avatar')
    expect(t('settings.viewer.cosmeticsUpsell')).toBe('Some frames and flairs are viewer premium.')
    expect(t('settings.viewer.frameHeading')).toBe('Avatar Frame')
    expect(t('settings.viewer.flairHeading')).toBe('Avatar Flair')
    // The catalog entry standing for "no frame" / "no flair".
    expect(t('settings.viewer.noneItem')).toBe('None')
    expect(t('settings.viewer.save')).toBe('Save')
    expect(t('settings.viewer.saving')).toBe('Saving…')
  })

  it('keeps the cosmetics save errors', () => {
    expect(t('settings.viewer.premiumRequired')).toBe('Premium required')
    expect(t('settings.viewer.saveFailed')).toBe('Save failed')
  })

  it('keeps the linked platforms card', () => {
    expect(t('settings.viewer.linkedHeading')).toBe('Linked Platforms')
    expect(t('settings.viewer.linkedBody')).toBe(
      'Connect additional platforms to share your cosmetics across all your chats'
    )
    expect(t('settings.viewer.connected')).toBe('Connected')
    expect(t('settings.viewer.connect')).toBe('Connect')
    expect(t('settings.viewer.connecting')).toBe('Connecting…')
    expect(t('settings.viewer.disconnect')).toBe('Disconnect')
    expect(t('settings.viewer.disconnecting')).toBe('Disconnecting…')
    expect(t('settings.viewer.loadLinkedFailed')).toBe('Could not load linked platforms')
    expect(t('settings.viewer.disconnectFailed')).toBe('Failed to disconnect platform')
  })
})

describe('API tokens page copy', () => {
  it('keeps the page chrome', () => {
    expect(t('settings.apiTokens.heading')).toBe('API Tokens')
    expect(t('settings.apiTokens.subheading')).toBe(
      'Personal access tokens for the Stream Deck and StreamController plugins.'
    )
    expect(t('settings.apiTokens.listHeading')).toBe('Your tokens')
    expect(t('settings.apiTokens.listBody')).toBe(
      'Only the details below are stored — the token itself is kept as a hash and can never be shown again.'
    )
    expect(t('settings.apiTokens.loadFailed')).toBe(
      'Could not load your tokens. Refresh the page to try again.'
    )
  })

  it('keeps the minted token reveal, the one-shot secret warning included', () => {
    expect(t('settings.apiTokens.revealRegionLabel', { name: 'Studio PC' })).toBe(
      'New token Studio PC'
    )
    expect(t('settings.apiTokens.revealHeading')).toBe('Copy your new token now')
    // The token name is emphasised mid-sentence, so the sentence stays whole and
    // the emphasised run is its own key. See settings.viewer for the convention.
    expect(t('settings.apiTokens.revealWarning', { name: 'Studio PC' })).toBe(
      'This is the only time Studio PC will ever be shown. We store only a hash of it, so it cannot be displayed again — if you lose it, revoke this token and create a new one.'
    )
    expect(t('settings.apiTokens.copyToken')).toBe('Copy token')
    expect(t('settings.apiTokens.copied')).toBe('Copied ✓')
    expect(t('settings.apiTokens.copyFailed')).toBe(
      'Could not copy automatically — select the token and copy it manually.'
    )
    // The render site spelled the apostrophe &apos;, which is U+0027.
    expect(t('settings.apiTokens.dismissReveal')).toBe("I've saved it")
  })

  it('keeps the create form', () => {
    expect(t('settings.apiTokens.createHeading')).toBe('Create a token')
    expect(t('settings.apiTokens.createBody')).toBe(
      'Give the token a name you will recognise later, and grant it only what the device needs.'
    )
    expect(t('settings.apiTokens.nameLabel')).toBe('Token name')
    expect(t('settings.apiTokens.namePlaceholder')).toBe('Stream Deck (studio PC)')
    expect(t('settings.apiTokens.nameDescription')).toBe(
      'Shown in the list below so you know what to revoke.'
    )
    expect(t('settings.apiTokens.scopesLegend')).toBe('Scopes')
    expect(t('settings.apiTokens.noScopesWarning')).toBe(
      'Pick at least one scope — a token with none can authenticate but do nothing.'
    )
    expect(t('settings.apiTokens.create')).toBe('Create token')
    expect(t('settings.apiTokens.creating')).toBe('Creating…')
    expect(t('settings.apiTokens.createFailed')).toBe('Could not create the token. Try again.')
  })

  it('keeps the scope titles and descriptions, keyed by the wire scope name', () => {
    expect(t('settings.apiTokens.scopeChatWriteTitle')).toBe('Send chat messages')
    expect(t('settings.apiTokens.scopeChatWriteDescription')).toBe(
      'Lets the plugin post messages to your connected chats.'
    )
    expect(t('settings.apiTokens.scopeEngagementWriteTitle')).toBe('Run polls and predictions')
    expect(t('settings.apiTokens.scopeEngagementWriteDescription')).toBe(
      'Lets the plugin open, resolve and cancel polls and predictions.'
    )
  })

  it('keeps the empty state and its plugin links', () => {
    expect(t('settings.apiTokens.emptyHeading')).toBe("You don't have any API tokens yet")
    expect(t('settings.apiTokens.emptyBody')).toBe(
      "A personal access token lets a device sign in as you without your password — it's how the Stream Deck and StreamController plugins send chat messages and run polls and predictions on your behalf. Create one per device so you can revoke it on its own."
    )
    expect(t('settings.apiTokens.setupGuides')).toBe('Setup guides:')
    expect(t('settings.apiTokens.streamDeckReadme')).toBe('Stream Deck plugin README')
    expect(t('settings.apiTokens.streamControllerReadme')).toBe('StreamController plugin README')
  })

  it('keeps the token row', () => {
    // One key with two placeholders, not two fragments joined by the middle dot:
    // a second language reorders the two dates.
    expect(t('settings.apiTokens.tokenDates', { created: '1 Jan 2026', lastUsed: 'never' })).toBe(
      'Created 1 Jan 2026 · Last used never'
    )
    expect(t('settings.apiTokens.neverUsed')).toBe('never')
    // Stands in for a missing or unparseable timestamp.
    expect(t('settings.apiTokens.unknownDate')).toBe('—')
    expect(t('settings.apiTokens.revokeLabel', { name: 'Studio PC' })).toBe('Revoke Studio PC')
    expect(t('settings.apiTokens.revoke')).toBe('Revoke')
  })

  it('keeps the revoke confirmation', () => {
    expect(t('settings.apiTokens.revokeConfirmTitle')).toBe('Revoke this token?')
    expect(t('settings.apiTokens.revokeConfirmBody', { name: 'Studio PC' })).toBe(
      '“Studio PC” stops working immediately. Any device using it will need a new token.'
    )
    expect(t('settings.apiTokens.revokeCancel')).toBe('Cancel')
    expect(t('settings.apiTokens.revokeConfirm')).toBe('Revoke token')
    expect(t('settings.apiTokens.revoking')).toBe('Revoking…')
  })
})

describe('paired devices page copy', () => {
  it('keeps the page chrome', () => {
    expect(t('settings.devices.heading')).toBe('Paired devices')
    expect(t('settings.devices.subheading')).toBe(
      'Stream Deck and StreamController control surfaces linked to your account. Each one is locked to a single overlay and lapses on its own if it stops being used.'
    )
    expect(t('settings.devices.listHeading')).toBe('Your devices')
    expect(t('settings.devices.listBody')).toBe(
      'Only the details below are stored. The credential itself was sent straight to the plugin and is kept as a hash here — it is never shown in this dashboard, which is why there is nothing on this page to copy.'
    )
    expect(t('settings.devices.loadingLabel')).toBe('Loading paired devices')
    expect(t('settings.devices.loadFailed')).toBe(
      'Could not load your paired devices. Refresh the page to try again.'
    )
  })

  it('keeps the empty state, the plugin-side link instruction included', () => {
    expect(t('settings.devices.emptyHeading')).toBe('No paired devices yet')
    // "Link with All-Chat" is emphasised mid-sentence, so it stays a placeholder
    // inside one whole sentence rather than three concatenated fragments.
    expect(t('settings.devices.emptyBody', { linkAction: 'Link with All-Chat' })).toBe(
      'Linking starts in the plugin, not here: open your Stream Deck or StreamController settings and press Link with All-Chat. Your browser opens an approve screen, you pick an overlay, and the plugin receives its credential directly — nothing is copied or pasted.'
    )
    expect(t('settings.devices.emptyLinkAction')).toBe('Link with All-Chat')
    expect(t('settings.devices.setupGuide')).toBe('Setup guide')
    expect(t('settings.devices.havePairingCode')).toBe('I have a pairing code')
  })

  it('keeps the device row', () => {
    expect(t('settings.devices.controlsOverlay', { overlay: 'Main', status: 'Active' })).toBe(
      'Controls Main · Active'
    )
    expect(t('settings.devices.rowDates', { lastUsed: 'never', paired: '1 Jan 2026' })).toBe(
      'Last used never · Paired 1 Jan 2026'
    )
    expect(t('settings.devices.neverUsed')).toBe('never')
    expect(t('settings.devices.unknownDate')).toBe('—')
    expect(t('settings.devices.revokeLabel', { name: 'Deck' })).toBe('Revoke Deck')
    expect(t('settings.devices.revoke')).toBe('Revoke')
  })

  it('keeps the three device statuses', () => {
    expect(t('settings.devices.statusRevoked', { date: '1 Feb 2026' })).toBe('Revoked 1 Feb 2026')
    expect(t('settings.devices.statusExpired', { date: '1 Feb 2026' })).toBe('Expired 1 Feb 2026')
    expect(t('settings.devices.statusActive', { date: '1 Feb 2026' })).toBe(
      'Active until 1 Feb 2026'
    )
  })

  it('keeps the headless-machine card and its two links', () => {
    expect(t('settings.devices.headlessHeading')).toBe('On a second machine or a headless box?')
    // Two links inside one sentence, so both are placeholders and the sentence
    // is never split. emphasise() cannot do this — see emphasise.tsx.
    expect(
      t('settings.devices.headlessBody', {
        tokenLink: 'personal access token',
        pairingLink: 'a pairing code',
      })
    ).toBe(
      'Linking needs the plugin and this browser on the same computer. When they are not — a Stream Deck driving a capture PC, a server with no desktop — use a personal access token instead, or start with a pairing code if your plugin is showing one.'
    )
    expect(t('settings.devices.headlessTokenLink')).toBe('personal access token')
    expect(t('settings.devices.headlessPairingLink')).toBe('a pairing code')
  })

  it('keeps the revoke confirmation', () => {
    expect(t('settings.devices.revokeConfirmTitle')).toBe('Revoke this device?')
    expect(t('settings.devices.revokeConfirmBody', { name: 'Deck' })).toBe(
      '“Deck” stops working immediately. Link it again from the plugin if you still want to use it.'
    )
    expect(t('settings.devices.revokeCancel')).toBe('Cancel')
    expect(t('settings.devices.revokeConfirm')).toBe('Revoke device')
    expect(t('settings.devices.revoking')).toBe('Revoking…')
  })
})

describe('streamer premium page copy', () => {
  it('keeps the page chrome', () => {
    expect(t('settings.premium.back')).toBe('← Back to Settings')
    expect(t('settings.premium.heading')).toBe('Premium')
    expect(t('settings.premium.connectPitch')).toBe(
      'Back All-Chat on Patreon to unlock premium features automatically.'
    )
    // The trailing clause after the Patreon link. It is a sentence tail, not a
    // fragment being concatenated into one: the {link} placeholder holds the
    // whole sentence together.
    expect(t('settings.premium.notAPatronSuffix', { link: 'Subscribe on Patreon' })).toBe(
      'Not a patron yet? Subscribe on Patreon, then connect.'
    )
    expect(t('settings.premium.premiumRow')).toBe('Premium')
    expect(t('settings.premium.statusExpired')).toBe('Below premium tier')
    expect(t('settings.premium.notGranting')).toBe(
      'Your Patreon is linked but not granting premium. Make sure your pledge is active and at or above the premium tier.'
    )
    expect(t('settings.premium.disconnectBody')).toBe(
      'This unlinks your Patreon account. Premium granted by your subscription will be removed.'
    )
  })
})

describe('viewer premium page copy', () => {
  it('keeps the authenticated page', () => {
    expect(t('settings.viewerPremium.back')).toBe('← Back to Viewer Identity')
    expect(t('settings.viewerPremium.heading')).toBe('Viewer Premium')
    expect(t('settings.viewerPremium.subheading')).toBe(
      'A cheaper subscription that unlocks viewer cosmetics — your premium chat badge and name gradient — across every overlay you appear in.'
    )
    expect(t('settings.viewerPremium.connectPitch')).toBe(
      'Back All-Chat on Patreon to unlock your viewer premium cosmetics automatically.'
    )
    expect(t('settings.viewerPremium.notAPatronSuffix', { link: 'Subscribe on Patreon' })).toBe(
      'Not a patron yet? Subscribe on Patreon (viewer tier from €2), then connect.'
    )
    expect(t('settings.viewerPremium.premiumRow')).toBe('Viewer premium')
    expect(t('settings.viewerPremium.statusExpired')).toBe('Below viewer tier')
    expect(t('settings.viewerPremium.notGranting')).toBe(
      'Your Patreon is linked but not granting viewer premium. Make sure your pledge is active and at or above the viewer tier.'
    )
    expect(t('settings.viewerPremium.disconnectBody')).toBe(
      'This unlinks your Patreon account. Viewer premium granted by your subscription will be removed.'
    )
  })

  it('keeps the unauthenticated page', () => {
    expect(t('settings.viewerPremium.signInHeading')).toBe('Sign in to manage viewer premium')
    expect(t('settings.viewerPremium.signInBody')).toBe(
      'Sign in with your streaming platform account, then back All-Chat on Patreon to unlock your premium chat badge and cosmetics.'
    )
    expect(t('settings.viewerPremium.signInLink')).toBe('Go to viewer sign-in →')
  })
})

describe('revocation toast copy', () => {
  it('keeps the device revocation toasts', () => {
    expect(t('settings.devices.revokedToast', { name: 'Stream Deck' })).toBe('Revoked Stream Deck')
    expect(t('settings.devices.revokeFailedToast')).toBe('Could not revoke that device')
  })

  it('keeps the API token revocation toasts', () => {
    // Same sentence shape as the device toast, deliberately not shared: the
    // two name different things and a language may inflect the verb for each.
    expect(t('settings.apiTokens.revokedToast', { name: 'CI token' })).toBe('Revoked CI token')
    expect(t('settings.apiTokens.revokeFailedToast')).toBe('Could not revoke that token')
  })
})

describe('settings index toast copy', () => {
  it('keeps the setup-guide and account toasts', () => {
    expect(t('settings.index.restartGuideFailedToast')).toBe('Could not restart the setup guide')
    expect(t('settings.index.accountDeletedToast')).toBe('Account deleted')
    expect(t('settings.index.accountDeleteFailedToast')).toBe('Failed to delete account')
  })

  it('keeps the Discord link toasts', () => {
    // Four distinct outcomes: a server connected, an account linked, an account
    // unlinked, and the unlink failing. Not collapsible -- they name different
    // things even though three of them read similarly.
    expect(t('settings.index.discordServerConnectedToast')).toBe('Discord server connected!')
    expect(t('settings.index.discordLinkedToast')).toBe('Discord account linked!')
    expect(t('settings.index.discordUnlinkedToast')).toBe('Discord account unlinked')
    expect(t('settings.index.discordUnlinkFailedToast')).toBe(
      'Could not unlink your Discord account'
    )
    expect(t('settings.index.discordDisconnectFailedToast')).toBe('Failed to disconnect server')
  })
})
