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
    }

    const result = visualSettingsToCss(full)

    expect(result).toContain('@layer visual-customizer')
    expect(result).toContain('--chat-font-family: Inter;')
    expect(result).toContain('--platform-twitch-accent: #9146ff;')
    expect(result).toContain('--platform-discord-accent: #5865f2;')
    expect(result).toContain('--chat-show-super-chat: block;')
    expect(result).toContain('--chat-bits-size-modifier: 1;')
    expect(result).toContain('--chat-membership-gift-size-modifier: 1.2;')
    // All 50 properties present (49 original + membershipGiftSizeModifier)
    expect((result.match(/--chat-|--platform-/g) ?? []).length).toBe(50)
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
})
