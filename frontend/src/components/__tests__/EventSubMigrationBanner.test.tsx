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
import { render, screen, cleanup, fireEvent, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import {
  EventSubMigrationBanner,
  getMigratableChannels,
} from '@/components/EventSubMigrationBanner'
import type { ChatSource } from '@/lib/types/overlay'

// jsdom in this project does not ship a working localStorage; stub one.
function stubLocalStorage() {
  const store: Record<string, string> = {}
  vi.stubGlobal('localStorage', {
    getItem: (k: string) => store[k] ?? null,
    setItem: (k: string, v: string) => {
      store[k] = v
    },
    removeItem: (k: string) => {
      delete store[k]
    },
    clear: () => {
      Object.keys(store).forEach((k) => delete store[k])
    },
  })
}

function src(overrides: Partial<ChatSource>): ChatSource {
  return {
    id: Math.random().toString(36),
    overlay_id: 'o1',
    platform: 'twitch',
    channel_id: 'caesarlp',
    created_at: '',
    updated_at: '',
    is_active: true,
    ...overrides,
  }
}

beforeEach(() => stubLocalStorage())

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
})

describe('getMigratableChannels', () => {
  it('includes only owned twitch channels still on IRC', () => {
    const result = getMigratableChannels({
      o1: [
        src({ channel_id: 'mine', is_own_channel: true, chat_via_eventsub: false }), // migratable
        src({ channel_id: 'already', is_own_channel: true, chat_via_eventsub: true }), // already on eventsub
        src({ channel_id: 'notmine', is_own_channel: false, chat_via_eventsub: false }), // not owned
        src({ channel_id: 'yt', platform: 'youtube', is_own_channel: true }), // not twitch
      ],
    })
    expect(result.map((c) => c.channelId)).toEqual(['mine'])
    expect(result[0].overlayId).toBe('o1')
  })

  it('dedupes the same channel across overlays', () => {
    const result = getMigratableChannels({
      o1: [src({ channel_id: 'Mine', is_own_channel: true, chat_via_eventsub: false })],
      o2: [src({ channel_id: 'mine', is_own_channel: true, chat_via_eventsub: false })],
    })
    expect(result).toHaveLength(1)
  })
})

describe('EventSubMigrationBanner', () => {
  it('renders nothing when no owned IRC channels exist', () => {
    const { container } = render(
      <EventSubMigrationBanner
        sourcesByOverlay={{
          o1: [src({ is_own_channel: true, chat_via_eventsub: true })],
        }}
      />
    )
    expect(container).toBeEmptyDOMElement()
  })

  it('renders a nudge when an owned channel is still on IRC', () => {
    render(
      <EventSubMigrationBanner
        sourcesByOverlay={{ o1: [src({ is_own_channel: true, chat_via_eventsub: false })] }}
      />
    )
    expect(screen.getByText(/legacy connection/i)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /reconnect now/i })).toBeInTheDocument()
  })

  it('starts the add-source reflow on reconnect', async () => {
    const fetchMock = vi
      .spyOn(globalThis, 'fetch')
      .mockResolvedValue(
        new Response(JSON.stringify({ auth_url: 'https://id.twitch.tv/oauth' }), { status: 200 })
      )
    render(
      <EventSubMigrationBanner
        sourcesByOverlay={{ o9: [src({ is_own_channel: true, chat_via_eventsub: false })] }}
      />
    )
    fireEvent.click(screen.getByRole('button', { name: /reconnect now/i }))
    await waitFor(() =>
      // Routed through apiClient: same-origin request (origin-prefixed URL) with
      // credentials so the httpOnly access cookie is sent (H3 cookie auth). No
      // Authorization header — the gateway derives the bearer via CookieToBearer.
      expect(fetchMock).toHaveBeenCalledWith(
        expect.stringContaining('/api/v1/auth/twitch/add-source/o9'),
        expect.objectContaining({ credentials: 'same-origin' })
      )
    )
  })

  it('stays dismissed after the dismiss button is clicked', () => {
    const { container, rerender } = render(
      <EventSubMigrationBanner
        sourcesByOverlay={{ o1: [src({ is_own_channel: true, chat_via_eventsub: false })] }}
      />
    )
    fireEvent.click(screen.getByRole('button', { name: /dismiss/i }))
    expect(container).toBeEmptyDOMElement()
    expect(window.localStorage.getItem('eventsub-migration-banner-dismissed')).toBe('1')

    // A fresh mount honors the persisted dismissal.
    rerender(
      <EventSubMigrationBanner
        sourcesByOverlay={{ o1: [src({ is_own_channel: true, chat_via_eventsub: false })] }}
      />
    )
    expect(screen.queryByText(/legacy connection/i)).not.toBeInTheDocument()
  })
})
