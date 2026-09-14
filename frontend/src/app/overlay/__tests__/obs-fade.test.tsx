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

// @vitest-environment jsdom

// The regression this file defends against: the overlay's fade timer used to
// re-arm on every append, so under continuous chat no message ever expired.
// The hook's contract here is just "call onChat with each admitted message";
// its own machinery (WebSocket, replay, dedup) is out of scope.

import '@testing-library/jest-dom/vitest'
import { act, cleanup, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import OBSOverlayPage from '@/app/overlay/[id]/page'
import type { UseOverlayStreamOptions } from '@/hooks/useOverlayStream'
import type { ChatMessage } from '@/lib/types/message'

// jsdom lacks window.matchMedia (useReducedMotion, rendered per message row)
// and scrollIntoView (the auto-scroll effect). Static non-matching stubs,
// same approach as overlays/[id]/__tests__/page.test.tsx.
window.matchMedia = (query: string) =>
  ({
    matches: false,
    media: query,
    addEventListener: () => {},
    removeEventListener: () => {},
  }) as unknown as MediaQueryList
Element.prototype.scrollIntoView = () => {}

// The page builds a fresh options object each render; the mock merges each
// one into this capture so the test always fires the latest callbacks.
const streamOptions: UseOverlayStreamOptions = {}

// Config lives in the hook RETURN, not the options — a mutable holder lets
// the mid-session tests swap display_settings and re-render with it, the
// same way the real 30s refresh delivers a fresh config object.
const streamConfig: { config: Record<string, unknown> } = {
  config: {
    display_settings: {
      message_duration: 10,
      max_messages: 50,
    },
  },
}

vi.mock('@/hooks/useOverlayStream', () => ({
  useOverlayStream: (_id: string, options: UseOverlayStreamOptions) => {
    Object.assign(streamOptions, options)
    return {
      config: streamConfig.config,
      sources: new Map(),
      activeChannels: new Set(),
      channelStatuses: new Map(),
      connectionStatus: 'connected',
      reconnectAttempts: 0,
      replayTruncated: false,
    }
  },
}))

function onChat(): (m: ChatMessage) => void {
  const cb = streamOptions.onChat
  if (!cb) throw new Error('onChat not captured yet — page not rendered')
  return cb
}

/**
 * The page reads its id from a promise prop via React's `use()`. A thenable
 * that already carries React's fulfilled bookkeeping unwraps without
 * suspending (same trick as overlays/[id]/__tests__/page.test.tsx).
 */
function resolvedParams(id: string): Promise<{ id: string }> {
  return Object.assign(Promise.resolve({ id }), {
    status: 'fulfilled' as const,
    value: { id },
  })
}

function chatMessage(id: string, text: string): ChatMessage {
  return {
    id,
    overlay_id: 'test',
    platform: 'twitch',
    channel_id: 'c',
    channel_name: 'c',
    user: { id: `u-${id}`, username: id, display_name: id, badges: [] },
    message: { text, emotes: [] },
    timestamp: new Date().toISOString(),
    metadata: {},
  }
}

describe('overlay fade regression', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    streamConfig.config = {
      display_settings: {
        message_duration: 10,
        max_messages: 50,
      },
    }
  })

  afterEach(() => {
    vi.useRealTimers()
    cleanup()
  })

  it('removes the head message at its own deadline while newer messages keep arriving', async () => {
    render(<OBSOverlayPage params={resolvedParams('test')} />)
    // One act tick lets the hook mock capture the callbacks.
    await act(async () => {})
    onChat()(chatMessage('m1', 'first'))

    // Continuous chat: a new message every second — shorter than the 10s
    // duration, the exact condition that used to re-arm the head timer forever.
    for (let i = 2; i <= 15; i++) {
      await vi.advanceTimersByTimeAsync(1000)
      onChat()(chatMessage(`m${i}`, `msg ${i}`))
    }

    // 14 seconds passed since m1 arrived; its 10s deadline must have fired.
    await act(async () => {
      await vi.advanceTimersByTimeAsync(0)
    })
    expect(screen.queryByText('first')).not.toBeInTheDocument()

    // Only the expired row left; the latest arrival is still on screen.
    expect(screen.getByText('msg 15')).toBeInTheDocument()
  })

  it('sweeps all overdue rows on one fire, not just the head', async () => {
    render(<OBSOverlayPage params={resolvedParams('test')} />)
    await act(async () => {})
    // Two rows arrive in the same tick — one throttled fire must later clear
    // BOTH, not pop one row per fire (the old prev.slice(1) behavior).
    onChat()(chatMessage('m1', 'first'))
    await act(async () => {})
    onChat()(chatMessage('m2', 'second'))
    await act(async () => {})
    await act(async () => {
      await vi.advanceTimersByTimeAsync(5_000)
    })
    onChat()(chatMessage('m3', 'third'))
    await act(async () => {})

    // t=10.5s: rows 1 and 2 (deadline t=10s) are both overdue, row 3 (deadline
    // t=15s) is not. The single sweep at their shared deadline must clear both.
    await act(async () => {
      await vi.advanceTimersByTimeAsync(5_500)
    })
    expect(screen.queryByText('first')).not.toBeInTheDocument()
    expect(screen.queryByText('second')).not.toBeInTheDocument()
    expect(screen.getByText('third')).toBeInTheDocument()
  })

  it("keeps a TikTok like-aggregate row fading on the original row's clock", async () => {
    render(<OBSOverlayPage params={resolvedParams('test')} />)
    await act(async () => {})
    const aggregate = (id: string, likes: number, isUpdate: boolean): ChatMessage => ({
      ...chatMessage(id, 'first'),
      event: {
        type: 'like_aggregate',
        tier: 'low',
        duration: 0,
        is_update: isUpdate,
        aggregation_id: 'agg-1',
        metadata: { like_count: likes },
      },
    })
    // Original aggregate row, like the TikTok normalizer emits it: an event
    // carrying the aggregation id, not a plain chat message.
    onChat()(aggregate('m1', 1, false))
    await act(async () => {})

    // Aggregate updates REPLACE the row under a new id. Without arrival
    // inheritance each replacement restarted the visual row's fade clock and
    // a hot stream kept it on screen forever — the same failure mode the
    // head-of-queue timer had.
    for (let likes = 2; likes <= 5; likes++) {
      await act(async () => {
        await vi.advanceTimersByTimeAsync(1_000)
      })
      streamOptions.onMessageUpdate?.(aggregate(`m1-u${likes}`, likes, true))
      await act(async () => {})
    }

    // 4s of like refreshes replaced the row's id four times, but the clock
    // still started at m1's arrival: the row must be gone by t=8s (low tier
    // duration), not 8s after the LAST update.
    await act(async () => {
      await vi.advanceTimersByTimeAsync(4_000)
    })
    expect(screen.queryByText('first')).not.toBeInTheDocument()
  })

  it('does not mass-expire the visible feed when fade is re-enabled mid-session', async () => {
    const { rerender } = render(<OBSOverlayPage params={resolvedParams('test')} />)
    await act(async () => {})
    onChat()(chatMessage('m1', 'first'))
    await act(async () => {})
    onChat()(chatMessage('m2', 'second'))
    await act(async () => {})

    // Fade off for 100s (config refresh): rows stay put, and their old arrival
    // stamps go stale. Without the re-stamp on the disabled→enabled toggle the
    // first sweep after re-enabling would clear the whole feed at once.
    const { display_settings: display } = streamConfig.config as {
      display_settings: Record<string, unknown>
    }
    streamConfig.config = {
      ...streamConfig.config,
      display_settings: { ...display, disable_message_fade: true },
    }
    // A re-render delivers the mutated config, like the real 30s refresh.
    rerender(<OBSOverlayPage params={resolvedParams('test')} />)
    await act(async () => {
      await vi.advanceTimersByTimeAsync(100_000)
    })
    expect(screen.getByText('first')).toBeInTheDocument()
    expect(screen.getByText('second')).toBeInTheDocument()

    streamConfig.config = {
      ...streamConfig.config,
      display_settings: { ...display, disable_message_fade: false },
    }
    rerender(<OBSOverlayPage params={resolvedParams('test')} />)
    await act(async () => {
      await vi.advanceTimersByTimeAsync(9_000)
    })
    // Rows survive the 9s after re-enable; the re-stamped clock means they
    // leave at toggle+10s, not at their original arrival+10s.
    expect(screen.getByText('first')).toBeInTheDocument()
    await act(async () => {
      await vi.advanceTimersByTimeAsync(1_000)
    })
    expect(screen.queryByText('first')).not.toBeInTheDocument()
    expect(screen.queryByText('second')).not.toBeInTheDocument()
  })

  it('re-arms the timer from existing arrivals when duration shrinks mid-session', async () => {
    const { rerender } = render(<OBSOverlayPage params={resolvedParams('test')} />)
    await act(async () => {})
    onChat()(chatMessage('m1', 'first'))
    await act(async () => {})

    // 3s after arrival the config refresh shrinks duration 10s → 2s: the row
    // is already past its new deadline, so the overdue clamp must sweep it on
    // the next tick rather than re-arming a fresh 2s timer from now.
    await act(async () => {
      await vi.advanceTimersByTimeAsync(3_000)
    })
    const { display_settings: currentDisplay } = streamConfig.config as {
      display_settings: Record<string, unknown>
    }
    streamConfig.config = {
      ...streamConfig.config,
      display_settings: { ...currentDisplay, message_duration: 2 },
    }
    rerender(<OBSOverlayPage params={resolvedParams('test')} />)
    await act(async () => {
      await vi.advanceTimersByTimeAsync(0)
    })
    expect(screen.queryByText('first')).not.toBeInTheDocument()
  })
})
