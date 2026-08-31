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
  // Settings → API tokens: minting, listing and revoking the personal access
  // tokens the Stream Deck and StreamController plugins authenticate with.
  apiTokens: {
    heading: 'API Tokens',
    subheading: 'Personal access tokens for the Stream Deck and StreamController plugins.',
    listHeading: 'Your tokens',
    listBody:
      'Only the details below are stored — the token itself is kept as a hash and can never be shown again.',
    loadFailed: 'Could not load your tokens. Refresh the page to try again.',
    revealRegionLabel: 'New token {name}',
    revealHeading: 'Copy your new token now',
    // {name} is emphasised at the render site. The sentence stays whole so a
    // translator can move the name; see docs/frontend/I18N.md.
    revealWarning:
      'This is the only time {name} will ever be shown. We store only a hash of it, so it cannot be displayed again — if you lose it, revoke this token and create a new one.',
    copyToken: 'Copy token',
    copied: 'Copied ✓',
    copyFailed: 'Could not copy automatically — select the token and copy it manually.',
    // The render site spelled the apostrophe &apos;, which is U+0027.
    dismissReveal: "I've saved it",
    createHeading: 'Create a token',
    createBody:
      'Give the token a name you will recognise later, and grant it only what the device needs.',
    nameLabel: 'Token name',
    namePlaceholder: 'Stream Deck (studio PC)',
    nameDescription: 'Shown in the list below so you know what to revoke.',
    scopesLegend: 'Scopes',
    noScopesWarning:
      'Pick at least one scope — a token with none can authenticate but do nothing.',
    create: 'Create token',
    creating: 'Creating…',
    createFailed: 'Could not create the token. Try again.',
    // Keyed by the wire scope name with the colon dropped, so API_TOKEN_SCOPES
    // stays the single list of scopes and a new one fails tsc until its copy
    // lands. A fourth key level is not available; see __tests__/messages.test.ts.
    scopeChatWriteTitle: 'Send chat messages',
    scopeChatWriteDescription: 'Lets the plugin post messages to your connected chats.',
    scopeEngagementWriteTitle: 'Run polls and predictions',
    scopeEngagementWriteDescription:
      'Lets the plugin open, resolve and cancel polls and predictions.',
    emptyHeading: "You don't have any API tokens yet",
    emptyBody:
      "A personal access token lets a device sign in as you without your password — it's how the Stream Deck and StreamController plugins send chat messages and run polls and predictions on your behalf. Create one per device so you can revoke it on its own.",
    setupGuides: 'Setup guides:',
    streamDeckReadme: 'Stream Deck plugin README',
    streamControllerReadme: 'StreamController plugin README',
    // One key, two placeholders: a second language reorders the two dates and
    // the middle dot is not a boundary it can rely on.
    tokenDates: 'Created {created} · Last used {lastUsed}',
    neverUsed: 'never',
    // Stands in for a missing or unparseable timestamp.
    unknownDate: '—',
    revokeLabel: 'Revoke {name}',
    revoke: 'Revoke',
    revokeConfirmTitle: 'Revoke this token?',
    revokeConfirmBody:
      '“{name}” stops working immediately. Any device using it will need a new token.',
    revokeCancel: 'Cancel',
    revokeConfirm: 'Revoke token',
    revoking: 'Revoking…',
  },
} as const
