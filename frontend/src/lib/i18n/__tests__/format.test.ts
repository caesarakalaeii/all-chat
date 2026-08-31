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

import { afterEach, describe, expect, it, vi } from 'vitest'

import { formatDateTime, formatNumber } from '@/lib/i18n/format'

afterEach(() => {
  vi.restoreAllMocks()
})

describe('formatNumber', () => {
  it('groups thousands using the default locale', () => {
    expect(formatNumber(1234567)).toBe('1,234,567')
  })

  it('formats using an explicitly passed locale', () => {
    expect(formatNumber(1234567, 'en')).toBe('1,234,567')
  })

  it('reuses one Intl.NumberFormat across calls with the same locale', () => {
    const spy = vi.spyOn(Intl, 'NumberFormat')
    formatNumber(1)
    formatNumber(2)
    expect(spy).not.toHaveBeenCalled()
  })
})

describe('formatDateTime', () => {
  const instant = new Date('2026-03-04T15:06:00Z')

  it('formats a Date with the given options in the default locale', () => {
    expect(
      formatDateTime(instant, {
        timeZone: 'UTC',
        month: 'short',
        day: 'numeric',
        hour: '2-digit',
        minute: '2-digit',
      })
    ).toBe('Mar 4, 03:06 PM')
  })

  it('accepts an epoch-millisecond number', () => {
    expect(
      formatDateTime(instant.getTime(), { timeZone: 'UTC', year: 'numeric', month: 'numeric' })
    ).toBe('3/2026')
  })

  it('ignores the browser locale, so output does not depend on the host', () => {
    // The point of pinning: whatever locale the OBS browser source runs under,
    // an English UI must render English month names.
    expect(formatDateTime(instant, { timeZone: 'UTC', month: 'long' }, 'en')).toBe('March')
  })

  it('reuses one Intl.DateTimeFormat across calls with the same locale and options', () => {
    const options: Intl.DateTimeFormatOptions = { timeZone: 'UTC', month: 'short' }
    formatDateTime(instant, options)
    const spy = vi.spyOn(Intl, 'DateTimeFormat')
    formatDateTime(instant, { ...options })
    expect(spy).not.toHaveBeenCalled()
  })

  it('builds separate formatters for different options', () => {
    expect(formatDateTime(instant, { timeZone: 'UTC', month: 'short' })).toBe('Mar')
    expect(formatDateTime(instant, { timeZone: 'UTC', month: 'long' })).toBe('March')
  })
})
