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

import { customDaysToSeconds, DAY_SECONDS, MAX_DAYS } from '../PremiumDurationChooser'

describe('customDaysToSeconds (ADR-0027 time-limited grant duration)', () => {
  it('converts a valid whole-day count to seconds', () => {
    expect(customDaysToSeconds('7')).toEqual({ seconds: 7 * DAY_SECONDS, valid: true })
    expect(customDaysToSeconds('1')).toEqual({ seconds: DAY_SECONDS, valid: true })
  })

  it('accepts the maximum but rejects beyond it', () => {
    expect(customDaysToSeconds(String(MAX_DAYS))).toEqual({
      seconds: MAX_DAYS * DAY_SECONDS,
      valid: true,
    })
    expect(customDaysToSeconds(String(MAX_DAYS + 1))).toEqual({ seconds: null, valid: false })
  })

  it('rejects blank, zero, negative, and non-numeric input', () => {
    for (const bad of ['', '   ', '0', '-1', 'abc', 'NaN']) {
      expect(customDaysToSeconds(bad)).toEqual({ seconds: null, valid: false })
    }
  })

  it('rounds fractional days to whole seconds', () => {
    expect(customDaysToSeconds('1.5')).toEqual({ seconds: Math.round(1.5 * DAY_SECONDS), valid: true })
  })
})
