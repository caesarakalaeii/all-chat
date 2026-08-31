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
 * Strings genuinely shared by two or more surfaces.
 *
 * A string belongs here only once a second surface reads it. Moving a string in
 * from one caller "for later" makes every surface depend on copy no reader owns.
 */

export const common = {
  brand: {
    // Set in lowercase by every nav that renders it (the marketing header, the
    // app nav, the admin rail and the admin top nav), unlike the 'All-Chat' of
    // prose and aria labels. A locale that transliterates the product name would
    // change this; one that does not leaves it alone.
    wordmark: 'all-chat',
  },
  // The logged-in app nav, which is chrome on every authenticated surface
  // (dashboard, settings, overlays, docs, admin) rather than copy owned by one of
  // them. Its wordmark and Discord label read common.brand and common.platforms.
  appNav: {
    dashboard: 'Dashboard',
    flairs: 'Flairs',
    admin: 'Admin',
    settings: 'Settings',
    docs: 'Docs',
    logOut: 'Log out',
  },
  // The soundPlayer presets. Read by the overlay editor's on-stream notification
  // sounds and by the monitor view's private activity sound. Casing rules are
  // language-specific, so the display name cannot be derived by capitalising the
  // stored lowercase value.
  soundPresets: {
    chime: 'Chime',
    pop: 'Pop',
    ping: 'Ping',
  },
  // Product names, keyed by the platform identifier the code already carries.
  // Read by the moderator roster and by the overlay editor's bubble colour
  // picker, which is why they are not in either namespace.
  platforms: {
    twitch: 'Twitch',
    youtube: 'YouTube',
    kick: 'Kick',
    tiktok: 'TikTok',
    discord: 'Discord',
  },
  // The Patreon connect flow. /settings/premium and /settings/viewer/premium
  // render these byte-identically; the strings that name which premium tier is
  // meant stay in each page's own namespace.
  patreon: {
    heading: 'Patreon',
    connect: 'Connect Patreon',
    connecting: 'Redirecting…',
    notAPatron: 'Not a patron yet?',
    subscribe: 'Subscribe on Patreon',
    subscriptionRow: 'Subscription',
    renewsRow: 'Renews',
    active: 'Active',
    inactive: 'Inactive',
    disconnect: 'Disconnect Patreon',
    disconnectTitle: 'Disconnect Patreon?',
    disconnectCancel: 'Cancel',
    disconnectConfirm: 'Yes, disconnect',
    // Pledge statuses, keyed by the value the payment API returns. The fifth,
    // 'expired', names the tier and so lives per page.
    statusActive: 'Active',
    statusDeclined: 'Payment declined',
    statusFormer: 'Ended',
    statusNotSubscribed: 'Not subscribed',
  },
  // Compact elapsed durations, read by formatCompactDuration in lib/utils.ts.
  // They sit here rather than in a surface namespace because that helper is
  // generic infrastructure: a lib module reaching into admin.* would invert the
  // dependency. The unit letters are copy — a language that does not abbreviate
  // days as 'd' has nowhere else to say so.
  duration: {
    justNow: 'just now',
    minutes: '{minutes}m',
    hours: '{hours}h',
    hoursAndMinutes: '{hours}h {minutes}m',
    days: '{days}d',
    daysAndHours: '{days}d {hours}h',
  },
  // AllChatBadge and PremiumBadge render on the overlay chat rows, the appearance
  // groups and the viewer settings tabs, so their labels belong to no one surface.
  badges: {
    allChatLabel: 'All-Chat badge',
    premiumLabel: 'Premium badge',
  },
  dialog: {
    closeLabel: 'Close dialog',
  },
  toast: {
    closeLabel: 'Close notification',
    // Three callers: /dashboard, /overlays/new and the onboarding create dialog.
    tryAgain: 'Please try again.',
    // Raised by both the viewer overlay and the editor's embedded preview.
    elevenLabsFallback: 'ElevenLabs unavailable \u2014 using browser voice.',
  },
  // SplitView and ResizableSplit share the divider label; the step buttons and
  // the preview iframe title are SplitView's.
  splitPane: {
    resizeLabel: 'Resize panels',
    shrinkConfigLabel: 'Shrink config panel',
    growConfigLabel: 'Grow config panel',
    previewTitle: 'Overlay live preview',
  },
  cssEditor: {
    regionLabel: 'Custom CSS editor',
    keyboardHint: 'Press Ctrl+M to toggle Tab capturing; Escape then Tab leaves the editor.',
    loading: 'Loading editor...',
  },
  impersonation: {
    bannerLabel: 'Admin Mode:',
    // One sentence with the username as a param: it was two JSX runs either side
    // of the name, which a language that fronts the name could not reorder.
    viewingAs: 'Viewing as {username}',
    exitButton: 'Exit & Return to Admin',
  },
  moderatingElsewhere: {
    channelOne: '{count} channel',
    channelMany: '{count} channels',
    // The bolded count phrase is a param, so emphasise can wrap it wherever the
    // language puts it.
    sentence: 'You moderate {channels} for other streamers.',
    openLink: 'Open',
  },
  maintenanceInfo: {
    buttonLabel: 'Service announcements',
    popoverHeading: 'Service announcements',
  },
  eventSubMigration: {
    body: 'The old IRC chat connection is being retired and can drop messages when many streams are live. Reconnect to move to the new connection and keep your chat reliable.',
    reconnectButton: 'Reconnect now',
    dismissLabel: 'Dismiss',
    connectedToast: 'Twitch chat connected',
    failedToast: 'Could not start the upgrade',
  },
  forbidden: {
    heading: '403 Forbidden',
    body: 'You do not have permission to access this page. Admin privileges are required.',
    dashboardButton: 'Go to Dashboard',
  },
  // Two whole sets rather than one template with a {platform} hole. The source
  // spliced a capitalised platform name into shared sentences, but YouTube's copy
  // differs from TikTok's by more than that name, so collapsing them would be a
  // rewording.
  betaWarning: {
    youtubeTitle: 'YouTube — OAuth Verification in Progress',
    tiktokTitle: 'TikTok — Closed Beta',
    youtubeBody:
      'YouTube integration is currently under Google OAuth verification review. We cannot add new test users during this period.',
    tiktokBody:
      "TikTok integration is currently in closed beta. If you haven't been added to the beta program yet, authentication will fail.",
    youtubeDiscordPrompt:
      'Join our Discord community to stay updated on verification progress and get support:',
    tiktokDiscordPrompt: 'To join the beta, please join our Discord community:',
    youtubeExistingUser:
      'If you were previously added as a test user, you can continue to use YouTube integration.',
    tiktokExistingUser: "If you're already in the beta, you can proceed with authentication.",
    discordLink: 'Join Discord Server',
    cancelButton: 'Cancel',
    continueButton: 'I Understand, Continue',
  },
} as const
