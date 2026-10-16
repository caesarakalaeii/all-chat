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
 * The UI locale this browser tab is rendering in (ADR-0066).
 *
 * A module singleton, not a Context or a request read: Storybook renders
 * components without a provider, and reading cookies() or headers() would make
 * every static page dynamic. The cost is that the server always renders
 * DEFAULT_LOCALE and the stored choice is applied on the client after
 * hydration, by restoreStoredLocale(), which components/LocaleSync.tsx calls.
 *
 * Until then the active locale stays DEFAULT_LOCALE on the client too, so the
 * first client render matches the server HTML for every reader: the hook, the
 * module-scope translator and the formatters.
 */

import { DEFAULT_LOCALE, isSupportedLocale, type Locale } from './config'
import type { LocaleMessages } from './translate'

const STORAGE_KEY = 'allchat_locale'

// Non-default catalogs are fetched only when a user picks them, so the English
// bundle every overlay and public page loads does not grow per language.
const catalogLoaders: {
  readonly [L in Exclude<Locale, typeof DEFAULT_LOCALE>]: () => Promise<LocaleMessages>
} = {
  de: () => import('./messages/de').then((module) => module.deMessages),
}

const loadedCatalogs = new Map<Locale, LocaleMessages>()
const listeners = new Set<() => void>()
let activeLocale: Locale = DEFAULT_LOCALE

export function getActiveLocale(): Locale {
  return activeLocale
}

/** The catalog for a non-default locale, once it has been loaded. */
export function getLoadedCatalog(locale: Locale): LocaleMessages | undefined {
  return loadedCatalogs.get(locale)
}

export function subscribeToLocale(listener: () => void): () => void {
  listeners.add(listener)
  return () => {
    listeners.delete(listener)
  }
}

async function activate(locale: Locale): Promise<void> {
  if (locale !== DEFAULT_LOCALE && !loadedCatalogs.has(locale)) {
    loadedCatalogs.set(locale, await catalogLoaders[locale]())
  }
  if (locale === activeLocale) return
  activeLocale = locale
  document.documentElement.lang = locale
  for (const listener of listeners) listener()
}

/**
 * Apply the locale this browser stored, if any. Runs once, after hydration.
 *
 * No stored choice means DEFAULT_LOCALE, never the browser's Accept-Language:
 * an OBS browser source on a German machine must not switch a streamer's
 * overlay chrome to German mid-broadcast because of a setting nobody chose.
 */
export async function restoreStoredLocale(): Promise<void> {
  let stored: string | null
  try {
    stored = localStorage.getItem(STORAGE_KEY)
  } catch {
    // Storage is blocked (sandboxed frame, strict privacy mode): stay on the default.
    return
  }
  if (stored !== null && isSupportedLocale(stored)) await activate(stored)
}

/** Switch the UI locale and remember the choice for this browser. */
export async function chooseLocale(locale: Locale): Promise<void> {
  await activate(locale)
  try {
    localStorage.setItem(STORAGE_KEY, locale)
  } catch {
    // Blocked storage: the switch still applies to this tab, it just is not remembered.
  }
}
