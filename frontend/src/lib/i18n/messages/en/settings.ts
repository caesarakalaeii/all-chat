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
 * The settings surfaces, including linked devices and platform connections.
 */

export const settings = {
  ambassador: {
    heading: 'Ambassador',
    // The render site spelled the apostrophe &rsquo; and the quotes
    // &ldquo;/&rdquo;. A catalog string is not HTML, so they are the characters.
    body: 'You’re an All-Chat ambassador. Choose whether to be featured on the public homepage.',
    featureToggle: 'Feature me on the homepage',
    cardReads: 'Your card reads: “{tagline}”',
    toastFeatured: 'You will now appear on the homepage',
    toastUnfeatured: 'Removed from the homepage showcase',
    toastFailed: 'Failed to update showcase setting',
  },
  // The /settings landing page: profile, the cards linking to each sub-page, the
  // Discord server and account links, and account deletion.
  index: {
    heading: 'Settings',
    profileHeading: 'Profile',
    usernameLabel: 'Username',
    primaryPlatformLabel: 'Primary Platform',
    primaryPlatformUnknown: 'Unknown',
    setupGuideHeading: 'Setup guide',
    setupGuideBody:
      'Walk through overlay setup again: create an overlay, connect chat, pick a theme, and get the OBS link.',
    setupGuideRestart: 'Restart',
    setupGuideRestarting: 'Restarting\u2026',
    premiumHeading: 'Premium',
    premiumBody: 'Unlock premium features by backing All-Chat on Patreon.',
    premiumManage: 'Manage Premium',
    devicesHeading: 'Paired devices',
    devicesBody:
      'See and revoke the Stream Deck and StreamController control surfaces linked to your account. Each one is locked to a single overlay.',
    devicesManage: 'Manage devices',
    tokensHeading: 'API tokens',
    tokensBody:
      'Create and revoke personal access tokens. Use these where linking cannot reach — a headless capture box, a second PC, or a script.',
    tokensManage: 'Manage tokens',
    // The render site spelled the ampersand &amp;; a catalog string is not HTML.
    privacyHeading: 'Data & Privacy',
    privacyBody:
      'We keep data collection minimal and transparent. Review the policies below for details about how tokens, overlays, and chat metadata are processed.',
    privacyPolicyLink: 'Privacy Policy',
    termsLink: 'Terms of Service',
    discordHeading: 'Discord',
    discordServersLoading: 'Loading Discord servers',
    discordNoServer: 'No Discord server connected.',
    discordConnectServer: 'Connect Discord Server',
    discordConnectAnother: 'Connect another server',
    discordDisconnect: 'Disconnect',
    discordDisconnectTitle: 'Disconnect {guild}?',
    discordDisconnectBody: 'This will remove all Discord sources connected to {guild}.',
    discordDisconnectCancel: 'Cancel',
    discordDisconnectConfirm: 'Yes, disconnect',
    discordAccountHeading: 'Your Discord account',
    discordAccountLoading: 'Loading your Discord account link',
    discordAccountLinked:
      'Linked as {username}. Needed so moderators can act on your Discord servers.',
    // Substituted for {username} when Discord did not return one.
    discordAccountFallbackName: 'your Discord account',
    discordAccountUnlinked:
      'Not linked. Link it to let moderators you invite act on your Discord servers.',
    discordAccountUnlink: 'Unlink',
    discordAccountLink: 'Link Discord account',
    dangerHeading: 'Danger Zone',
    dangerBody:
      'Deleting your account removes all overlays, OAuth grants, and cached chat sources. This action is permanent and cannot be undone.',
    deleteAccount: 'Delete Account',
    deleteConfirmTitle: 'Delete your account?',
    deleteConfirmBody:
      'This permanently deletes your account and all overlays. This action cannot be undone.',
    deleteCancel: 'Cancel',
    deleteConfirm: 'Yes, delete my account',
  },
  // The viewer-facing identity page (/settings/viewer): name colour, avatar
  // cosmetics and the platform links that share them across chats.
  viewer: {
    heading: 'Viewer Identity',
    subheading: 'Customize how your name appears across all overlays',
    signInHeading: 'Sign in to manage your viewer identity',
    signInBody:
      'Connect your streaming platform account to set a custom name color and manage your viewer identity.',
    // One key per platform rather than a {platform} placeholder: the source has
    // three separate buttons and a placeholder would hide which three.
    signInTwitch: 'Sign in with Twitch',
    signInYoutube: 'Sign in with YouTube',
    signInKick: 'Sign in with Kick',
    profileHeading: 'Profile',
    // Stands in for a viewer with neither a display name nor a username.
    viewerFallbackName: 'Viewer',
    nameColorHeading: 'Name Color',
    nameColorBody: 'Set a custom color or gradient for your name on overlays',
    solidTab: 'Solid Color',
    gradientTab: 'Gradient',
    premiumPill: 'Premium',
    gradientUpsell: 'Gradient names are a viewer premium cosmetic.',
    unlockPremium: 'Unlock viewer premium',
    colorPickerLabel: 'Name color picker',
    colorHexLabel: 'Name color hex value',
    savedFeedback: 'Saved ✓',
    autoSaveNote: 'Changes save automatically',
    previewLabel: 'Preview',
    previewMessage: 'Hello world!',
    removeStopLabel: 'Remove stop {index}',
    addStop: '+ Add stop',
    angleLabel: 'Angle',
    angleDegreesLabel: 'Angle in degrees',
    saveGradient: 'Save gradient',
    savingGradient: 'Saving…',
    cosmeticsHeading: 'Avatar Cosmetics',
    cosmeticsBody: 'Choose a frame and flair for your avatar',
    cosmeticsUpsell: 'Some frames and flairs are viewer premium.',
    frameHeading: 'Avatar Frame',
    flairHeading: 'Avatar Flair',
    // The catalog row standing for "no frame" / "no flair".
    noneItem: 'None',
    save: 'Save',
    saving: 'Saving…',
    premiumRequired: 'Premium required',
    saveFailed: 'Save failed',
    linkedHeading: 'Linked Platforms',
    linkedBody: 'Connect additional platforms to share your cosmetics across all your chats',
    connected: 'Connected',
    connect: 'Connect',
    connecting: 'Connecting…',
    disconnect: 'Disconnect',
    disconnecting: 'Disconnecting…',
    loadLinkedFailed: 'Could not load linked platforms',
    disconnectFailed: 'Failed to disconnect platform',
  },
} as const
