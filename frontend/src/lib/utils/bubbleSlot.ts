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
 * Stable palette slot per message, for the "differently-coloured bubbles"
 * setting (`bubblePalette`).
 *
 * Why not `:nth-child` or the render index: the feed keeps the newest N with
 * `slice(-max)`, so once the buffer is full every surviving row's position
 * shifts by one on each new message. Cycling colours on position therefore
 * re-colours the WHOLE feed on every message — a visible shimmer, and the reason
 * the bundled `nth-child(even)` themes flicker on a busy stream today.
 *
 * Arrival order does not shift. Each message gets a monotonic number the first
 * time it is seen, and its slot is `number % paletteLength` — so a row keeps its
 * colour for as long as it is on screen, and changing the palette length
 * recolours everything consistently instead of leaving old rows behind.
 *
 * This is a plain immutable value rather than a mutable tracker in a ref: the
 * overlay pages read it while rendering, and a ref read during render can return
 * something the committed tree never saw. Every function here is pure, and
 * `admitBubbleSlot` is idempotent, so it is safe inside a React state updater
 * (which may run twice).
 */

export interface BubbleSlotState {
  /** Arrival number the next unseen message will take. */
  readonly next: number
  /** message id → arrival number, in ascending arrival order. */
  readonly byId: ReadonlyMap<string, number>
}

export const EMPTY_BUBBLE_SLOTS: BubbleSlotState = { next: 0, byId: new Map() }

/**
 * Records arrival order for a message, keeping at most `keep` entries.
 *
 * Re-admitting a known id returns the state unchanged, so an edited or
 * re-delivered message never jumps colour and a double-invoked state updater
 * cannot burn a number.
 *
 * Eviction drops the oldest arrivals: a stream running for hours would otherwise
 * accumulate one entry per message forever. `Map` preserves insertion order and
 * insertions only ever happen in ascending arrival order, so the oldest entry is
 * simply the first key. Pass roughly twice the on-screen buffer so nothing
 * visible can be evicted.
 */
export function admitBubbleSlot(state: BubbleSlotState, id: string, keep: number): BubbleSlotState {
  if (state.byId.has(id)) return state

  const byId = new Map(state.byId)
  byId.set(id, state.next)
  const limit = Math.max(keep, 1)
  while (byId.size > limit) {
    const oldest = byId.keys().next()
    if (oldest.done) break
    byId.delete(oldest.value)
  }
  return { next: state.next + 1, byId }
}

/**
 * Slot for a rendered row, or undefined when there is no palette to cycle (the
 * caller then omits the attribute entirely). An id that was never admitted also
 * yields undefined rather than a wrong-but-plausible 0.
 */
export function bubbleSlot(
  state: BubbleSlotState,
  id: string,
  paletteLength: number
): number | undefined {
  if (paletteLength < 2) return undefined
  const arrival = state.byId.get(id)
  return arrival === undefined ? undefined : arrival % paletteLength
}
