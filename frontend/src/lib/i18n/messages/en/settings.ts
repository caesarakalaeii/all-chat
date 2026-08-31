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
} as const
