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
import { channelUrl } from '../platform-channel-url'

describe('channelUrl', () => {
  it('builds a Twitch URL from the login (lowercased)', () => {
    expect(channelUrl('twitch', 'XQC')).toBe('https://twitch.tv/xqc')
  })

  it('builds a Kick URL from the slug', () => {
    expect(channelUrl('kick', 'xqc')).toBe('https://kick.com/xqc')
  })

  it('builds a TikTok URL, tolerating a leading @', () => {
    expect(channelUrl('tiktok', 'someone')).toBe('https://www.tiktok.com/@someone')
    expect(channelUrl('tiktok', '@someone')).toBe('https://www.tiktok.com/@someone')
  })

  it('prefers a YouTube @handle when present', () => {
    expect(channelUrl('youtube', 'UCsomethingignored0000000', '@MrBeast')).toBe(
      'https://www.youtube.com/@MrBeast'
    )
    expect(channelUrl('youtube', '', 'MrBeast')).toBe('https://www.youtube.com/@MrBeast')
  })

  it('builds a YouTube /channel/ URL only for a canonical UC… id', () => {
    expect(channelUrl('youtube', 'UC-lHJZR3Gqxm24_Vd_AJ5Yw')).toBe(
      'https://www.youtube.com/channel/UC-lHJZR3Gqxm24_Vd_AJ5Yw'
    )
  })

  it('returns null for a YouTube id that is not a UC… channel id and no handle', () => {
    // e.g. a legacy row holding a handle/display in channel_id, or a Google account id.
    expect(channelUrl('youtube', 'someLegacyValue')).toBeNull()
    expect(channelUrl('youtube', '')).toBeNull()
  })

  it('returns null for non-addressable platforms', () => {
    expect(channelUrl('discord', '123456789012345678')).toBeNull()
    expect(channelUrl('shared_overlay', 'some-overlay-uuid')).toBeNull()
    expect(channelUrl('unknown', 'x')).toBeNull()
  })

  it('returns null when the identifier is empty', () => {
    expect(channelUrl('twitch', '')).toBeNull()
    expect(channelUrl('kick', '   ')).toBeNull()
  })
})
