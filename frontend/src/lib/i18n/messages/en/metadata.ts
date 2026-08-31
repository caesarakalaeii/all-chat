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
 * Route `export const metadata` copy: SEO titles and descriptions.
 *
 * Read from Server Components, so via `getTranslations()`.
 */

export const metadata = {
  // The generated social card. Its text ships as an image, so the alt is the
  // only version a screen reader ever reaches; both are copy.
  socialCard: {
    alt: 'All-Chat — Every chat. One overlay.',
    title: 'All-Chat',
    subtitle: 'Every chat. One overlay.',
    emoteProviders: '7TV + BTTV + FFZ Emotes',
    tagline: 'One overlay. Every chat. All platforms.',
  },
  impressum: {
    title: 'Impressum | All-Chat',
    description: 'Legal notice (Impressum) as required by § 5 DDG.',
  },
  terms: {
    title: 'Terms of Service | All-Chat',
    description: 'Understand the rules and responsibilities for using All-Chat.',
  },
} as const
