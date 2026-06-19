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

import { afterEach, describe, expect, it, vi } from 'vitest'
import { preloadOverlayEmotes } from '@/lib/utils/preloadEmotes'

function mockFetch(byChannel: Record<string, { ok: boolean; emotes?: Array<{ url?: string }> }>) {
  return vi.fn(async (input: string | URL) => {
    const url = typeof input === 'string' ? input : input.toString()
    // Path is /api/v1/emotes/channel/<channel>?...
    const match = url.match(/\/emotes\/channel\/([^?]+)/)
    const channel = match ? decodeURIComponent(match[1]) : ''
    const entry = byChannel[channel]
    if (!entry) return { ok: false, json: async () => ({}) } as Response
    return {
      ok: entry.ok,
      json: async () => ({ emotes: entry.emotes ?? [] }),
    } as Response
  })
}

afterEach(() => {
  vi.restoreAllMocks()
})

describe('preloadOverlayEmotes', () => {
  it('fetches each channel and starts an image load per distinct emote URL', async () => {
    vi.stubGlobal(
      'fetch',
      mockFetch({
        '123': { ok: true, emotes: [{ url: 'https://cdn/a.webp' }, { url: 'https://cdn/b.webp' }] },
        xyz: { ok: true, emotes: [{ url: 'https://cdn/c.webp' }] },
      })
    )
    const loaded: string[] = []

    const count = await preloadOverlayEmotes(
      [
        { platform: 'twitch', channelId: '123' },
        { platform: 'kick', channelId: 'xyz' },
      ],
      { loadImage: (u) => loaded.push(u) }
    )

    expect(count).toBe(3)
    expect(loaded.sort()).toEqual(['https://cdn/a.webp', 'https://cdn/b.webp', 'https://cdn/c.webp'])
  })

  it('deduplicates URLs shared across channels', async () => {
    vi.stubGlobal(
      'fetch',
      mockFetch({
        a: { ok: true, emotes: [{ url: 'https://cdn/shared.webp' }] },
        b: { ok: true, emotes: [{ url: 'https://cdn/shared.webp' }] },
      })
    )
    const loaded: string[] = []

    const count = await preloadOverlayEmotes(
      [
        { platform: 'twitch', channelId: 'a' },
        { platform: 'twitch', channelId: 'b' },
      ],
      { loadImage: (u) => loaded.push(u) }
    )

    expect(count).toBe(1)
    expect(loaded).toEqual(['https://cdn/shared.webp'])
  })

  it('passes platform and the 7TV override as query params', async () => {
    const fetchSpy = mockFetch({ a: { ok: true, emotes: [] } })
    vi.stubGlobal('fetch', fetchSpy)

    await preloadOverlayEmotes([{ platform: 'youtube', channelId: 'a' }], {
      seventvSetId: 'set-42',
      loadImage: () => {},
    })

    const calledUrl = fetchSpy.mock.calls[0][0] as string
    expect(calledUrl).toContain('platform=youtube')
    expect(calledUrl).toContain('seventv_set_id=set-42')
  })

  it('honors the maxUrls cap', async () => {
    vi.stubGlobal(
      'fetch',
      mockFetch({
        a: {
          ok: true,
          emotes: [{ url: 'https://cdn/1' }, { url: 'https://cdn/2' }, { url: 'https://cdn/3' }],
        },
      })
    )
    let count = 0

    const started = await preloadOverlayEmotes([{ platform: 'twitch', channelId: 'a' }], {
      maxUrls: 2,
      loadImage: () => {
        count++
      },
    })

    expect(started).toBe(2)
    expect(count).toBe(2)
  })

  it('swallows fetch failures and skips channels without ids', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => {
        throw new Error('network down')
      })
    )
    const loaded: string[] = []

    const count = await preloadOverlayEmotes(
      [
        { platform: 'twitch', channelId: '' },
        { platform: 'twitch', channelId: 'a' },
      ],
      { loadImage: (u) => loaded.push(u) }
    )

    expect(count).toBe(0)
    expect(loaded).toEqual([])
  })
})
