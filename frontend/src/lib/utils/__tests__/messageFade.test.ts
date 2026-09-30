/**
 * This file is part of All-Chat.
 * Copyright (C) 2026 caesarakalaeii
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
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
 * The regression this file defends against: the overlay's fade timer used to
 * re-arm on every append, so under continuous chat no message ever expired.
 * Expiry must be a property of each row's arrival, so a later arrival can
 * never push an earlier row's deadline back.
 */

import { describe, expect, it } from 'vitest'
import { getTierDuration, messageExpiry, nextFadeDelayMs } from '../messageFade'
import type { ChatMessage, EventTier } from '@/lib/types/message'

function chatMessage(id: string, event?: ChatMessage['event']): ChatMessage {
  return {
    id,
    overlay_id: 'ovl',
    platform: 'twitch',
    channel_id: 'c',
    channel_name: 'c',
    user: { id: 'u', username: 'u', display_name: 'u', badges: [] },
    message: { text: 'hi', emotes: [] },
    timestamp: '2026-09-14T00:00:00Z',
    metadata: {},
    event,
  }
}

function eventInfo(tier: EventTier, duration: number): ChatMessage['event'] {
  return {
    type: 'subscription',
    tier,
    duration,
    is_update: false,
    metadata: {},
  }
}

describe('messageExpiry', () => {
  it('anchors expiry to the row arrival, not the last append', () => {
    const arrivedAt = 1_000_000
    // Row arrives, then a second row arrives later. Both use the same
    // configured duration; the first row's deadline must not move.
    const first = messageExpiry(chatMessage('a'), arrivedAt, 10)
    const laterAppendArrival = arrivedAt + 5_000
    const second = messageExpiry(chatMessage('b'), laterAppendArrival, 10)
    expect(first).toBe(arrivedAt + 10_000)
    expect(second).toBe(laterAppendArrival + 10_000)
    // The core property: the second arrival leaves the first expiry unchanged.
    expect(first).toBeLessThan(second)
  })

  it('keeps the earliest deadline fixed while later rows arrive', () => {
    // The old head-of-queue timer re-armed from the full duration on every
    // append; this is the property that broke. After each append the next
    // sweep must still wait only until row a's ORIGINAL deadline.
    const arrivals = new Map<string, number>([['a', 1_000_000]])
    const deadline = 1_000_000 + 10_000
    expect(nextFadeDelayMs([chatMessage('a')], arrivals, 10, 1_005_000)).toBe(deadline - 1_005_000)
    // Two appends later — both after 'a' — the wait shrinks only by elapsed
    // time, never resets to the full duration.
    arrivals.set('b', 1_006_000)
    arrivals.set('c', 1_007_000)
    expect(
      nextFadeDelayMs(
        [chatMessage('a'), chatMessage('b'), chatMessage('c')],
        arrivals,
        10,
        1_007_000
      )
    ).toBe(deadline - 1_007_000)
  })

  it('prefers the event duration, falling back to the tier default', () => {
    const arrivedAt = 5_000
    expect(messageExpiry(chatMessage('e', eventInfo('medium', 42)), arrivedAt, 10)).toBe(
      arrivedAt + 42_000
    )
    // The tier fallback is the documented event display order — higher tiers
    // stay up longer — pinned as relations rather than literals.
    expect(messageExpiry(chatMessage('e', eventInfo('high', 0)), arrivedAt, 10)).toBeGreaterThan(
      messageExpiry(chatMessage('e', eventInfo('medium', 0)), arrivedAt, 10)
    )
    expect(messageExpiry(chatMessage('e', eventInfo('medium', 0)), arrivedAt, 10)).toBeGreaterThan(
      messageExpiry(chatMessage('e', eventInfo('low', 0)), arrivedAt, 10)
    )
  })

  it('expires immediately when the arrival time is unknown', () => {
    const now = Date.now()
    expect(messageExpiry(chatMessage('a'), undefined, 10)).toBeLessThanOrEqual(now)
  })
})

describe('nextFadeDelayMs', () => {
  it('waits until the earliest row expiry', () => {
    const arrivals = new Map<string, number>([
      ['a', 1_000],
      ['b', 2_000],
    ])
    // Earliest deadline is row a at 11s; at now=5s the timer must fire in 6s.
    expect(nextFadeDelayMs([chatMessage('b'), chatMessage('a')], arrivals, 10, 5_000)).toBe(6_000)
  })

  it('clamps overdue rows to an immediate sweep', () => {
    const arrivals = new Map<string, number>([['a', 0]])
    expect(nextFadeDelayMs([chatMessage('a')], arrivals, 10, 60_000)).toBe(0)
  })

  it('returns null for an empty feed', () => {
    expect(nextFadeDelayMs([], new Map(), 10, 0)).toBeNull()
  })

  it('ignores arrival times of rows no longer on screen', () => {
    const arrivals = new Map<string, number>([['gone', 1_000]])
    expect(nextFadeDelayMs([chatMessage('a')], arrivals, 10, 0)).toBe(10_000)
  })
})
