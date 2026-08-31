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
 * Number and date formatting pinned to the UI locale.
 *
 * Passing `undefined` to an Intl constructor means "whatever locale the browser
 * or OBS instance happens to have", so the same English copy could render German
 * month abbreviations. These wrappers default to the UI locale instead, making
 * output deterministic across machines.
 */

import { DEFAULT_LOCALE, type Locale } from './config'

// Constructing an Intl formatter is expensive relative to formatting with one,
// and code that moves off a module-level constant tends to construct per render.
// The date cache is keyed by locale plus the serialised options, so a caller may
// pass a fresh options object each render without paying for a new formatter.
// Passing the same keys in a different order costs one extra cache entry and
// never a wrong result.
const numberFormatters = new Map<Locale, Intl.NumberFormat>()
const dateTimeFormatters = new Map<string, Intl.DateTimeFormat>()

function numberFormatter(locale: Locale): Intl.NumberFormat {
  const cached = numberFormatters.get(locale)
  if (cached) return cached
  const formatter = new Intl.NumberFormat(locale)
  numberFormatters.set(locale, formatter)
  return formatter
}

function dateTimeFormatter(
  locale: Locale,
  options: Intl.DateTimeFormatOptions
): Intl.DateTimeFormat {
  const cacheKey = `${locale}|${JSON.stringify(options)}`
  const cached = dateTimeFormatters.get(cacheKey)
  if (cached) return cached
  const formatter = new Intl.DateTimeFormat(locale, options)
  dateTimeFormatters.set(cacheKey, formatter)
  return formatter
}

export function formatNumber(value: number, locale: Locale = DEFAULT_LOCALE): string {
  return numberFormatter(locale).format(value)
}

/** `value` is a Date or epoch milliseconds. */
export function formatDateTime(
  value: Date | number,
  options: Intl.DateTimeFormatOptions = {},
  locale: Locale = DEFAULT_LOCALE
): string {
  return dateTimeFormatter(locale, options).format(value)
}
