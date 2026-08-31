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

import {
  formatDate,
  formatDateTime,
  formatNumber,
  formatTime,
  formatTimestamp,
} from '@/lib/i18n/format'

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

/**
 * The three presets exist so a call site migrating off toLocaleDateString(),
 * toLocaleTimeString() or toLocaleString() has a drop-in with the same output.
 * Each is asserted against what the corresponding toLocale* call produces for
 * the same instant in the same locale: the migration's rule is that rendered
 * copy does not change, and a preset whose option set is merely plausible would
 * break that silently.
 */
describe('the toLocale* presets render exactly what they replace', () => {
  const instant = new Date('2026-03-04T15:06:07Z')

  it('formatDate matches toLocaleDateString', () => {
    expect(formatDate(instant, 'UTC')).toBe(instant.toLocaleDateString('en', { timeZone: 'UTC' }))
    expect(formatDate(instant, 'UTC')).toBe('3/4/2026')
  })

  it('formatTime matches toLocaleTimeString', () => {
    expect(formatTime(instant, 'UTC')).toBe(instant.toLocaleTimeString('en', { timeZone: 'UTC' }))
    expect(formatTime(instant, 'UTC')).toBe('3:06:07 PM')
  })

  it('formatTimestamp matches toLocaleString, keeping both date and time', () => {
    // dateStyle:'short' would have been the obvious option set and is wrong: it
    // renders a 2-digit year, so timestamps would have silently lost a century.
    expect(formatTimestamp(instant, 'UTC')).toBe(instant.toLocaleString('en', { timeZone: 'UTC' }))
    expect(formatTimestamp(instant, 'UTC')).toBe('3/4/2026, 3:06:07 PM')
  })

  it('pins the locale, so a host default cannot leak in', () => {
    expect(formatDate(instant, 'UTC')).not.toBe(
      instant.toLocaleDateString('de', { timeZone: 'UTC' })
    )
  })
})
