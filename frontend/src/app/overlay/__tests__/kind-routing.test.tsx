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

// The render route dispatches on the overlay's kind (ADR-0068), read from the
// public config the page already fetches. Chat overlays must keep rendering
// exactly as before; alerts overlays render the alert box; goal and list show
// a placeholder until their own renderer issues land.

window.matchMedia = (query: string) =>
  ({
    matches: false,
    media: query,
    addEventListener: () => {},
    removeEventListener: () => {},
  }) as unknown as MediaQueryList
Element.prototype.scrollIntoView = () => {}

const streamOptions: UseOverlayStreamOptions = {}
const streamConfig: { config: Record<string, unknown> | null } = { config: {} }

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

  it('renders an alert frame on an alerts overlay, and nothing before one', async () => {
    streamConfig.config = { overlay_type: 'alerts' }
    const { container } = render(<OBSOverlayPage params={resolvedParams('test')} />)
    await act(async () => {})
    expect(container).toBeEmptyDOMElement()

    act(() => {
      streamOptions.onAlert?.({
        alert_id: 'a1',
        overlay_id: 'test',
        platform: 'twitch',
        event_type: 'bits',
        event_data: { type: 'bits' },
        user: { id: 'u', name: 'E2ECheck', avatar_url: '' },
        profile: {
          event_type: 'bits',
          text_template: '{username} cheered {amount}!',
          sound_url: '',
          image_url: '',
          duration_ms: 5000,
          volume: 100,
          enabled: true,
          sort_after: 0,
          text: 'E2ECheck cheered 100 bits!',
        },
      })
    })

    expect(screen.getByRole('status')).toHaveTextContent('E2ECheck cheered 100 bits!')
  })

  it('plays an alert that arrives before the config has loaded', async () => {
    // Regression: the kind came only from the public config, so an alert that
    // arrived while the config fetch had failed (e.g. during a rollout) sat in
    // the queue unrendered until the 30s refresh. Only alerts/goal/list
    // sockets are ever sent alert frames (ADR-0070), so the frame itself says
    // the overlay renders alerts.
    streamConfig.config = null
    render(<OBSOverlayPage params={resolvedParams('test')} />)
    await act(async () => {})

    act(() => {
      streamOptions.onAlert?.({
        alert_id: 'early',
        overlay_id: 'test',
        platform: 'twitch',
        event_type: 'follow',
        event_data: { type: 'follow' },
        user: { id: 'u', name: 'FirstFollower', avatar_url: '' },
        profile: {
          event_type: 'follow',
          text_template: '{username} just followed!',
          sound_url: '',
          image_url: '',
          duration_ms: 5000,
          volume: 100,
          enabled: true,
          sort_after: 0,
          text: 'FirstFollower just followed!',
        },
      })
    })

    expect(screen.getByRole('status')).toHaveTextContent('FirstFollower just followed!')
  })

  it.each(['goal', 'list'] as const)('renders the placeholder for a %s overlay', async (kind) => {
    streamConfig.config = { overlay_type: kind }
    render(<OBSOverlayPage params={resolvedParams('test')} />)
    await act(async () => {})

    expect(screen.getByText(/not supported yet/i)).toBeInTheDocument()
  })
})
