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

import { describe, it, expect } from 'vitest'
import { shouldFilterMessage, SAY_HI_PHRASES } from '../filterMessage'
import type { ChatMessage } from '@/lib/types/message'
import type { FilterSettings } from '@/lib/types/overlay'

function makeMsg(
  overrides: {
    username?: string
    display_name?: string
    text?: string
    platform?: ChatMessage['platform']
  } = {}
): ChatMessage {
  return {
    id: '1',
    overlay_id: 'ov1',
    platform: overrides.platform ?? 'twitch',
    channel_id: 'ch1',
    channel_name: 'test',
    user: {
      id: 'u1',
      username: overrides.username ?? 'testuser',
      display_name: overrides.display_name ?? 'TestUser',
      badges: [],
    },
    message: { text: overrides.text ?? 'hello world', emotes: [] },
    timestamp: new Date().toISOString(),
    metadata: {},
  }
}

describe('shouldFilterMessage', () => {
  // Null / undefined / empty settings — nothing should be filtered
  it('returns false when settings is null', () => {
    expect(shouldFilterMessage(makeMsg(), null)).toBe(false)
  })

  it('returns false when settings is undefined', () => {
    expect(shouldFilterMessage(makeMsg(), undefined)).toBe(false)
  })

  it('returns false when settings is empty object {}', () => {
    expect(shouldFilterMessage(makeMsg(), {})).toBe(false)
  })

  // banned_users — exact case-insensitive username match
  it('returns true when message.user.username matches banned_users entry (case-insensitive)', () => {
    const settings: FilterSettings = { banned_users: ['Nightbot'] }
    expect(shouldFilterMessage(makeMsg({ username: 'nightbot' }), settings)).toBe(true)
  })

  it('returns true when message.user.display_name matches banned_users entry (case-insensitive)', () => {
    const settings: FilterSettings = { banned_users: ['nightbot'] }
    expect(shouldFilterMessage(makeMsg({ display_name: 'NightBot' }), settings)).toBe(true)
  })

  it('returns false when banned_users entry is substring of username but not exact match', () => {
    const settings: FilterSettings = { banned_users: ['nightbot'] }
    expect(shouldFilterMessage(makeMsg({ username: 'nightbot_fan' }), settings)).toBe(false)
  })

  // banned_words — regex matching against message text
  it('returns true when message text matches a banned_words regex pattern (case-insensitive)', () => {
    const settings: FilterSettings = { banned_words: ['follow me'] }
    expect(shouldFilterMessage(makeMsg({ text: 'FOLLOW ME on twitter' }), settings)).toBe(true)
  })

  it('returns true when banned_words contains a regex pattern and message text matches', () => {
    const settings: FilterSettings = { banned_words: ['buy\\s+followers'] }
    expect(shouldFilterMessage(makeMsg({ text: 'buy followers now' }), settings)).toBe(true)
  })

  it('returns false when banned_words regex does not match', () => {
    const settings: FilterSettings = { banned_words: ['buy\\s+followers'] }
    expect(shouldFilterMessage(makeMsg({ text: 'hello world' }), settings)).toBe(false)
  })

  it('silently skips invalid regex without throwing and returns false if no other filter matches', () => {
    const settings: FilterSettings = { banned_words: ['[invalid'] }
    expect(() => shouldFilterMessage(makeMsg(), settings)).not.toThrow()
    expect(shouldFilterMessage(makeMsg(), settings)).toBe(false)
  })

  // hide_commands — messages starting with !
  it('returns true when hide_commands is true and message text starts with "!"', () => {
    const settings: FilterSettings = { hide_commands: true }
    expect(shouldFilterMessage(makeMsg({ text: '!lurk' }), settings)).toBe(true)
  })

  it('returns false when hide_commands is true and message text does not start with "!"', () => {
    const settings: FilterSettings = { hide_commands: true }
    expect(shouldFilterMessage(makeMsg({ text: 'hello!' }), settings)).toBe(false)
  })

  it('returns false when hide_commands is false and message starts with "!"', () => {
    const settings: FilterSettings = { hide_commands: false }
    expect(shouldFilterMessage(makeMsg({ text: '!lurk' }), settings)).toBe(false)
  })

  // min_message_length
  it('returns true when min_message_length is 5 and message text is shorter', () => {
    const settings: FilterSettings = { min_message_length: 5 }
    expect(shouldFilterMessage(makeMsg({ text: 'hi' }), settings)).toBe(true)
  })

  it('returns false when min_message_length is 0 (disabled)', () => {
    const settings: FilterSettings = { min_message_length: 0 }
    expect(shouldFilterMessage(makeMsg({ text: 'hi' }), settings)).toBe(false)
  })

  it('returns false when min_message_length is 5 and message text is exactly 5 chars (not shorter)', () => {
    const settings: FilterSettings = { min_message_length: 5 }
    expect(shouldFilterMessage(makeMsg({ text: 'hello' }), settings)).toBe(false)
  })

  // Edge case: missing user/message fields
  it('handles missing user/message gracefully without crashing', () => {
    const settings: FilterSettings = {
      banned_users: ['bot'],
      banned_words: ['spam'],
      hide_commands: true,
      min_message_length: 3,
    }
    const broken = {
      id: '1',
      overlay_id: 'ov1',
      platform: 'twitch' as const,
      channel_id: 'ch1',
      channel_name: 'test',
      user: {} as ChatMessage['user'],
      message: {} as ChatMessage['message'],
      timestamp: new Date().toISOString(),
      metadata: {},
    }
    expect(() => shouldFilterMessage(broken, settings)).not.toThrow()
  })

  // hide_youtube_say_hi — YouTube's free "Say hi!" button posts a plain "said hi" message
  it('returns true for a youtube "said hi" message when hide_youtube_say_hi is true', () => {
    const settings: FilterSettings = { hide_youtube_say_hi: true }
    expect(shouldFilterMessage(makeMsg({ platform: 'youtube', text: 'said hi' }), settings)).toBe(
      true
    )
  })

  it('returns true for a say-hi message with surrounding whitespace', () => {
    const settings: FilterSettings = { hide_youtube_say_hi: true }
    expect(
      shouldFilterMessage(makeMsg({ platform: 'youtube', text: '  said hi  ' }), settings)
    ).toBe(true)
  })

  it('returns true for a say-hi message in mixed case', () => {
    const settings: FilterSettings = { hide_youtube_say_hi: true }
    expect(shouldFilterMessage(makeMsg({ platform: 'youtube', text: 'Said Hi' }), settings)).toBe(
      true
    )
  })

  it('returns false for "said hi!" because trailing punctuation is not a built-in phrase', () => {
    const settings: FilterSettings = { hide_youtube_say_hi: true }
    expect(shouldFilterMessage(makeMsg({ platform: 'youtube', text: 'said hi!' }), settings)).toBe(
      false
    )
  })

  it('returns true for "said hi!" when it is listed in say_hi_extra_phrases', () => {
    const settings: FilterSettings = {
      hide_youtube_say_hi: true,
      say_hi_extra_phrases: ['said hi!'],
    }
    expect(shouldFilterMessage(makeMsg({ platform: 'youtube', text: 'said hi!' }), settings)).toBe(
      true
    )
  })

  it('returns false when a say-hi phrase is only a substring of the message', () => {
    const settings: FilterSettings = { hide_youtube_say_hi: true }
    expect(
      shouldFilterMessage(
        makeMsg({ platform: 'youtube', text: 'my friend said hi to you' }),
        settings
      )
    ).toBe(false)
  })

  it('returns false for a non-youtube "said hi" message even when the toggle is on', () => {
    const settings: FilterSettings = { hide_youtube_say_hi: true }
    expect(shouldFilterMessage(makeMsg({ platform: 'twitch', text: 'said hi' }), settings)).toBe(
      false
    )
  })

  it('returns false for a youtube "said hi" message when the toggle is absent', () => {
    expect(shouldFilterMessage(makeMsg({ platform: 'youtube', text: 'said hi' }), {})).toBe(false)
  })

  it('returns false for a youtube "said hi" message when the toggle is false', () => {
    const settings: FilterSettings = { hide_youtube_say_hi: false }
    expect(shouldFilterMessage(makeMsg({ platform: 'youtube', text: 'said hi' }), settings)).toBe(
      false
    )
  })

  it('returns true for a localised say-hi phrase supplied via say_hi_extra_phrases', () => {
    const settings: FilterSettings = {
      hide_youtube_say_hi: true,
      say_hi_extra_phrases: ['hat hallo gesagt'],
    }
    expect(
      shouldFilterMessage(makeMsg({ platform: 'youtube', text: 'Hat Hallo gesagt' }), settings)
    ).toBe(true)
  })

  it('ignores a whitespace-only say_hi_extra_phrases entry', () => {
    const settings: FilterSettings = { hide_youtube_say_hi: true, say_hi_extra_phrases: ['   '] }
    expect(shouldFilterMessage(makeMsg({ platform: 'youtube', text: '' }), settings)).toBe(false)
    expect(shouldFilterMessage(makeMsg({ platform: 'youtube', text: 'anything' }), settings)).toBe(
      false
    )
  })

  it('ships exactly one built-in say-hi phrase', () => {
    expect(SAY_HI_PHRASES).toHaveLength(1)
    expect(SAY_HI_PHRASES).toContain('said hi')
  })
})
