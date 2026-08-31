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
} as const
