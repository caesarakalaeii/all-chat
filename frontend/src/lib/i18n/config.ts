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
 * Locale configuration.
 *
 * English is the only supported locale today. Adding a language means editing
 * SUPPORTED_LOCALES here and adding one file under messages/ — nothing else in
 * this module, and nothing at any call site.
 *
 * Locale deliberately never appears in a URL: overlay URLs are pasted into OBS
 * browser sources, shared links, the browser extension and the Stream Deck
 * plugins, so a /[locale]/ segment would break every overlay that already
 * exists. See docs/adr/0055-ui-string-catalog-without-locale-routing.md.
 */

export const SUPPORTED_LOCALES = ['en'] as const

export type Locale = (typeof SUPPORTED_LOCALES)[number]

export const DEFAULT_LOCALE: Locale = 'en'

export function isSupportedLocale(value: string): value is Locale {
  return (SUPPORTED_LOCALES as readonly string[]).includes(value)
}
