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
 * Call-site entry points for UI strings.
 *
 * See docs/frontend/I18N.md for how to add a string and the checklist for adding
 * a second locale.
 */

import type { Locale } from './config'
import type { TFunction } from './translate'
import { activeTranslator, translatorFor } from './translators'

/**
 * For Server Components, plain modules and tests.
 *
 * Without an argument it follows the active locale. On the server that is
 * always DEFAULT_LOCALE, so statically rendered pages stay static and English;
 * on the client it is whatever the user chose, applied after hydration.
 * A string resolved once at module load is fixed at DEFAULT_LOCALE.
 */
export function getTranslations(locale?: Locale): TFunction {
  return locale === undefined ? activeTranslator : translatorFor(locale)
}

export { useActiveLocale, useTranslations } from './use-translations'
export { chooseLocale, restoreStoredLocale } from './active-locale'

export {
  DEFAULT_LOCALE,
  LOCALE_NATIVE_NAMES,
  SUPPORTED_LOCALES,
  isSupportedLocale,
  type Locale,
} from './config'
export {
  DATE_ONLY,
  TIMESTAMP,
  TIME_ONLY,
  formatCurrency,
  formatDate,
  formatDateTime,
  formatNumber,
  formatTime,
  formatTimestamp,
} from './format'
export type { MessageKey, MessageParams, TFunction } from './translate'
