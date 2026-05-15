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

// Inline copy of the seen-id cache used by both the OBS overlay page and the
// shared WebSocketClient. Keeping this in sync with the implementations is a
// manual contract but the surface is tiny.
function makeSeenIdCache(capacity: number) {
  const set = new Set<string>()
  const order: string[] = []
  return {
    markIdSeen(id: string): boolean {
      if (!id) return false
      if (set.has(id)) return true
      set.add(id)
      order.push(id)
      if (order.length > capacity) {
        const evicted = order.shift()
        if (evicted) set.delete(evicted)
      }
      return false
    },
    size(): number {
      return set.size
    },
  }
}

describe('seen-id dedup cache', () => {
  it('returns false on first sight and true on duplicate', () => {
    const cache = makeSeenIdCache(1024)
    expect(cache.markIdSeen('msg-1')).toBe(false)
    expect(cache.markIdSeen('msg-1')).toBe(true)
  })

  it('treats empty id as never-seen so empty ids never dedup', () => {
    const cache = makeSeenIdCache(1024)
    expect(cache.markIdSeen('')).toBe(false)
    expect(cache.markIdSeen('')).toBe(false)
  })

  it('evicts oldest id once capacity is exceeded', () => {
    const cache = makeSeenIdCache(3)
    cache.markIdSeen('a')
    cache.markIdSeen('b')
    cache.markIdSeen('c')
    cache.markIdSeen('d') // evicts 'a'

    expect(cache.size()).toBe(3)
    expect(cache.markIdSeen('a')).toBe(false) // 'a' was evicted, reappears as new
    expect(cache.markIdSeen('b')).toBe(true)
    expect(cache.markIdSeen('c')).toBe(true)
    expect(cache.markIdSeen('d')).toBe(true)
  })

  it('keeps recent ids when many distinct ids stream in', () => {
    const cache = makeSeenIdCache(4)
    for (let i = 0; i < 100; i++) cache.markIdSeen(`id-${i}`)
    expect(cache.size()).toBe(4)
    expect(cache.markIdSeen('id-99')).toBe(true)  // most recent
    expect(cache.markIdSeen('id-0')).toBe(false)  // long evicted
  })
})

describe('lastSeen watermark advance', () => {
  // Mirrors the inline logic in page.tsx and websocket.ts. Refactoring to share
  // would mean cross-file plumbing; the snippet is two lines.
  function advance(current: number, timestampIso: string): number {
    const tsMs = Date.parse(timestampIso)
    if (Number.isFinite(tsMs) && tsMs > current) return tsMs
    return current
  }

  it('advances on newer messages', () => {
    let last = 0
    last = advance(last, '2026-05-15T10:00:00.000Z')
    expect(last).toBe(Date.UTC(2026, 4, 15, 10, 0, 0))
    last = advance(last, '2026-05-15T10:00:01.000Z')
    expect(last).toBe(Date.UTC(2026, 4, 15, 10, 0, 1))
  })

  it('does not regress on older or out-of-order messages', () => {
    let last = Date.UTC(2026, 4, 15, 10, 0, 5)
    last = advance(last, '2026-05-15T10:00:00.000Z')
    expect(last).toBe(Date.UTC(2026, 4, 15, 10, 0, 5))
  })

  it('ignores malformed timestamps', () => {
    let last = 1000
    last = advance(last, 'not-a-date')
    expect(last).toBe(1000)
  })
})
