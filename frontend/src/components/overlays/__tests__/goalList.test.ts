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

import { applyGoalFrame, applyListUpdate, goalPercent } from '../goalList'
import type { GoalState, ListTop } from '@/lib/types/goal'

describe('goalPercent', () => {
  it('is the share of the target, clamped to 0..100', () => {
    expect(goalPercent(25, 100)).toBe(25)
    expect(goalPercent(150, 100)).toBe(100)
    expect(goalPercent(-5, 100)).toBe(0)
    expect(goalPercent(10, 0)).toBe(0)
  })
})

describe('applyGoalFrame', () => {
  const state: GoalState = {
    goal: { metric: 'bits', platform: 'twitch', target: 1000, starting_value: 0, title: 'Bits' },
    configured: true,
    current: 100,
    status: '',
  }

  it('takes current, target and title from a frame of the same metric', () => {
    const next = applyGoalFrame(state, {
      type: 'goal',
      overlay_id: 'ov',
      goal: 'bits',
      current: 400,
      target: 2000,
      title: 'More bits',
    })
    expect(next.current).toBe(400)
    expect(next.goal.target).toBe(2000)
    expect(next.goal.title).toBe('More bits')
  })

  it('switches metric when the owner saved a new goal', () => {
    const next = applyGoalFrame(state, {
      type: 'goal',
      overlay_id: 'ov',
      goal: 'followers',
      current: 7,
      target: 50,
      title: '',
    })
    expect(next.goal.metric).toBe('followers')
    expect(next.goal.platform).toBe('twitch')
    expect(next.current).toBe(7)
  })
})

describe('applyListUpdate', () => {
  const top: ListTop = { title: 'Top', category: 'bits', limit: 2, entries: [] }
  const entries = [
    { rank: 2, name: 'B', platform: 'twitch', amount: 50 },
    { rank: 1, name: 'A', platform: 'twitch', amount: 500 },
    { rank: 3, name: 'C', platform: 'twitch', amount: 5 },
  ]

  it('orders by rank and cuts to the configured limit', () => {
    const next = applyListUpdate(top, {
      type: 'list_update',
      overlay_id: 'ov',
      category: 'bits',
      entries,
    })
    expect(next.entries.map((e) => e.name)).toEqual(['A', 'B'])
  })

  it('ignores another board', () => {
    const next = applyListUpdate(top, {
      type: 'list_update',
      overlay_id: 'ov',
      category: 'gifts',
      entries,
    })
    expect(next).toBe(top)
  })

  it("switches board on the owner's save", () => {
    const next = applyListUpdate(top, {
      type: 'list_update',
      overlay_id: 'ov',
      category: 'gifts',
      entries,
      limit: 3,
      title: 'Gifters',
    })
    expect(next.category).toBe('gifts')
    expect(next.title).toBe('Gifters')
    expect(next.entries).toHaveLength(3)
  })
})
