import type { VisualSettings } from '@/lib/types/visual-settings'

/**
 * Authoritative mapping from VisualSettings field names to CSS custom property names.
 * Order determines declaration order in the generated CSS string.
 */
const PROPERTY_MAP: ReadonlyArray<[keyof VisualSettings, string]> = [
  // Typography
  ['fontFamily',            '--chat-font-family'],
  ['fontWeight',            '--chat-font-weight'],
  ['lineHeight',            '--chat-line-height'],
  ['letterSpacing',         '--chat-letter-spacing'],
  ['fontSize',              '--chat-font-size'],
  // Colors
  ['messageColor',          '--chat-message-color'],
  ['usernameColor',         '--chat-username-color'],
  ['timestampColor',        '--chat-timestamp-color'],
  // Username typography
  ['usernameFontFamily',    '--chat-username-font-family'],
  ['timestampFontFamily',   '--chat-timestamp-font-family'],
  ['usernameFontWeight',    '--chat-username-font-weight'],
  ['usernameFontSize',      '--chat-username-font-size'],
  ['timestampFontSize',     '--chat-timestamp-font-size'],
  // Background & Bubbles
  ['overlayBgColor',        '--chat-overlay-bg-color'],
  ['overlayBgOpacity',      '--chat-overlay-bg-opacity'],
  ['overlayPadding',        '--chat-overlay-padding'],
  ['bubbleBgColor',         '--chat-bubble-bg-color'],
  ['bubbleBgOpacity',       '--chat-bubble-bg-opacity'],
  ['bubbleBorderRadius',    '--chat-bubble-border-radius'],
  ['bubbleBorderWidth',     '--chat-bubble-border-width'],
  ['bubbleBorderColor',     '--chat-bubble-border-color'],
  ['bubblePadding',         '--chat-bubble-padding'],
  ['bubbleShadow',          '--chat-bubble-shadow'],
  ['messageGap',            '--chat-message-gap'],
  ['backdropBlur',          '--chat-backdrop-blur'],
  ['maxWidth',              '--chat-max-width'],
  // Visibility
  ['showAvatars',           '--chat-show-avatars'],
  ['showBadges',            '--chat-show-badges'],
  ['showTimestamps',        '--chat-show-timestamps'],
  ['showPlatformBadge',     '--chat-show-platform-badge'],
  ['showEmotes',            '--chat-show-emotes'],
  ['showUsername',          '--chat-show-username'],
  // Sizing
  ['avatarSize',            '--chat-avatar-size'],
  ['badgeSize',             '--chat-badge-size'],
  ['emoteScale',            '--chat-emote-scale'],
  // Platform accents
  ['twitchAccent',          '--platform-twitch-accent'],
  ['youtubeAccent',         '--platform-youtube-accent'],
  ['kickAccent',            '--platform-kick-accent'],
  ['tiktokAccent',          '--platform-tiktok-accent'],
  ['discordAccent',         '--platform-discord-accent'],
  // Event visibility
  ['showSuperChat',         '--chat-show-super-chat'],
  ['showSubscriptions',     '--chat-show-subscriptions'],
  ['showRaids',             '--chat-show-raids'],
  ['showBits',              '--chat-show-bits'],
  ['showMembershipGift',    '--chat-show-membership-gift'],
  // Event size modifiers
  ['superChatSizeModifier',      '--chat-super-chat-size-modifier'],
  ['subscriptionSizeModifier',   '--chat-subscription-size-modifier'],
  ['raidSizeModifier',           '--chat-raid-size-modifier'],
  ['bitsSizeModifier',           '--chat-bits-size-modifier'],
  ['membershipGiftSizeModifier', '--chat-membership-gift-size-modifier'],
]

/**
 * Converts a VisualSettings object to a @layer visual-customizer CSS string.
 *
 * - Empty or all-undefined input → returns ""
 * - Only set (non-undefined) properties are emitted
 * - Output is placed inside @layer visual-customizer { :root { ... } }
 */
export function visualSettingsToCss(settings: Partial<VisualSettings>): string {
  const declarations: string[] = []

  for (const [field, cssVar] of PROPERTY_MAP) {
    const value = settings[field]
    if (value !== undefined) {
      declarations.push(`    ${cssVar}: ${value};`)
    }
  }

  if (declarations.length === 0) {
    return ''
  }

  return [
    '@layer visual-customizer {',
    '  :root {',
    ...declarations,
    '  }',
    '}',
  ].join('\n')
}
