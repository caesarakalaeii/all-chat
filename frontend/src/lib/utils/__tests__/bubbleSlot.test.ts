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
import {
  admitBubbleSlot,
  bubbleSlot,
  EMPTY_BUBBLE_SLOTS,
  type BubbleSlotState,
} from '../bubbleSlot'

const admitAll = (ids: string[], keep = 100): BubbleSlotState =>
  ids.reduce((state, id) => admitBubbleSlot(state, id, keep), EMPTY_BUBBLE_SLOTS)

describe('bubble palette slots', () => {
  it('cycles through the palette in arrival order', () => {
    const state = admitAll(['a', 'b', 'c', 'd'])
    expect(['a', 'b', 'c', 'd'].map((id) => bubbleSlot(state, id, 3))).toEqual([0, 1, 2, 0])
  })

  /**
   * The whole reason this module exists. The feed keeps the newest N with
   * `slice(-max)`, so cycling on list position would re-colour every row on
   * screen each time a message arrives.
   */
  it('keeps a row on its colour as older rows are pruned', () => {
    let state = admitAll(['a', 'b', 'c'])
    const before = bubbleSlot(state, 'c', 3)

    // 'd' arrives — 'c' has moved from index 2 to index 1 in the rendered list
    state = admitBubbleSlot(state, 'd', 100)

    expect(bubbleSlot(state, 'c', 3)).toBe(before)
    expect(bubbleSlot(state, 'd', 3)).toBe(0)
  })

  it('recolours consistently when the palette length changes', () => {
    const state = admitAll(['a', 'b', 'c', 'd'])
    expect(['a', 'b', 'c', 'd'].map((id) => bubbleSlot(state, id, 2))).toEqual([0, 1, 0, 1])
    expect(['a', 'b', 'c', 'd'].map((id) => bubbleSlot(state, id, 4))).toEqual([0, 1, 2, 3])
  })

  /** Safe inside a React state updater, which may run twice. */
  it('is idempotent per id and returns the same object', () => {
    const first = admitAll(['a', 'b'])
    const again = admitBubbleSlot(first, 'a', 100)

    expect(again).toBe(first)
    expect(bubbleSlot(again, 'a', 3)).toBe(0)
    expect(bubbleSlot(again, 'b', 3)).toBe(1)
  })

  it('never mutates the state it was given', () => {
    const first = admitAll(['a'])
    admitBubbleSlot(first, 'b', 100)
    expect(first.byId.size).toBe(1)
    expect(first.next).toBe(1)
  })

  it('has no slot without a palette to cycle, or for an unknown id', () => {
    const state = admitAll(['a'])
    expect(bubbleSlot(state, 'a', 0)).toBeUndefined()
    expect(bubbleSlot(state, 'a', 1)).toBeUndefined()
    expect(bubbleSlot(state, 'never-seen', 3)).toBeUndefined()
  })

  it('evicts the oldest arrivals instead of growing without bound', () => {
    let state: BubbleSlotState = EMPTY_BUBBLE_SLOTS
    for (let i = 0; i < 5000; i++) {
      state = admitBubbleSlot(state, `m${i}`, 100)
    }

    expect(state.byId.size).toBe(100)
    // Arrival numbers keep counting, so the cycle does not restart on eviction
    expect(bubbleSlot(state, 'm4999', 3)).toBe(4999 % 3)
    expect(bubbleSlot(state, 'm0', 3)).toBeUndefined()
  })

  it('keeps at least one entry even with a nonsensical limit', () => {
    const state = admitBubbleSlot(admitAll(['a'], 0), 'b', 0)
    expect(state.byId.size).toBe(1)
    expect(bubbleSlot(state, 'b', 2)).toBe(1 % 2)
  })
})
