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
import '@testing-library/jest-dom/vitest'
import { act, cleanup, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import OBSOverlayPage from '@/app/overlay/[id]/page'
import type { UseOverlayStreamOptions } from '@/hooks/useOverlayStream'
import type { ChatMessage } from '@/lib/types/message'

// The render route dispatches on the overlay's kind (ADR-0064), read from the
// public config the page already fetches. Chat overlays must keep rendering
// exactly as before; the three new kinds show a placeholder until their own
// renderer issues land.

window.matchMedia = (query: string) =>
  ({
    matches: false,
    media: query,
    addEventListener: () => {},
    removeEventListener: () => {},
  }) as unknown as MediaQueryList
Element.prototype.scrollIntoView = () => {}

const streamOptions: UseOverlayStreamOptions = {}
const streamConfig: { config: Record<string, unknown> } = { config: {} }

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

describe('overlay render route kind dispatch', () => {
  beforeEach(() => {
    streamConfig.config = {}
  })

  afterEach(() => cleanup())

  it('renders the chat feed when the kind is chat or absent', async () => {
    streamConfig.config = { overlay_type: 'chat' }
    render(<OBSOverlayPage params={resolvedParams('test')} />)
    await act(async () => {})

    act(() => {
      streamOptions.onChat?.(chatMessage('m1', 'still a chat overlay'))
    })

    expect(screen.getByText('still a chat overlay')).toBeInTheDocument()
  })

  it('renders the chat feed for a config that predates overlay_type', async () => {
    // An overlay created before the column existed — and the first render
    // before the config fetch resolves — must behave identically.
    streamConfig.config = { display_settings: {} }
    render(<OBSOverlayPage params={resolvedParams('test')} />)
    await act(async () => {})

    act(() => {
      streamOptions.onChat?.(chatMessage('m2', 'legacy shape'))
    })

    expect(screen.getByText('legacy shape')).toBeInTheDocument()
  })

  it.each(['alerts', 'goal', 'list'] as const)(
    'renders the placeholder for a %s overlay',
    async (kind) => {
      streamConfig.config = { overlay_type: kind }
      render(<OBSOverlayPage params={resolvedParams('test')} />)
      await act(async () => {})

      expect(screen.getByText(/not supported yet/i)).toBeInTheDocument()
    },
  )
})
