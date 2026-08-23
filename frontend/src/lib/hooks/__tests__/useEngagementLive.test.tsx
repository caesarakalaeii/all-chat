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

// @vitest-environment jsdom
import { act, cleanup, renderHook } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { useEngagementLive } from '@/lib/hooks/useEngagementLive'

// Mock WebSocket in the same style as src/lib/api/__tests__/websocket.test.ts:
// the hook is exercised through the real WebSocketClient, so counting
// constructions here counts real socket opens (initial connect + reconnects).
class MockWebSocket {
  static OPEN = 1
  static CONNECTING = 0
  static CLOSING = 2
  static CLOSED = 3
  static instances: MockWebSocket[] = []

  url: string
  readyState = MockWebSocket.CONNECTING
  onopen: (() => void) | null = null
  onmessage: ((ev: { data: string }) => void) | null = null
  onerror: ((ev?: unknown) => void) | null = null
  onclose: ((ev?: unknown) => void) | null = null

  constructor(url: string) {
    this.url = url
    MockWebSocket.instances.push(this)
  }
  send() {}
  close() {
    this.readyState = MockWebSocket.CLOSED
  }
  simulateOpen() {
    this.readyState = MockWebSocket.OPEN
    this.onopen?.()
  }
  simulateServerClose() {
    this.readyState = MockWebSocket.CLOSED
    this.onclose?.({ code: 1006, reason: 'gone' })
  }
}

const latest = () => MockWebSocket.instances[MockWebSocket.instances.length - 1]

/** Longer than the client's 30s max backoff, so one advance always fires the retry. */
const PAST_MAX_BACKOFF_MS = 35000

beforeEach(() => {
  MockWebSocket.instances = []
  vi.stubGlobal('WebSocket', MockWebSocket as unknown as typeof WebSocket)
  vi.useFakeTimers()
})

afterEach(() => {
  cleanup()
  vi.useRealTimers()
  vi.unstubAllGlobals()
})

describe('useEngagementLive', () => {
  it('gives up after exactly maxReconnectAttempts consecutive failed reconnects', () => {
    const maxReconnectAttempts = 3
    renderHook(() => useEngagementLive('o1', () => {}, { maxReconnectAttempts }))

    // The initial connect is not a reconnect attempt, so the cap allows
    // maxReconnectAttempts further sockets on top of it.
    expect(MockWebSocket.instances).toHaveLength(1)

    for (let i = 0; i < maxReconnectAttempts; i++) {
      act(() => {
        latest().simulateServerClose()
        vi.advanceTimersByTime(PAST_MAX_BACKOFF_MS)
      })
      expect(MockWebSocket.instances).toHaveLength(i + 2)
    }

    // Cap reached: the next close must not schedule anything, however long we wait.
    act(() => {
      latest().simulateServerClose()
      vi.advanceTimersByTime(PAST_MAX_BACKOFF_MS * 5)
    })
    expect(MockWebSocket.instances).toHaveLength(maxReconnectAttempts + 1)
  })

  it('resets the attempt budget after a successful open', () => {
    renderHook(() => useEngagementLive('o1', () => {}, { maxReconnectAttempts: 1 }))

    // Fail once, reconnect, and let that socket open successfully.
    act(() => {
      latest().simulateServerClose()
      vi.advanceTimersByTime(PAST_MAX_BACKOFF_MS)
    })
    expect(MockWebSocket.instances).toHaveLength(2)
    act(() => latest().simulateOpen())

    // The successful open clears the consecutive-failure count, so the budget
    // of 1 is available again rather than exhausted.
    act(() => {
      latest().simulateServerClose()
      vi.advanceTimersByTime(PAST_MAX_BACKOFF_MS)
    })
    expect(MockWebSocket.instances).toHaveLength(3)
  })

  it('does not re-subscribe when the caller re-renders with a new callback identity', () => {
    // The hook keeps onSignal in a ref precisely so an inline arrow function —
    // which every call site passes — cannot churn the socket. A deps-array fix
    // that reintroduces a per-render dependency shows up here as extra sockets.
    const { rerender } = renderHook(
      ({ unrelated }: { unrelated: number }) =>
        useEngagementLive('o1', () => unrelated, { maxReconnectAttempts: 8 }),
      { initialProps: { unrelated: 0 } }
    )
    act(() => latest().simulateOpen())
    expect(MockWebSocket.instances).toHaveLength(1)

    for (let i = 1; i <= 5; i++) {
      rerender({ unrelated: i })
    }

    expect(MockWebSocket.instances).toHaveLength(1)
  })

  it('reconnects without a bound when maxReconnectAttempts is omitted', () => {
    renderHook(() => useEngagementLive('o1', () => {}))

    // The OBS/monitor default (0 = unlimited) must survive far more failures
    // than any bounded socket would.
    for (let i = 0; i < 12; i++) {
      act(() => {
        latest().simulateServerClose()
        vi.advanceTimersByTime(PAST_MAX_BACKOFF_MS)
      })
    }
    expect(MockWebSocket.instances).toHaveLength(13)
  })

  it('does not open a socket when overlayId is empty', () => {
    renderHook(() => useEngagementLive('', () => {}))
    expect(MockWebSocket.instances).toHaveLength(0)
  })
})
