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
 * Locale to TFunction: the only place that maps a locale to its catalog.
 *
 * Kept apart from ./index.ts so ./use-translations.ts can reach it without an
 * import cycle through the barrel.
 */

import { getActiveLocale, getLoadedCatalog } from './active-locale'
import { DEFAULT_LOCALE, type Locale } from './config'
import { enMessages } from './messages/en'
import { translate, translateWithFallback, type TFunction } from './translate'

/**
 * One TFunction per locale, built on first use, for getTranslations(locale).
 *
 * The identity has to be stable: a component that resolves copy inside
 * useEffect or useCallback must list t in its dependency array, and a fresh
 * closure per render would re-fire that effect on every render. Guarded by
 * __tests__/index.test.tsx.
 */
const translatorsByLocale = new Map<Locale, TFunction>()

export function translatorFor(locale: Locale): TFunction {
  const cached = translatorsByLocale.get(locale)
  if (cached) return cached
  // The catalog is looked up per call, not captured: a non-default catalog
  // loads asynchronously, and a translator built before it arrived must not
  // stay English. A key the catalog lacks resolves to English.
  const translator: TFunction =
    locale === DEFAULT_LOCALE
      ? (key, params) => translate(enMessages, key, params)
      : (key, params) =>
          translateWithFallback(getLoadedCatalog(locale) ?? {}, enMessages, key, params)
  translatorsByLocale.set(locale, translator)
  return translator
}

/**
 * Resolves the active locale on every call: what useTranslations() and an
 * argument-less getTranslations() return, so neither changes identity on a
 * switch and a module-scope `t` follows one.
 */
export const activeTranslator: TFunction = (key, params) =>
  translatorFor(getActiveLocale())(key, params)
