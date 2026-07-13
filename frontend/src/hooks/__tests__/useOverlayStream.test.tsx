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
import { act, cleanup, renderHook, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import type { ChatMessage } from '@/lib/types/message'
import { useOverlayStream } from '@/hooks/useOverlayStream'
import {
  HEARTBEAT_INTERVAL_MS,
  LIVENESS_TIMEOUT_MS,
  WATCHDOG_INTERVAL_MS,
} from '@/lib/utils/overlayStreamCore'

vi.mock('@/lib/twitchBadges', () => ({ resolveTwitchBadgeIcons: vi.fn(async (m: ChatMessage) => m) }))
vi.mock('@/lib/badgeOrder', () => ({ sortMessageBadges: vi.fn((m: ChatMessage) => m) }))

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
    // Real close() fires onclose asynchronously; tests drive that explicitly
    // via simulateServerClose() to keep reconnect scheduling deterministic.
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

function chat(id: string, ts = '2026-05-30T10:00:00.000Z'): ChatMessage {
  return {
    id,
    overlay_id: 'o1',
    platform: 'twitch',
    channel_id: 'c1',
    channel_name: 'chan',
    user: { id: 'u1', username: 'u', display_name: 'U', badges: [] },
    message: { text: 'hi', emotes: [] },
    timestamp: ts,
    metadata: {},
  }
}

const latest = () => MockWebSocket.instances[MockWebSocket.instances.length - 1]

// jsdom in this project is configured without a Storage; stub a functional one.
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
  vi.stubGlobal(
    'fetch',
    vi.fn(async () => ({ ok: true, json: async () => ({ sources: [] }) })) as unknown as typeof fetch,
  )
  vi.stubGlobal('localStorage', makeLocalStorageMock())
})

afterEach(() => {
  // Unmount any rendered hooks first so their heartbeat/watchdog intervals are
  // cleared before we tear down the fake timers and stubbed globals.
  cleanup()
  vi.useRealTimers()
  vi.unstubAllGlobals()
})

describe('useOverlayStream — connection', () => {
  it('builds a ws:// url without ?since when there is no watermark', () => {
    renderHook(() => useOverlayStream('o1', {}))
    expect(latest().url).toMatch(/^ws:\/\//)
    expect(latest().url).toContain('/ws/overlay/o1')
    expect(latest().url).not.toContain('?since=')
  })

  it('includes ?since from the stored watermark', () => {
    localStorage.setItem('ws_last_seen_o1', '12345')
    renderHook(() => useOverlayStream('o1', {}))
    expect(latest().url).toContain('?since=12345')
  })

  it('uses wss:// when the page is served over https', () => {
    const original = window.location
    Object.defineProperty(window, 'location', {
      configurable: true,
      value: { ...original, protocol: 'https:', host: 'allch.at' },
    })
    try {
      renderHook(() => useOverlayStream('o1', {}))
      expect(latest().url).toBe('wss://allch.at/ws/overlay/o1')
    } finally {
      Object.defineProperty(window, 'location', { configurable: true, value: original })
    }
  })

  it('reports open and fires onConnected on handshake', async () => {
    const onConnected = vi.fn()
    const { result } = renderHook(() => useOverlayStream('o1', { onConnected }))
    await act(async () => {
      latest().simulateOpen()
    })
    expect(result.current.connectionStatus).toBe('open')
    expect(onConnected).toHaveBeenCalledTimes(1)
  })

  it('schedules an exponential-backoff reconnect on close', async () => {
    vi.useFakeTimers()
    renderHook(() => useOverlayStream('o1', {}))
    expect(MockWebSocket.instances).toHaveLength(1)
    await act(async () => {
      latest().simulateServerClose()
    })
    // computeBackoffDelay(0) is < 2000ms; advancing well past it fires the reconnect.
    await act(async () => {
      await vi.advanceTimersByTimeAsync(2500)
    })
    expect(MockWebSocket.instances.length).toBeGreaterThanOrEqual(2)
  })

  it('closes the socket and cancels reconnect on unmount', async () => {
    vi.useFakeTimers()
    const { unmount } = renderHook(() => useOverlayStream('o1', {}))
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

describe('useOverlayStream — message routing', () => {
  it('replays buffered deletions via onDeletion(source=replay) and never as chat', async () => {
    const onDeletion = vi.fn()
    const onChat = vi.fn()
    renderHook(() => useOverlayStream('o1', { onDeletion, onChat }))
    await act(async () => {
      latest().simulateMessage({
        type: 'replay_response',
        data: [
          { deletion_type: 'single', target_uuid: 'a' },
          { deletion_type: 'clear' },
        ],
      })
    })
    expect(onDeletion).toHaveBeenCalledTimes(2)
    expect(onDeletion).toHaveBeenNthCalledWith(1, { deletion_type: 'single', target_uuid: 'a' }, 'replay')
    expect(onDeletion).toHaveBeenNthCalledWith(2, { deletion_type: 'clear' }, 'replay')
    expect(onChat).not.toHaveBeenCalled()
  })

  it('routes a live message_deletion to onDeletion(source=live)', async () => {
    const onDeletion = vi.fn()
    renderHook(() => useOverlayStream('o1', { onDeletion }))
    await act(async () => {
      latest().simulateMessage({
        type: 'chat_message',
        data: {
          ...chat('d1'),
          event: {
            type: 'message_deletion',
            tier: 'low',
            duration: 0,
            is_update: false,
            metadata: { deletion_type: 'single', target_uuid: 'm0' },
          },
        },
      })
    })
    expect(onDeletion).toHaveBeenCalledWith({ deletion_type: 'single', target_uuid: 'm0' }, 'live')
  })

  it('dedups chat messages by id but not aggregate updates', async () => {
    const onChat = vi.fn()
    const onMessageUpdate = vi.fn()
    renderHook(() => useOverlayStream('o1', { onChat, onMessageUpdate }))
    await act(async () => {
      latest().simulateMessage({ type: 'chat_message', data: chat('m1') })
      latest().simulateMessage({ type: 'chat_message', data: chat('m1') })
    })
    await waitFor(() => expect(onChat).toHaveBeenCalledTimes(1))

    await act(async () => {
      latest().simulateMessage({ type: 'message_update', data: chat('agg') })
      latest().simulateMessage({ type: 'message_update', data: chat('agg') })
    })
    await waitFor(() => expect(onMessageUpdate).toHaveBeenCalledTimes(2))
  })

  it('advances and persists the watermark from chat timestamps', async () => {
    const onChat = vi.fn()
    renderHook(() => useOverlayStream('o1', { onChat }))
    await act(async () => {
      latest().simulateMessage({ type: 'chat_message', data: chat('m1', '2026-05-30T10:00:05.000Z') })
    })
    await waitFor(() => expect(onChat).toHaveBeenCalled())
    expect(localStorage.getItem('ws_last_seen_o1')).toBe(String(Date.UTC(2026, 4, 30, 10, 0, 5)))
  })

  it('tracks connected channels in activeChannels', async () => {
    const { result } = renderHook(() => useOverlayStream('o1', {}))
    await act(async () => {
      latest().simulateMessage({
        type: 'platform_status',
        data: { platform: 'twitch', channel_id: 'c1', status: 'connected' },
      })
    })
    await waitFor(() => expect(result.current.activeChannels.has('c1')).toBe(true))
    expect(result.current.channelStatuses.get('c1')?.status).toBe('connected')
  })

  it('self-heals: reconnects to replay when a source recovers from a down state, but not on initial connect', async () => {
    vi.useFakeTimers()
    renderHook(() => useOverlayStream('o1', {}))
    await act(async () => {
      latest().simulateOpen()
    })
    expect(MockWebSocket.instances).toHaveLength(1)

    // Initial 'connected' establishes the source — it must NOT trigger a reconnect
    // (otherwise every fresh overlay load would immediately reconnect).
    await act(async () => {
      latest().simulateMessage({
        type: 'platform_status',
        data: { platform: 'twitch', channel_id: 'c1', status: 'connected' },
      })
      await vi.advanceTimersByTimeAsync(100)
    })
    expect(MockWebSocket.instances).toHaveLength(1)

    // Source drops (transport stays alive; the silence watchdog cannot see this)...
    await act(async () => {
      latest().simulateMessage({
        type: 'platform_status',
        data: { platform: 'twitch', channel_id: 'c1', status: 'offline' },
      })
    })
    // ...and comes back: the down->connected transition must force an immediate replay
    // reconnect so the gap is backfilled without a manual page refresh.
    await act(async () => {
      latest().simulateMessage({
        type: 'platform_status',
        data: { platform: 'twitch', channel_id: 'c1', status: 'connected' },
      })
      await vi.advanceTimersByTimeAsync(50)
    })
    expect(MockWebSocket.instances.length).toBeGreaterThanOrEqual(2)
    expect(latest().url).toContain('/ws/overlay/o1')
  })

  it('always calls the latest callback after a re-render (no stale closure)', async () => {
    const first = vi.fn()
    const second = vi.fn()
    const { rerender } = renderHook(({ cb }) => useOverlayStream('o1', { onChat: cb }), {
      initialProps: { cb: first },
    })
    rerender({ cb: second })
    await act(async () => {
      latest().simulateMessage({ type: 'chat_message', data: chat('m1') })
    })
    await waitFor(() => expect(second).toHaveBeenCalledTimes(1))
    expect(first).not.toHaveBeenCalled()
  })
})

describe('useOverlayStream — liveness / heartbeat', () => {
  const sentTypes = (ws: MockWebSocket) =>
    ws.sent.map((s) => (JSON.parse(s) as { type?: string }).type)

  it('sends an app-level ping heartbeat while the socket is open', async () => {
    vi.useFakeTimers()
    renderHook(() => useOverlayStream('o1', {}))
    await act(async () => {
      latest().simulateOpen()
    })
    const ws = latest()
    await act(async () => {
      await vi.advanceTimersByTimeAsync(HEARTBEAT_INTERVAL_MS + 100)
    })
    expect(sentTypes(ws).filter((t) => t === 'ping').length).toBeGreaterThanOrEqual(1)
  })

  it('stays open as long as inbound frames keep arriving (pong replies)', async () => {
    vi.useFakeTimers()
    const { result } = renderHook(() => useOverlayStream('o1', {}))
    await act(async () => {
      latest().simulateOpen()
    })
    const ws = latest()
    // Server answers the heartbeat well within the timeout, repeatedly, for far
    // longer than LIVENESS_TIMEOUT_MS. The watchdog must never trip.
    for (let i = 0; i < 6; i++) {
      await act(async () => {
        await vi.advanceTimersByTimeAsync(LIVENESS_TIMEOUT_MS - 5000)
      })
      await act(async () => {
        ws.simulateMessage({ type: 'pong' })
      })
    }
    expect(result.current.connectionStatus).toBe('open')
    expect(MockWebSocket.instances).toHaveLength(1)
  })

  it('detects a silent half-open socket (no onclose) and reconnects', async () => {
    vi.useFakeTimers()
    const { result } = renderHook(() => useOverlayStream('o1', {}))
    await act(async () => {
      latest().simulateOpen()
    })
    const first = latest()
    expect(result.current.connectionStatus).toBe('open')

    // Connection silently dies: no frames, no onclose. Past the timeout the
    // watchdog must notice and flip the status — without waiting on onclose.
    await act(async () => {
      await vi.advanceTimersByTimeAsync(LIVENESS_TIMEOUT_MS + WATCHDOG_INTERVAL_MS + 100)
    })
    expect(result.current.connectionStatus).toBe('reconnecting')
    expect(first.closed).toBe(true)
    const detectedCount = MockWebSocket.instances.length

    // The scheduled reconnect then fires and opens a fresh socket.
    await act(async () => {
      await vi.advanceTimersByTimeAsync(3000)
    })
    expect(MockWebSocket.instances.length).toBeGreaterThan(detectedCount)
  })

  it('reconnects immediately (skipping backoff) when the network comes back online', async () => {
    vi.useFakeTimers()
    renderHook(() => useOverlayStream('o1', {}))
    await act(async () => {
      latest().simulateOpen()
    })
    await act(async () => {
      latest().simulateServerClose() // schedules a ~1.5s backoff reconnect
    })
    const before = MockWebSocket.instances.length
    await act(async () => {
      window.dispatchEvent(new Event('online'))
      await vi.advanceTimersByTimeAsync(50) // far less than the backoff delay
    })
    expect(MockWebSocket.instances.length).toBe(before + 1)
  })

  it('reconnects when the tab becomes visible again on a dead socket', async () => {
    vi.useFakeTimers()
    renderHook(() => useOverlayStream('o1', {}))
    await act(async () => {
      latest().simulateOpen()
    })
    await act(async () => {
      latest().simulateServerClose()
    })
    const before = MockWebSocket.instances.length
    await act(async () => {
      document.dispatchEvent(new Event('visibilitychange'))
      await vi.advanceTimersByTimeAsync(50)
    })
    expect(MockWebSocket.instances.length).toBe(before + 1)
  })

  it('resets the attempt counter after a successful reconnect (no escalation)', async () => {
    vi.useFakeTimers()
    const { result } = renderHook(() => useOverlayStream('o1', {}))
    await act(async () => {
      latest().simulateOpen()
    })
    // First drop → after the reconnect fires the counter is 1.
    await act(async () => {
      latest().simulateServerClose()
    })
    await act(async () => {
      await vi.advanceTimersByTimeAsync(2500)
    })
    expect(result.current.reconnectAttempts).toBe(1)
    // Reconnect succeeds → counter resets to 0.
    await act(async () => {
      latest().simulateOpen()
    })
    expect(result.current.reconnectAttempts).toBe(0)
    // A fresh drop starts back at 1, not escalated from the previous burst.
    await act(async () => {
      latest().simulateServerClose()
    })
    await act(async () => {
      await vi.advanceTimersByTimeAsync(2500)
    })
    expect(result.current.reconnectAttempts).toBe(1)
  })

  it('does not tear down a healthy socket on online/visibility events', async () => {
    vi.useFakeTimers()
    const { result } = renderHook(() => useOverlayStream('o1', {}))
    await act(async () => {
      latest().simulateOpen()
    })
    const before = MockWebSocket.instances.length
    await act(async () => {
      window.dispatchEvent(new Event('online'))
      document.dispatchEvent(new Event('visibilitychange'))
      await vi.advanceTimersByTimeAsync(50)
    })
    expect(MockWebSocket.instances.length).toBe(before)
    expect(result.current.connectionStatus).toBe('open')
  })
})
