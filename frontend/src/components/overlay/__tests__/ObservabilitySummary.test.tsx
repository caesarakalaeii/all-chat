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
import { render, screen, cleanup } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'

import type { SourceInfo } from '@/components/PlatformStatusIndicators'
import { ObservabilitySummary } from '@/components/overlay/ObservabilitySummary'
import type { EventSettings, PublicOverlayConfig } from '@/lib/types/overlay'

afterEach(() => cleanup())

const sources = new Map<string, SourceInfo>([
  ['c1', { platform: 'twitch', channelId: 'c1', channelName: 'caesar' }],
  ['c2', { platform: 'youtube', channelId: 'c2', channelName: 'caesarTube' }],
])

const config: PublicOverlayConfig = {
  filter_settings: {
    banned_words: ['a', 'b', 'c'],
    banned_users: ['x'],
    min_message_length: 2,
    hide_commands: true,
  },
  seventv_emote_set_id: 'set-123',
}

function eventSettings(over: Partial<EventSettings> = {}): EventSettings {
  return {
    id: 'e1',
    overlay_id: 'o1',
    enable_twitch_subs: true,
    enable_twitch_resubs: false,
    enable_twitch_gift_subs: false,
    enable_twitch_bits: true,
    enable_twitch_raids: false,
    enable_twitch_channel_points: false,
    enable_twitch_follows: false,
    enable_twitch_watch_streaks: true,
    enable_youtube_super_chat: false,
    enable_youtube_super_sticker: false,
    enable_youtube_members: false,
    enable_youtube_member_milestones: false,
    enable_youtube_member_gifts: false,
    enable_kick_subs: false,
    enable_kick_gifts: false,
    enable_tiktok_likes: false,
    enable_tiktok_gifts: false,
    enable_tiktok_follows: false,
    enable_tiktok_shares: false,
    enable_token_warnings: true,
    tiktok_like_aggregation_window_seconds: 5,
    event_display_duration_multiplier: 1,
    ...over,
  }
}

describe('ObservabilitySummary', () => {
  it('lists configured sources with live/idle status', () => {
    render(
      <ObservabilitySummary
        config={config}
        sources={sources}
        activeChannels={new Set(['c1'])}
        eventSettings={eventSettings()}
        observedEventTypes={new Set()}
      />
    )
    expect(screen.getByText('caesar')).toBeInTheDocument()
    expect(screen.getByText('caesarTube')).toBeInTheDocument()
    expect(screen.getByText('live')).toBeInTheDocument()
    expect(screen.getByText('idle')).toBeInTheDocument()
  })

  it('renders the full event-toggle list when event settings are available', () => {
    render(
      <ObservabilitySummary
        config={config}
        sources={sources}
        activeChannels={new Set()}
        eventSettings={eventSettings()}
        observedEventTypes={new Set()}
      />
    )
    expect(screen.getByText('Twitch Subs')).toBeInTheDocument()
    expect(screen.getByText('Twitch Bits')).toBeInTheDocument()
    expect(screen.getByText('Token Warnings')).toBeInTheDocument()
  })

  it('falls back to observed event types when event settings are unavailable', () => {
    render(
      <ObservabilitySummary
        config={config}
        sources={sources}
        activeChannels={new Set()}
        eventSettings={null}
        observedEventTypes={new Set(['subscription', 'bits'])}
      />
    )
    expect(screen.getByText('subscription')).toBeInTheDocument()
    expect(screen.getByText('bits')).toBeInTheDocument()
    expect(screen.queryByText('Twitch Subs')).not.toBeInTheDocument()
  })

  it('summarizes filter settings and the 7TV set', () => {
    render(
      <ObservabilitySummary
        config={config}
        sources={sources}
        activeChannels={new Set()}
        eventSettings={null}
        observedEventTypes={new Set()}
      />
    )
    expect(screen.getByText('set-123')).toBeInTheDocument()
    expect(screen.getByText('Banned words')).toBeInTheDocument()
    // banned_words has 3 entries
    expect(screen.getByText('3')).toBeInTheDocument()
  })
})
