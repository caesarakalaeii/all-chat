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
