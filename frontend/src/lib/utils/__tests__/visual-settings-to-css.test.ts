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
import { visualSettingsToCss } from '../visual-settings-to-css'
import type { VisualSettings } from '@/lib/types/visual-settings'

describe('visualSettingsToCss', () => {
  it('returns empty string for empty input', () => {
    expect(visualSettingsToCss({})).toBe('')
  })

  it('returns empty string for undefined-only input', () => {
    expect(visualSettingsToCss({ fontFamily: undefined, messageColor: undefined })).toBe('')
  })

  it('emits only set properties for partial input', () => {
    const result = visualSettingsToCss({ fontFamily: 'Inter', messageColor: '#ffffff' })

    expect(result).toContain('@layer visual-customizer')
    expect(result).toContain(':root')
    expect(result).toContain('--chat-font-family: Inter;')
    expect(result).toContain('--chat-message-color: #ffffff;')

    // Properties not set must not appear
    expect(result).not.toContain('--chat-font-weight')
    expect(result).not.toContain('--chat-bubble-bg-color')
  })

  it('emits all properties for full input', () => {
    const full: Required<VisualSettings> = {
      fontFamily: 'Inter',
      fontWeight: '400',
      lineHeight: '1.5',
      letterSpacing: '0px',
      fontSize: '16px',
      textShadow: '0 1px 2px rgba(0, 0, 0, 0.6)',
      messageColor: '#ffffff',
      usernameColor: '#aaaaaa',
      timestampColor: '#666666',
      usernameFontFamily: 'Inter',
      timestampFontFamily: 'Georgia',
      usernameFontWeight: '600',
      usernameFontSize: '14px',
      timestampFontSize: '11px',
      overlayBgColor: '#000000',
      overlayBgOpacity: '0.5',
      overlayPadding: '16px',
      bubbleBgColor: '#111111',
      bubbleBgOpacity: '0.9',
      bubbleBorderRadius: '8px',
      bubbleBorderWidth: '1px',
      bubbleBorderColor: '#333333',
      bubblePadding: '12px',
      bubbleShadow: 'none',
      messageGap: '8px',
      backdropBlur: '0px',
      maxWidth: '100%',
      showAvatars: 'inline',
      showBadges: 'inline',
      showTimestamps: 'block',
      showPlatformBadge: 'inline',
      showPlatformIndicators: 'block',
      showEmotes: 'inline',
      showUsername: 'inline',
      avatarSize: '40px',
      badgeSize: '18px',
      emoteScale: '1',
      twitchAccent: '#9146ff',
      youtubeAccent: '#ff0000',
      kickAccent: '#00e701',
      tiktokAccent: '#000000',
      discordAccent: '#5865f2',
      showSuperChat: 'block',
      showSubscriptions: 'block',
      showRaids: 'block',
      showBits: 'block',
      showMembershipGift: 'block',
      superChatSizeModifier: '1',
      subscriptionSizeModifier: '1',
      raidSizeModifier: '1',
      bitsSizeModifier: '1',
      membershipGiftSizeModifier: '1.2',
      platformBadgePosition: 'before',
      platformBadgeStyle: 'text',
      // Message entry animation (not CSS-driven, not emitted as CSS properties)
      messageAnimation: 'fly-left',
      // Phase 9: Pronoun display (not CSS-driven, not emitted as CSS properties)
      showPronouns: 'inline',
      pronounPosition: 'after',
      pronounColor: '#7B68EE',
    }

    const result = visualSettingsToCss(full)

    expect(result).toContain('@layer visual-customizer')
    expect(result).toContain('--chat-font-family: Inter;')
    expect(result).toContain('--platform-twitch-accent: #9146ff;')
    expect(result).toContain('--platform-discord-accent: #5865f2;')
    expect(result).toContain('--chat-show-super-chat: block;')
    expect(result).toContain('--chat-bits-size-modifier: 1;')
    expect(result).toContain('--chat-membership-gift-size-modifier: 1.2;')
    // platformBadgePosition and platformBadgeStyle are not CSS properties, so not emitted
    expect(result).not.toContain('platformBadgePosition')
    expect(result).not.toContain('platformBadgeStyle')
    // messageAnimation is applied as a .msg-anim-* class, never as a CSS property
    expect(result).not.toContain('messageAnimation')
    expect(result).not.toContain('fly-left')
    // All 52 CSS properties present (excludes non-CSS fields)
    expect((result.match(/--chat-|--platform-/g) ?? []).length).toBe(52)
  })

  it('wraps output in correct cascade layer syntax', () => {
    const result = visualSettingsToCss({ fontFamily: 'Roboto' })
    expect(result.trim()).toMatch(/^@layer visual-customizer \{/)
    expect(result).toContain('  :root {')
    expect(result.trim()).toMatch(/\}$/)
  })

  it('emits --chat-username-font-family for usernameFontFamily', () => {
    const result = visualSettingsToCss({ usernameFontFamily: 'Inter' })
    expect(result).toContain('--chat-username-font-family: Inter;')
  })

  it('emits --chat-timestamp-font-family for timestampFontFamily', () => {
    const result = visualSettingsToCss({ timestampFontFamily: 'Georgia' })
    expect(result).toContain('--chat-timestamp-font-family: Georgia;')
  })

  it('emits --chat-text-shadow for textShadow, including multi-shadow values', () => {
    const result = visualSettingsToCss({
      textShadow: '0 1px 2px rgba(0, 0, 0, 0.9), 0 2px 6px rgba(0, 0, 0, 0.7)',
    })
    expect(result).toContain(
      '--chat-text-shadow: 0 1px 2px rgba(0, 0, 0, 0.9), 0 2px 6px rgba(0, 0, 0, 0.7);'
    )
  })
})

/**
 * A `--chat-*` variable that nothing consumes paints nothing, and the inline
 * styles that used to carry these three settings lose to the `!important`
 * declarations bundled themes use. So they are also emitted as `!important`
 * rules inside the cascade layer — but only when the user actually set them.
 */
describe('visualSettingsToCss forced overrides', () => {
  const OUTLINE = '1px 1px 0 #000, -1px 1px 0 #000'

  it('forces text-shadow on the text nodes themes restyle, on both surfaces', () => {
    const result = visualSettingsToCss({ textShadow: OUTLINE })

    for (const scope of ['.overlay-live-body', '.overlay-preview-body']) {
      for (const node of ['.break-words', '.chat-username', '.text-xs.text-slate-500']) {
        expect(result).toContain(`${scope} ${node}`)
      }
    }
    expect(result).toContain(`text-shadow: ${OUTLINE} !important;`)
  })

  it('forces box-shadow on chat rows only — never events, never the sentinel', () => {
    const result = visualSettingsToCss({ bubbleShadow: '0 2px 6px rgba(0, 0, 0, 0.5)' })

    expect(result).toContain('.overlay-live-body > div:not(.event-message):not(.scroll-anchor)')
    expect(result).toContain('.overlay-preview-body > div:not(.event-message):not(.scroll-anchor)')
    expect(result).toContain('box-shadow: 0 2px 6px rgba(0, 0, 0, 0.5) !important;')
  })

  it('recolours a platform badge via both color and the SVG shape fill', () => {
    const result = visualSettingsToCss({ twitchAccent: '#9146ff' })

    expect(result).toContain(".overlay-live-body [data-platform='twitch'] .platform-badge,")
    expect(result).toContain(".overlay-preview-body [data-platform='twitch'] .platform-badge {")
    expect(result).toContain(".overlay-live-body [data-platform='twitch'] .platform-badge svg *")
    expect(result).toContain('color: #9146ff !important;')
    expect(result).toContain('fill: #9146ff !important;')
    // Only the platform that was set
    expect(result).not.toContain("data-platform='youtube'")
  })

  it('emits no forced rule for an unset setting, leaving the theme alone', () => {
    const result = visualSettingsToCss({ fontFamily: 'Inter' })

    expect(result).not.toContain('!important')
    expect(result).not.toContain('.overlay-live-body')
  })

  it('stays inside the cascade layer', () => {
    const result = visualSettingsToCss({ textShadow: OUTLINE })
    expect(result.trim()).toMatch(/^@layer visual-customizer \{/)
    expect(result.trim()).toMatch(/\}$/)
    // one layer block only — the vars and the forced rules share it
    expect((result.match(/@layer/g) ?? []).length).toBe(1)
  })

  it('skips a value with unbalanced parens instead of corrupting the block', () => {
    const result = visualSettingsToCss({
      textShadow: '0 1px 2px rgba(0, 0, 0, 0.9',
      messageColor: '#ffffff',
    })
    expect(result).not.toContain('text-shadow')
    expect(result).toContain('--chat-message-color: #ffffff;')
  })
})
