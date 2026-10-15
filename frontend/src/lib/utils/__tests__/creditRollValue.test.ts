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

import { getTranslations } from '@/lib/i18n'
import type { LeaderboardEntry } from '@/lib/types/overlay'
import { creditRollValue } from '@/lib/utils/creditRollValue'

const t = getTranslations()

function entry(totalValue: number | undefined, currency?: string): LeaderboardEntry {
  return {
    rank: 1,
    user_id: 'u1',
    display_name: 'Viewer',
    avatar_url: '',
    platform: 'twitch',
    total_value: totalValue,
    metadata: currency === undefined ? {} : { currency },
  }
}

describe('creditRollValue', () => {
  // Regression: every leaderboard total used to render as dollars, so a 500-bit
  // cheer read "$500.00" and a 3-month sub "$3.00".
  it('renders non-monetary totals in their own unit, never as money', () => {
    expect(creditRollValue(t, entry(500, 'bits'))).toBe('500 bits')
    expect(creditRollValue(t, entry(3, 'months'))).toBe('3 months')
    expect(creditRollValue(t, entry(250, 'viewers'))).toBe('250 viewers')
    expect(creditRollValue(t, entry(5000, 'points'))).toBe('5,000 points')
    expect(creditRollValue(t, entry(5, 'gifts'))).toBe('5 gifts')
  })

  it("uses the streamer's own name for their channel points", () => {
    const named = {
      ...entry(5000, 'points'),
      metadata: { currency: 'points', points_name: 'Kappa Coins' },
    }
    expect(creditRollValue(t, named)).toBe('5,000 Kappa Coins')
    // Only channel points are renamed, even if the field rides along on another entry.
    const bits = {
      ...entry(500, 'bits'),
      metadata: { currency: 'bits', points_name: 'Kappa Coins' },
    }
    expect(creditRollValue(t, bits)).toBe('500 bits')
  })

  it('uses the singular for exactly one', () => {
    expect(creditRollValue(t, entry(1, 'bits'))).toBe('1 bit')
    expect(creditRollValue(t, entry(1, 'months'))).toBe('1 month')
    expect(creditRollValue(t, entry(1, 'gift'))).toBe('1 gift')
  })

  it('picks the unit per entry, since one leaderboard can mix platforms', () => {
    // Top Gifters: a Twitch gift sub next to a TikTok gift counted in diamonds.
    expect(creditRollValue(t, entry(2, 'gift'))).toBe('2 gifts')
    expect(creditRollValue(t, entry(300, 'diamonds'))).toBe('300 diamonds')
  })

  it('renders only ISO currency totals as money, in their own currency', () => {
    expect(creditRollValue(t, entry(12.5, 'USD'))).toBe('$12.50')
    expect(creditRollValue(t, entry(12.5, 'EUR'))).toBe('€12.50')
  })

  it('shows nothing for count-only events or an empty total', () => {
    expect(creditRollValue(t, entry(1))).toBe('') // Twitch follow: no value at all
    expect(creditRollValue(t, entry(1, 'follow'))).toBe('') // TikTok follow
    expect(creditRollValue(t, entry(0, 'bits'))).toBe('')
    expect(creditRollValue(t, entry(undefined, 'bits'))).toBe('')
  })
})
