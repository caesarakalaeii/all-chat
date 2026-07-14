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

import { describe, it, expect } from 'vitest'
import { formatCompactDuration, formatConnectedFor } from '@/lib/utils'

const MIN = 60_000
const HOUR = 60 * MIN
const DAY = 24 * HOUR

describe('formatCompactDuration', () => {
  it('renders sub-minute durations as "just now"', () => {
    expect(formatCompactDuration(0)).toBe('just now')
    expect(formatCompactDuration(59_000)).toBe('just now')
  })

  it('renders minute-only durations', () => {
    expect(formatCompactDuration(MIN)).toBe('1m')
    expect(formatCompactDuration(8 * MIN)).toBe('8m')
    expect(formatCompactDuration(59 * MIN)).toBe('59m')
  })

  it('renders hours with minutes, dropping zero minutes', () => {
    expect(formatCompactDuration(HOUR)).toBe('1h')
    expect(formatCompactDuration(5 * HOUR + 12 * MIN)).toBe('5h 12m')
    expect(formatCompactDuration(3 * HOUR)).toBe('3h')
  })

  it('renders days with hours, dropping zero hours', () => {
    expect(formatCompactDuration(DAY)).toBe('1d')
    expect(formatCompactDuration(3 * DAY + 4 * HOUR)).toBe('3d 4h')
    expect(formatCompactDuration(2 * DAY + 30 * MIN)).toBe('2d') // minutes hidden at day scale
  })

  it('treats negative and non-finite input as zero', () => {
    expect(formatCompactDuration(-5000)).toBe('just now')
    expect(formatCompactDuration(Number.NaN)).toBe('just now')
    expect(formatCompactDuration(Number.POSITIVE_INFINITY)).toBe('just now')
  })
})

describe('formatConnectedFor', () => {
  const now = Date.parse('2026-07-13T12:00:00Z')

  it('returns null for missing input', () => {
    expect(formatConnectedFor(null, now)).toBeNull()
    expect(formatConnectedFor(undefined, now)).toBeNull()
    expect(formatConnectedFor('', now)).toBeNull()
  })

  it('returns null for unparseable timestamps', () => {
    expect(formatConnectedFor('not-a-date', now)).toBeNull()
  })

  it('formats a valid RFC3339 start relative to now', () => {
    expect(formatConnectedFor('2026-07-13T08:48:00Z', now)).toBe('3h 12m')
    expect(formatConnectedFor('2026-07-10T12:00:00Z', now)).toBe('3d')
  })

  it('clamps a future start to "just now"', () => {
    expect(formatConnectedFor('2026-07-13T12:05:00Z', now)).toBe('just now')
  })
})
