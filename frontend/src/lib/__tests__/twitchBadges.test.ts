import { describe, it, expect, vi, beforeEach } from 'vitest'
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

  it('does not overwrite AllChat premium badge icon_url with Twitch Prime Gaming crown', async () => {
    const { resolveTwitchBadgeIcons } = await import('../twitchBadges')

    const message = makeTwitchMessage([
      { name: 'premium', version: '1', icon_url: '' },
    ])

    const result = await resolveTwitchBadgeIcons(message)

    const premiumBadge = result.user.badges.find((b) => b.name === 'premium')
    expect(premiumBadge).toBeDefined()
    // icon_url must remain empty — AllChat premium renders via PremiumBadge component
    expect(premiumBadge!.icon_url).toBe('')
    // Must not be set to the Twitch prime crown URL
    expect(premiumBadge!.icon_url).not.toContain('cdn.twitch.tv')
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
      { name: 'premium', version: '1', icon_url: '' },
      { name: 'moderator', version: '1', icon_url: '' },
    ])

    const result = await resolveTwitchBadgeIcons(message)

    const allchatBadge = result.user.badges.find((b) => b.name === 'allchat')
    const premiumBadge = result.user.badges.find((b) => b.name === 'premium')
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
})
