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
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { WebSocketClient } from '@/lib/api/websocket'
import { LIVENESS_TIMEOUT_MS, WATCHDOG_INTERVAL_MS } from '@/lib/utils/overlayStreamCore'

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

function makeLocalStorageMock(): Storage {
  let store: Record<string, string> = {}
  return {
    getItem: (k: string) => (k in store ? store[k] : null),
    setItem: (k: string, v: string) => {
      store[k] = String(v)
    },
    removeItem: (k: string) => {
      delete store[k]
    },
    clear: () => {
      store = {}
    },
    key: (i: number) => Object.keys(store)[i] ?? null,
    get length() {
      return Object.keys(store).length
    },
  } as Storage
}

beforeEach(() => {
  MockWebSocket.instances = []
  vi.stubGlobal('WebSocket', MockWebSocket as unknown as typeof WebSocket)
  vi.stubGlobal('localStorage', makeLocalStorageMock())
})

afterEach(() => {
  vi.useRealTimers()
  vi.unstubAllGlobals()
})

describe('WebSocketClient — liveness & resilient reconnect', () => {
  it('reports disconnected once the socket goes silent past the liveness timeout', () => {
    vi.useFakeTimers()
    const client = new WebSocketClient()
    client.connect('o1', 'tok')
    latest().simulateOpen()
    expect(client.isConnected()).toBe(true)

    // Half-open: readyState stays OPEN but no frames arrive. isConnected() must
    // stop lying before the 1s preview poll reads it.
    vi.advanceTimersByTime(LIVENESS_TIMEOUT_MS + 500)
    expect(client.isConnected()).toBe(false)
    client.disconnect()
  })

  it('sends an app-level ping heartbeat while open', () => {
    vi.useFakeTimers()
    const client = new WebSocketClient()
    client.connect('o1', 'tok')
    const ws = latest()
    ws.simulateOpen()
    vi.advanceTimersByTime(LIVENESS_TIMEOUT_MS - 1000)
    expect(ws.sent.map((s) => JSON.parse(s).type)).toContain('ping')
    client.disconnect()
  })

  it('reconnects a silent half-open socket without waiting for onclose', () => {
    vi.useFakeTimers()
    const client = new WebSocketClient()
    client.connect('o1', 'tok')
    latest().simulateOpen()
    const before = MockWebSocket.instances.length

    vi.advanceTimersByTime(LIVENESS_TIMEOUT_MS + WATCHDOG_INTERVAL_MS + 100) // watchdog trips
    vi.advanceTimersByTime(2000) // scheduled reconnect fires
    expect(MockWebSocket.instances.length).toBeGreaterThan(before)
    client.disconnect()
  })

  it('never permanently gives up — keeps reconnecting well past the old 10-attempt cap', () => {
    vi.useFakeTimers()
    const client = new WebSocketClient()
    client.connect('o1', 'tok')
    for (let i = 0; i < 15; i++) {
      latest().simulateOpen()
      latest().simulateServerClose()
      vi.advanceTimersByTime(35000) // longer than the 30s max backoff
    }
    const countAfter15 = MockWebSocket.instances.length
    // A 16th close must still schedule another reconnect (old code stopped at 10).
    latest().simulateServerClose()
    vi.advanceTimersByTime(35000)
    expect(MockWebSocket.instances.length).toBeGreaterThan(countAfter15)
    client.disconnect()
  })

  it('stops reconnecting after disconnect()', () => {
    vi.useFakeTimers()
    const client = new WebSocketClient()
    client.connect('o1', 'tok')
    latest().simulateOpen()
    client.disconnect()
    const count = MockWebSocket.instances.length
    latest().simulateServerClose()
    vi.advanceTimersByTime(60000)
    expect(MockWebSocket.instances.length).toBe(count)
  })
})
