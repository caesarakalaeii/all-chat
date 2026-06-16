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

import { useNotificationSocket } from '@/hooks/useNotificationSocket'
import { HEARTBEAT_INTERVAL_MS, LIVENESS_TIMEOUT_MS } from '@/lib/utils/overlayStreamCore'

// --- Mock WebSocket -------------------------------------------------------
class MockWebSocket {
  static OPEN = 1
  static CONNECTING = 0
  static CLOSING = 2
  static CLOSED = 3
  static instances: MockWebSocket[] = []

  url: string
  readyState = MockWebSocket.CONNECTING
  closed = false
  sent: string[] = []
  onopen: (() => void) | null = null
  onmessage: ((ev: { data: string }) => void) | null = null
  onerror: ((ev?: unknown) => void) | null = null
  onclose: ((ev?: unknown) => void) | null = null

  constructor(url: string) {
    this.url = url
    MockWebSocket.instances.push(this)
  }
  send(data: string) {
    this.sent.push(data)
  }
  close() {
    this.closed = true
    this.readyState = MockWebSocket.CLOSED
    // Real close() fires onclose asynchronously; tests drive that explicitly via
    // simulateServerClose() to keep reconnect scheduling deterministic.
  }
  simulateOpen() {
    this.readyState = MockWebSocket.OPEN
    this.onopen?.()
  }
  simulateMessage(obj: unknown) {
    this.onmessage?.({ data: JSON.stringify(obj) })
  }
  simulateServerClose() {
    this.readyState = MockWebSocket.CLOSED
    this.onclose?.({ code: 1006, reason: 'gone' })
  }
}

const latest = () => MockWebSocket.instances[MockWebSocket.instances.length - 1]

beforeEach(() => {
  MockWebSocket.instances = []
  vi.stubGlobal('WebSocket', MockWebSocket as unknown as typeof WebSocket)
})

afterEach(() => {
  cleanup()
  vi.useRealTimers()
  vi.unstubAllGlobals()
})

describe('useNotificationSocket', () => {
  it('does not connect without an id or token', () => {
    renderHook(() => useNotificationSocket(undefined, 't', () => {}))
    renderHook(() => useNotificationSocket('o1', undefined, () => {}))
    expect(MockWebSocket.instances).toHaveLength(0)
  })

  it('opens an authenticated socket for the overlay', () => {
    renderHook(() => useNotificationSocket('o1', 'tok', () => {}))
    expect(latest().url).toContain('/ws/overlay/o1')
    expect(latest().url).toContain('token=tok')
  })

  it('delivers parsed envelopes to the callback', async () => {
    const onEnvelope = vi.fn()
    renderHook(() => useNotificationSocket('o1', 'tok', onEnvelope))
    await act(async () => {
      latest().simulateOpen()
      latest().simulateMessage({ type: 'share_revoked', data: { revoked_by_username: 'bob' } })
    })
    expect(onEnvelope).toHaveBeenCalledWith({
      type: 'share_revoked',
      data: { revoked_by_username: 'bob' },
    })
  })

  it('ignores malformed frames without throwing', async () => {
    const onEnvelope = vi.fn()
    renderHook(() => useNotificationSocket('o1', 'tok', onEnvelope))
    await act(async () => {
      latest().simulateOpen()
      latest().onmessage?.({ data: 'not json' })
    })
    expect(onEnvelope).not.toHaveBeenCalled()
  })

  it('sends an app-level ping heartbeat once open', async () => {
    vi.useFakeTimers()
    renderHook(() => useNotificationSocket('o1', 'tok', () => {}))
    await act(async () => {
      latest().simulateOpen()
    })
    await act(async () => {
      await vi.advanceTimersByTimeAsync(HEARTBEAT_INTERVAL_MS + 100)
    })
    expect(latest().sent.some((m) => JSON.parse(m).type === 'ping')).toBe(true)
  })

  it('reconnects on a clean server close (never gives up)', async () => {
    vi.useFakeTimers()
    renderHook(() => useNotificationSocket('o1', 'tok', () => {}))
    expect(MockWebSocket.instances).toHaveLength(1)
    await act(async () => {
      latest().simulateOpen()
      latest().simulateServerClose()
    })
    await act(async () => {
      await vi.advanceTimersByTimeAsync(2500)
    })
    expect(MockWebSocket.instances.length).toBeGreaterThanOrEqual(2)
  })

  it('forces a reconnect when the socket goes silent (half-open)', async () => {
    vi.useFakeTimers()
    renderHook(() => useNotificationSocket('o1', 'tok', () => {}))
    const first = latest()
    await act(async () => {
      first.simulateOpen()
    })
    // No inbound frames at all: the watchdog must declare the path dead and the
    // backoff must then fire a fresh socket — all without any onclose.
    await act(async () => {
      await vi.advanceTimersByTimeAsync(LIVENESS_TIMEOUT_MS + 5000)
    })
    await act(async () => {
      await vi.advanceTimersByTimeAsync(2500)
    })
    expect(first.closed).toBe(true)
    expect(MockWebSocket.instances.length).toBeGreaterThanOrEqual(2)
  })

  it('closes the socket and cancels reconnect on unmount', async () => {
    vi.useFakeTimers()
    const { unmount } = renderHook(() => useNotificationSocket('o1', 'tok', () => {}))
    const ws = latest()
    await act(async () => {
      ws.simulateOpen()
    })
    unmount()
    expect(ws.closed).toBe(true)
    const countAfterUnmount = MockWebSocket.instances.length
    await act(async () => {
      await vi.advanceTimersByTimeAsync(60000)
    })
    expect(MockWebSocket.instances.length).toBe(countAfterUnmount)
  })
})
