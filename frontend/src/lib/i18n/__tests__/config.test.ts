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

import { describe, expect, it } from 'vitest'

import { DEFAULT_LOCALE, SUPPORTED_LOCALES, isSupportedLocale } from '@/lib/i18n/config'

describe('isSupportedLocale', () => {
  it('accepts every supported locale', () => {
    for (const locale of SUPPORTED_LOCALES) {
      expect(isSupportedLocale(locale)).toBe(true)
    }
  })

  it('rejects a locale that is not in SUPPORTED_LOCALES', () => {
    expect(isSupportedLocale('de')).toBe(false)
  })

  it('rejects a region-tagged form of a supported locale', () => {
    // The catalog is keyed by the bare code, so 'en-GB' is not a key and must not
    // pass the guard: accepting it would resolve to no catalog at all.
    expect(isSupportedLocale('en-GB')).toBe(false)
  })
})

describe('DEFAULT_LOCALE', () => {
  it('is one of the supported locales', () => {
    // Guards the retrofit mistake of adding a locale and repointing the default
    // at a code with no catalog behind it.
    expect(isSupportedLocale(DEFAULT_LOCALE)).toBe(true)
  })
})
