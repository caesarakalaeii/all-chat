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

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import type { ChatMessage } from '../types/message'

// Mock fetch to simulate the Twitch badge API response.
// The global "premium" badge set is intentionally included — this is the
// Amazon Prime Gaming badge that was previously overwriting the AllChat
// premium badge icon_url with a crown icon URL.
const MOCK_GLOBAL_BADGE_RESPONSE = {
  badge_sets: {
    premium: {
      versions: {
        '1': {
          id: '1',
          image_url_1x: 'https://cdn.twitch.tv/badges/prime/1x.png',
          image_url_2x: 'https://cdn.twitch.tv/badges/prime/2x.png',
          image_url_4x: 'https://cdn.twitch.tv/badges/prime/4x.png',
        },
      },
    },
    moderator: {
      versions: {
        '1': {
          id: '1',
          image_url_1x: 'https://cdn.twitch.tv/badges/mod/1x.png',
        },
      },
    },
  },
}

function makeTwitchMessage(badges: { name: string; version: string; icon_url: string }[]): ChatMessage {
  return {
    id: 'test-id',
    overlay_id: 'overlay-1',
    platform: 'twitch',
    channel_id: 'channel-1',
    channel_name: 'testchannel',
    user: {
      id: 'user-1',
      username: 'testuser',
      display_name: 'TestUser',
      badges,
    },
    message: { text: 'hello', emotes: [] },
    timestamp: '2026-01-01T00:00:00Z',
    metadata: { twitch_room_id: '12345' },
  }
}

describe('resolveTwitchBadgeIcons', () => {
  beforeEach(() => {
    vi.resetModules()
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(MOCK_GLOBAL_BADGE_RESPONSE),
    }))
  })

  afterEach(() => {
    // Always restore real timers so a fake-timer test can't leak into others.
    vi.useRealTimers()
  })

  // Simulates the browser fetch abort contract: the request never settles on its
  // own, but rejects with an AbortError as soon as its AbortSignal is aborted.
  // This is what makes fetchBadgeSets' timeout observable in tests.
  function makeHangingFetch() {
    return vi.fn((_url: string, options?: { signal?: AbortSignal }) => {
      return new Promise((_resolve, reject) => {
        options?.signal?.addEventListener('abort', () => {
          reject(new DOMException('The operation was aborted.', 'AbortError'))
        })
      })
    })
  }

  it('does not overwrite AllChat allchat badge icon_url with Twitch API data', async () => {
    const { resolveTwitchBadgeIcons } = await import('../twitchBadges')

    const message = makeTwitchMessage([
      { name: 'allchat', version: '1', icon_url: '' },
    ])

    const result = await resolveTwitchBadgeIcons(message)

    const allchatBadge = result.user.badges.find((b) => b.name === 'allchat')
    expect(allchatBadge).toBeDefined()
    expect(allchatBadge!.icon_url).toBe('')
  })

  it('does not overwrite AllChat premium badge icon_url with Twitch API data', async () => {
    const { resolveTwitchBadgeIcons } = await import('../twitchBadges')

    const message = makeTwitchMessage([
      { name: 'allchat-premium', version: '1', icon_url: '' },
    ])

    const result = await resolveTwitchBadgeIcons(message)

    const premiumBadge = result.user.badges.find((b) => b.name === 'allchat-premium')
    expect(premiumBadge).toBeDefined()
    // icon_url must remain empty — AllChat premium renders via PremiumBadge component
    expect(premiumBadge!.icon_url).toBe('')
  })

  it('resolves Twitch Prime Gaming badge (premium) normally', async () => {
    const { resolveTwitchBadgeIcons } = await import('../twitchBadges')

    const message = makeTwitchMessage([
      { name: 'premium', version: '1', icon_url: '' },
    ])

    const result = await resolveTwitchBadgeIcons(message)

    const primeBadge = result.user.badges.find((b) => b.name === 'premium')
    expect(primeBadge).toBeDefined()
    // Twitch "premium" badge should now resolve to the Prime Gaming crown
    expect(primeBadge!.icon_url).toContain('cdn.twitch.tv')
  })

  it('still resolves regular Twitch badges (e.g. moderator) normally', async () => {
    const { resolveTwitchBadgeIcons } = await import('../twitchBadges')

    const message = makeTwitchMessage([
      { name: 'moderator', version: '1', icon_url: '' },
    ])

    const result = await resolveTwitchBadgeIcons(message)

    const modBadge = result.user.badges.find((b) => b.name === 'moderator')
    expect(modBadge).toBeDefined()
    expect(modBadge!.icon_url).toBe('https://cdn.twitch.tv/badges/mod/1x.png')
  })

  it('handles a message with both AllChat badges and regular Twitch badges', async () => {
    const { resolveTwitchBadgeIcons } = await import('../twitchBadges')

    const message = makeTwitchMessage([
      { name: 'allchat', version: '1', icon_url: '' },
      { name: 'allchat-premium', version: '1', icon_url: '' },
      { name: 'moderator', version: '1', icon_url: '' },
    ])

    const result = await resolveTwitchBadgeIcons(message)

    const allchatBadge = result.user.badges.find((b) => b.name === 'allchat')
    const premiumBadge = result.user.badges.find((b) => b.name === 'allchat-premium')
    const modBadge = result.user.badges.find((b) => b.name === 'moderator')

    expect(allchatBadge!.icon_url).toBe('')
    expect(premiumBadge!.icon_url).toBe('')
    expect(modBadge!.icon_url).toBe('https://cdn.twitch.tv/badges/mod/1x.png')
  })

  it('skips non-twitch messages entirely', async () => {
    const { resolveTwitchBadgeIcons } = await import('../twitchBadges')

    const message: ChatMessage = {
      ...makeTwitchMessage([{ name: 'premium', version: '1', icon_url: '' }]),
      platform: 'youtube',
    }

    const result = await resolveTwitchBadgeIcons(message)
    expect(result).toBe(message) // exact same reference — no copy made
  })

  it('does not wedge when the badge fetch hangs — it aborts after the timeout and resolves', async () => {
    vi.useFakeTimers()
    const hangingFetch = makeHangingFetch()
    vi.stubGlobal('fetch', hangingFetch)

    const { resolveTwitchBadgeIcons, BADGE_FETCH_TIMEOUT_MS } = await import('../twitchBadges')

    const message = makeTwitchMessage([{ name: 'moderator', version: '1', icon_url: '' }])
    const resultPromise = resolveTwitchBadgeIcons(message)

    // Drive the timeout deterministically past the abort deadline.
    await vi.advanceTimersByTimeAsync(BADGE_FETCH_TIMEOUT_MS + 1)

    // The call must resolve (not hang forever) even though fetch never responded.
    const result = await resultPromise
    expect(result).toBeDefined()

    // Resolution failed, so badges are returned unmodified (icon_url stays empty).
    const modBadge = result.user.badges.find((b) => b.name === 'moderator')
    expect(modBadge).toBeDefined()
    expect(modBadge!.icon_url).toBe('')

    // fetch must have been given an AbortSignal so the timeout can cancel it.
    expect(hangingFetch).toHaveBeenCalled()
    const [, options] = hangingFetch.mock.calls[0]
    expect(options).toEqual(expect.objectContaining({ signal: expect.anything() }))
  })

  it('self-heals after a hang: the next message issues a fresh fetch and resolves badges', async () => {
    vi.useFakeTimers()
    const hangingFetch = makeHangingFetch()
    vi.stubGlobal('fetch', hangingFetch)

    const { resolveTwitchBadgeIcons, BADGE_FETCH_TIMEOUT_MS } = await import('../twitchBadges')

    // First message hangs, then times out.
    const firstMessage = makeTwitchMessage([{ name: 'moderator', version: '1', icon_url: '' }])
    const firstPromise = resolveTwitchBadgeIcons(firstMessage)
    await vi.advanceTimersByTimeAsync(BADGE_FETCH_TIMEOUT_MS + 1)
    const firstResult = await firstPromise
    expect(firstResult.user.badges.find((b) => b.name === 'moderator')!.icon_url).toBe('')

    // A healthy fetch is now available. Because the timed-out request did not
    // poison inflightRequests (or badgeCache), the next call must issue a NEW
    // fetch and successfully resolve the badge icon.
    const healthyFetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(MOCK_GLOBAL_BADGE_RESPONSE),
    })
    vi.stubGlobal('fetch', healthyFetch)

    const secondMessage = makeTwitchMessage([{ name: 'moderator', version: '1', icon_url: '' }])
    const secondResult = await resolveTwitchBadgeIcons(secondMessage)

    const modBadge = secondResult.user.badges.find((b) => b.name === 'moderator')
    expect(modBadge!.icon_url).toBe('https://cdn.twitch.tv/badges/mod/1x.png')
    expect(healthyFetch).toHaveBeenCalled()
  })
})
