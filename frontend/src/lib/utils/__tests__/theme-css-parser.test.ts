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
import { parseCssToVisualSettings } from '../theme-css-parser'

describe('parseCssToVisualSettings', () => {
  it('returns empty object for empty input', () => {
    expect(parseCssToVisualSettings('')).toEqual({})
  })

  it('returns single known var from CSS string', () => {
    const css = ":root { --chat-font-family: 'Inter', sans-serif; }"
    expect(parseCssToVisualSettings(css)).toEqual({ fontFamily: "'Inter', sans-serif" })
  })

  it('triggers console.warn for unknown var and excludes it from result', () => {
    const warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {})
    try {
      const css = ':root { --chat-unknown-xyz: red; }'
      const result = parseCssToVisualSettings(css)
      expect(result).toEqual({})
      expect(warnSpy).toHaveBeenCalledTimes(1)
      expect(warnSpy.mock.calls[0][0]).toContain('--chat-unknown-xyz')
    } finally {
      warnSpy.mockRestore()
    }
  })

  it('extracts all 50 known properties from a full theme CSS string', () => {
    const fullThemeCss = `
      :root {
        --chat-font-family: Inter;
        --chat-font-weight: 400;
        --chat-line-height: 1.5;
        --chat-letter-spacing: 0px;
        --chat-font-size: 16px;
        --chat-message-color: #ffffff;
        --chat-username-color: #aaaaaa;
        --chat-timestamp-color: #666666;
        --chat-username-font-family: Inter;
        --chat-timestamp-font-family: Georgia;
        --chat-username-font-weight: 600;
        --chat-username-font-size: 14px;
        --chat-timestamp-font-size: 11px;
        --chat-overlay-bg-color: #000000;
        --chat-overlay-bg-opacity: 0.5;
        --chat-overlay-padding: 16px;
        --chat-bubble-bg-color: #111111;
        --chat-bubble-bg-opacity: 0.9;
        --chat-bubble-border-radius: 8px;
        --chat-bubble-border-width: 1px;
        --chat-bubble-border-color: #333333;
        --chat-bubble-padding: 12px;
        --chat-bubble-shadow: none;
        --chat-message-gap: 8px;
        --chat-backdrop-blur: 0px;
        --chat-max-width: 100%;
        --chat-show-avatars: inline;
        --chat-show-badges: inline;
        --chat-show-timestamps: block;
        --chat-show-platform-badge: inline;
        --chat-show-emotes: inline;
        --chat-show-username: inline;
        --chat-avatar-size: 40px;
        --chat-badge-size: 18px;
        --chat-emote-scale: 1;
        --platform-twitch-accent: #9146ff;
        --platform-youtube-accent: #ff0000;
        --platform-kick-accent: #00e701;
        --platform-tiktok-accent: #000000;
        --platform-discord-accent: #5865f2;
        --chat-show-super-chat: block;
        --chat-show-subscriptions: block;
        --chat-show-raids: block;
        --chat-show-bits: block;
        --chat-show-membership-gift: block;
        --chat-super-chat-size-modifier: 1;
        --chat-subscription-size-modifier: 1;
        --chat-raid-size-modifier: 1;
        --chat-bits-size-modifier: 1;
        --chat-membership-gift-size-modifier: 1.2;
      }
    `
    const result = parseCssToVisualSettings(fullThemeCss)
    expect(Object.keys(result).length).toBe(50)
    expect(result.fontFamily).toBe('Inter')
    expect(result.membershipGiftSizeModifier).toBe('1.2')
    expect(result.twitchAccent).toBe('#9146ff')
    expect(result.discordAccent).toBe('#5865f2')
  })

  it('parses overlayBgColor and overlayBgOpacity as independent fields', () => {
    const css = ':root { --chat-overlay-bg-color: #000000; --chat-overlay-bg-opacity: 0.8; }'
    expect(parseCssToVisualSettings(css)).toEqual({
      overlayBgColor: '#000000',
      overlayBgOpacity: '0.8',
    })
  })

  it('second call is not affected by stale regex state from first call', () => {
    const cssA = ':root { --chat-font-family: Inter; }'
    const cssB = ':root { --chat-font-weight: 700; }'
    const resultA = parseCssToVisualSettings(cssA)
    const resultB = parseCssToVisualSettings(cssB)
    expect(resultA).toEqual({ fontFamily: 'Inter' })
    expect(resultB).toEqual({ fontWeight: '700' })
  })

  it('trims whitespace from values', () => {
    const css = ':root { --chat-font-weight:  700  ; }'
    expect(parseCssToVisualSettings(css)).toEqual({ fontWeight: '700' })
  })
})
