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

import { DEFAULT_LOCALE, type Locale } from './config'
import { enMessages } from './messages/en'
import { translate, type MessageParams, type MessageKey, type TFunction } from './translate'

/**
 * For Server Components, plain modules and tests.
 *
 * The locale argument is accepted and ignored while English is the only catalog,
 * so that call sites already pass what locale #2 will need. Picking a catalog
 * from it then happens here and nowhere else.
 */
export function getTranslations(_locale: Locale = DEFAULT_LOCALE): TFunction {
  return (key: MessageKey, params?: MessageParams) => translate(enMessages, key, params)
}

/**
 * For Client Components.
 *
 * Deliberately not a hook over a context: the frontend has no React Context
 * anywhere, and Storybook renders components standalone under a required CI
 * gate, so a component needing a wrapping provider would fail to render there.
 * It exists as a hook-shaped seam so that adding a locale is a change inside
 * src/lib/i18n/ rather than a sweep across every component.
 */
export function useTranslations(): TFunction {
  return getTranslations(DEFAULT_LOCALE)
}

export { DEFAULT_LOCALE, SUPPORTED_LOCALES, isSupportedLocale, type Locale } from './config'
export { formatDateTime, formatNumber } from './format'
export type { MessageKey, MessageParams, TFunction } from './translate'
