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

/**
 * VisualSettings — structured CSS properties for the visual-customizer cascade layer.
 *
 * Each field maps to a CSS custom property on :root.
 * All fields are optional; only set fields are emitted as CSS.
 *
 * Property mapping: camelCase field → --chat-* or --platform-* CSS variable
 * See visual-settings-to-css.ts for the authoritative mapping.
 */
export interface VisualSettings {
  // Typography
  fontFamily?: string         // --chat-font-family
  fontWeight?: string         // --chat-font-weight
  lineHeight?: string         // --chat-line-height
  letterSpacing?: string      // --chat-letter-spacing
  fontSize?: string           // --chat-font-size
  textShadow?: string         // --chat-text-shadow (inherited from the overlay container)

  // Colors
  messageColor?: string       // --chat-message-color
  usernameColor?: string      // --chat-username-color
  timestampColor?: string     // --chat-timestamp-color

  // Username typography
  usernameFontFamily?: string // --chat-username-font-family
  timestampFontFamily?: string // --chat-timestamp-font-family
  usernameFontWeight?: string // --chat-username-font-weight
  usernameFontSize?: string   // --chat-username-font-size
  timestampFontSize?: string  // --chat-timestamp-font-size

  // Background & Bubbles
  overlayBgColor?: string       // --chat-overlay-bg-color
  overlayBgOpacity?: string     // --chat-overlay-bg-opacity
  overlayPadding?: string       // --chat-overlay-padding
  bubbleBgColor?: string        // --chat-bubble-bg-color
  bubbleBgOpacity?: string      // --chat-bubble-bg-opacity
  bubbleBorderRadius?: string   // --chat-bubble-border-radius
  bubbleBorderWidth?: string    // --chat-bubble-border-width
  bubbleBorderColor?: string    // --chat-bubble-border-color
  bubblePadding?: string        // --chat-bubble-padding
  bubbleShadow?: string         // --chat-bubble-shadow
  messageGap?: string           // --chat-message-gap
  backdropBlur?: string         // --chat-backdrop-blur
  maxWidth?: string             // --chat-max-width

  // Visibility toggles ('inline' | 'none' for inline elements; 'block' | 'none' for block)
  showAvatars?: 'inline' | 'none'        // --chat-show-avatars
  showBadges?: 'inline' | 'none'         // --chat-show-badges
  showTimestamps?: 'block' | 'none'      // --chat-show-timestamps
  showPlatformBadge?: 'inline' | 'none'  // --chat-show-platform-badge
  showPlatformIndicators?: 'block' | 'none'  // --chat-show-platform-indicators
  showEmotes?: 'inline' | 'none'         // --chat-show-emotes
  showUsername?: 'inline' | 'none'       // --chat-show-username

  // Platform badge options (not CSS-driven, stored for settings persistence)
  platformBadgePosition?: 'before' | 'after'
  platformBadgeStyle?: 'text' | 'icon'

  // Phase 9: Pronoun display
  showPronouns?: 'inline' | 'none'      // --chat-show-pronouns
  pronounPosition?: 'before' | 'after'  // not CSS-driven, stored for persistence
  pronounColor?: string                 // not CSS-driven, stored for persistence

  // Sizing
  avatarSize?: string   // --chat-avatar-size
  badgeSize?: string    // --chat-badge-size
  emoteScale?: string   // --chat-emote-scale

  // Platform accent colors
  twitchAccent?: string    // --platform-twitch-accent
  youtubeAccent?: string   // --platform-youtube-accent
  kickAccent?: string      // --platform-kick-accent
  tiktokAccent?: string    // --platform-tiktok-accent
  discordAccent?: string   // --platform-discord-accent

  // Event visibility
  showSuperChat?: 'block' | 'none'       // --chat-show-super-chat
  showSubscriptions?: 'block' | 'none'   // --chat-show-subscriptions
  showRaids?: 'block' | 'none'           // --chat-show-raids
  showBits?: 'block' | 'none'            // --chat-show-bits
  showMembershipGift?: 'block' | 'none'  // --chat-show-membership-gift

  // Event size modifiers
  superChatSizeModifier?: string       // --chat-super-chat-size-modifier
  subscriptionSizeModifier?: string    // --chat-subscription-size-modifier
  raidSizeModifier?: string            // --chat-raid-size-modifier
  bitsSizeModifier?: string            // --chat-bits-size-modifier
  membershipGiftSizeModifier?: string  // --chat-membership-gift-size-modifier
}
