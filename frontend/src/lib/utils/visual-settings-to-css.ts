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

import type { VisualSettings } from '@/lib/types/visual-settings'
import { canonicalizeTextShadow } from './text-outline'

/**
 * Authoritative mapping from VisualSettings field names to CSS custom property names.
 * Order determines declaration order in the generated CSS string.
 */
export const PROPERTY_MAP: ReadonlyArray<[keyof VisualSettings, string]> = [
  // Typography
  ['fontFamily', '--chat-font-family'],
  ['fontWeight', '--chat-font-weight'],
  ['lineHeight', '--chat-line-height'],
  ['letterSpacing', '--chat-letter-spacing'],
  ['fontSize', '--chat-font-size'],
  ['textShadow', '--chat-text-shadow'],
  // Colors
  ['messageColor', '--chat-message-color'],
  ['usernameColor', '--chat-username-color'],
  ['timestampColor', '--chat-timestamp-color'],
  // Username typography
  ['usernameFontFamily', '--chat-username-font-family'],
  ['timestampFontFamily', '--chat-timestamp-font-family'],
  ['usernameFontWeight', '--chat-username-font-weight'],
  ['usernameFontSize', '--chat-username-font-size'],
  ['timestampFontSize', '--chat-timestamp-font-size'],
  // Background & Bubbles
  ['overlayBgColor', '--chat-overlay-bg-color'],
  ['overlayBgOpacity', '--chat-overlay-bg-opacity'],
  ['overlayPadding', '--chat-overlay-padding'],
  ['bubbleBgColor', '--chat-bubble-bg-color'],
  ['bubbleBgOpacity', '--chat-bubble-bg-opacity'],
  ['bubbleBorderRadius', '--chat-bubble-border-radius'],
  ['bubbleBorderWidth', '--chat-bubble-border-width'],
  ['bubbleBorderColor', '--chat-bubble-border-color'],
  ['bubblePadding', '--chat-bubble-padding'],
  ['bubbleShadow', '--chat-bubble-shadow'],
  ['messageGap', '--chat-message-gap'],
  ['backdropBlur', '--chat-backdrop-blur'],
  ['maxWidth', '--chat-max-width'],
  // Visibility
  ['showAvatars', '--chat-show-avatars'],
  ['showBadges', '--chat-show-badges'],
  ['showTimestamps', '--chat-show-timestamps'],
  ['showPlatformBadge', '--chat-show-platform-badge'],
  ['showPlatformIndicators', '--chat-show-platform-indicators'],
  ['showEmotes', '--chat-show-emotes'],
  ['showUsername', '--chat-show-username'],
  // Sizing
  ['avatarSize', '--chat-avatar-size'],
  ['badgeSize', '--chat-badge-size'],
  ['emoteScale', '--chat-emote-scale'],
  // Platform accents
  ['twitchAccent', '--platform-twitch-accent'],
  ['youtubeAccent', '--platform-youtube-accent'],
  ['kickAccent', '--platform-kick-accent'],
  ['tiktokAccent', '--platform-tiktok-accent'],
  ['discordAccent', '--platform-discord-accent'],
  // Event visibility
  ['showSuperChat', '--chat-show-super-chat'],
  ['showSubscriptions', '--chat-show-subscriptions'],
  ['showRaids', '--chat-show-raids'],
  ['showBits', '--chat-show-bits'],
  ['showMembershipGift', '--chat-show-membership-gift'],
  // Event size modifiers
  ['superChatSizeModifier', '--chat-super-chat-size-modifier'],
  ['subscriptionSizeModifier', '--chat-subscription-size-modifier'],
  ['raidSizeModifier', '--chat-raid-size-modifier'],
  ['bitsSizeModifier', '--chat-bits-size-modifier'],
  ['membershipGiftSizeModifier', '--chat-membership-gift-size-modifier'],
]

/**
 * Presence flags: emitted as `1` when the paired field is set, absent otherwise.
 *
 * A theme reading `var(--chat-bubble-bg-color, <own colour>)` cannot tell which
 * branch it got, and a theme whose look varies the bubble colour PER ROW needs
 * to know. Sticky Notes gives every note a different paper; the moment the user
 * picks one colour that variety has to move somewhere else, so the theme
 * multiplies a per-note shading wash by this flag — absent (`0`) leaves its own
 * three papers pixel-identical, `1` restores the note-to-note rhythm on top of
 * the user's colour.
 *
 * Deliberately NOT `--chat-*`/`--platform-*` prefixed: theme-css-parser
 * reverse-maps those out of a theme's `var()` usages, so a flag under that
 * prefix would warn as an unknown variable and be read back as a setting.
 */
const PRESENCE_FLAGS: ReadonlyArray<[keyof VisualSettings, string]> = [
  ['bubbleBgColor', '--customizer-bubble-bg-set'],
]

/**
 * Values with unbalanced parentheses are partial extractions of complex CSS
 * functions (e.g. a truncated `linear-gradient`) and corrupt every declaration
 * that follows them in the block.
 */
function hasBalancedParens(value: string): boolean {
  return (value.match(/\(/g) ?? []).length === (value.match(/\)/g) ?? []).length
}

/** Feed-body scope hook, one per overlay surface (live OBS page + editor preview). */
const FEED_SCOPES = ['.overlay-live-body', '.overlay-preview-body'] as const

/**
 * A chat row. Events are excluded (their chrome is theme-owned through the
 * `--event-*` tokens) and so is `.scroll-anchor`, the invisible auto-scroll
 * sentinel — an `!important` declaration inside a cascade layer outranks the
 * unlayered `!important` reset globals.css gives it, so anything styling rows
 * from inside a layer has to skip it by selector or the sentinel grows a box.
 */
const CHAT_ROW = '> div:not(.event-message):not(.scroll-anchor)'

/**
 * The text nodes bundled themes restyle. A `text-shadow` on the feed body does
 * inherit down to these, but inheritance loses to any direct declaration — and
 * every bundled theme has one.
 */
const TEXT_NODES = ['.break-words', '.chat-username', '.text-xs.text-slate-500'] as const

/** VisualSettings field → the `data-platform` value whose badge it recolours. */
const ACCENT_PLATFORMS: ReadonlyArray<[keyof VisualSettings, string]> = [
  ['twitchAccent', 'twitch'],
  ['youtubeAccent', 'youtube'],
  ['kickAccent', 'kick'],
  ['tiktokAccent', 'tiktok'],
  ['discordAccent', 'discord'],
]

interface OverrideRule {
  field: keyof VisualSettings
  /** Selector suffixes, appended to each feed scope. `''` targets the scope itself. */
  targets: readonly string[]
  /** CSS properties to set to the field's value. */
  properties: readonly string[]
}

/**
 * Customizer properties delivered as an `!important` rule instead of a bare
 * custom property on `:root`.
 *
 * A `--chat-*` variable only reaches the pixels if SOMETHING consumes it —
 * a rule in events.css, or a `var()` in the active theme. These properties have
 * neither, so the overlay pages applied them as plain inline styles: the
 * weakest declaration in the cascade. Every bundled theme declares
 * `text-shadow` / `box-shadow` with `!important`, and an `!important`
 * declaration beats a normal inline style, so the controls were inert on every
 * themed overlay — the Text Shadow control (Soft / Strong / **Outline**) did
 * nothing, and the whole Platform Colors section did nothing anywhere because
 * `--platform-*-accent` had no consumer at all.
 *
 * These rules are `!important` inside `@layer visual-customizer`, which beats a
 * theme's unlayered `!important` (CSS Cascade 5 reverses layer order for
 * important declarations and ranks unlayered last) — the same mechanism
 * events.css already uses for every other customizer property.
 *
 * They are emitted ONLY when the field is set, and that is what makes forcing
 * them safe: no bundled theme declares or reads any of these variables, so a
 * value here can only have come from the user, and an unset control emits
 * nothing and leaves the theme's own look untouched.
 *
 * Do NOT extend this table to a variable that themes DO read
 * (`--chat-bubble-bg-color`, `--chat-font-size`, `--chat-overlay-bg-color`, …).
 * theme-css-parser back-fills those fields from the theme's own `var()`
 * fallbacks, so "set" there does not mean "the user chose it", and forcing it
 * would pin every overlay to its theme's default. customizer-coverage.test.ts
 * guards both halves of this invariant.
 */
const OVERRIDE_RULES: readonly OverrideRule[] = [
  { field: 'textShadow', targets: TEXT_NODES, properties: ['text-shadow'] },
  { field: 'bubbleShadow', targets: [CHAT_ROW], properties: ['box-shadow'] },
  // The text badge takes `color`. The icon badge is an inline SVG whose brand
  // colour sits in a `fill` presentation attribute on the shape itself — lower
  // priority than any CSS declaration, but only reachable by targeting the
  // shape, not the wrapping <svg>.
  ...ACCENT_PLATFORMS.flatMap(([field, platform]): OverrideRule[] => [
    {
      field,
      targets: [`[data-platform='${platform}'] .platform-badge`],
      properties: ['color'],
    },
    {
      field,
      targets: [`[data-platform='${platform}'] .platform-badge svg *`],
      properties: ['fill'],
    },
  ]),
]

/** Field names this module forces with an `!important` rule (see OVERRIDE_RULES). */
export const OVERRIDDEN_FIELDS: ReadonlySet<keyof VisualSettings> = new Set(
  OVERRIDE_RULES.map((rule) => rule.field)
)

/**
 * Longest bubble palette the editor offers and the emitter renders. Bounded
 * because each entry costs a rule per surface, and because a rhythm the eye can
 * follow needs a short cycle — beyond half a dozen it just reads as noise.
 */
export const MAX_BUBBLE_PALETTE = 6

/** Per-row attribute the overlay surfaces write for the palette to key on. */
export const BUBBLE_SLOT_ATTR = 'data-bubble-slot'

/** VisualSettings field → the `data-platform` value whose bubble it fills. */
const BUBBLE_TINT_PLATFORMS: ReadonlyArray<[keyof VisualSettings, string]> = [
  ['twitchBubbleBg', 'twitch'],
  ['youtubeBubbleBg', 'youtube'],
  ['kickBubbleBg', 'kick'],
  ['tiktokBubbleBg', 'tiktok'],
  ['discordBubbleBg', 'discord'],
]

/**
 * The palette actually in effect: entries that are usable CSS colours, clamped
 * to MAX_BUBBLE_PALETTE, and only if at least two survive (one colour is the
 * plain "Bubble background" setting, not a cycle).
 *
 * Exported because the overlay surfaces need the same length to compute
 * `data-bubble-slot`; deriving it twice would drift.
 */
export function resolveBubblePalette(settings: Partial<VisualSettings>): string[] {
  const palette = (settings.bubblePalette ?? [])
    .filter((color) => typeof color === 'string' && color !== '' && hasBalancedParens(color))
    .slice(0, MAX_BUBBLE_PALETTE)
  return palette.length >= 2 ? palette : []
}

/**
 * Differently-coloured bubbles: a palette cycled down the feed, plus per-platform
 * fills that win over it.
 *
 * Emitted as `!important` rules inside the cascade layer for the same reason as
 * OVERRIDE_RULES — a theme's own `background: … !important` on the row would
 * otherwise beat them — and, like those, only when configured, so an unset
 * control leaves the theme's fill alone.
 *
 * Order matters and is the precedence rule: palette first, platform second. Both
 * selectors are one class plus one attribute, so specificity ties and the later
 * rule wins. That is what makes "a platform tint overrides the palette on that
 * platform's rows" true without any `:not()` gymnastics.
 */
function bubbleFillRules(settings: Partial<VisualSettings>): string[] {
  const blocks: string[] = []

  const rule = (predicate: string, color: string): string =>
    [
      FEED_SCOPES.map((scope) => `  ${scope} > div${predicate}:not(.event-message)`).join(',\n') +
        ' {',
      `    background-color: ${color} !important;`,
      '  }',
    ].join('\n')

  // `.scroll-anchor` needs no exclusion here: the sentinel carries neither a
  // slot attribute nor a platform, so no predicate below can match it.
  resolveBubblePalette(settings).forEach((color, slot) => {
    blocks.push(rule(`[${BUBBLE_SLOT_ATTR}='${slot}']`, color))
  })

  for (const [field, platform] of BUBBLE_TINT_PLATFORMS) {
    const color = settings[field]
    if (typeof color !== 'string' || color === '' || !hasBalancedParens(color)) continue
    blocks.push(rule(`[data-platform='${platform}']`, color))
  }

  return blocks
}

/**
 * The value to emit for a field, which is not always the value that was
 * stored. `textShadow` carries the outline's thickness inside the declaration
 * (see text-outline.ts), so the declaration is re-derived from that thickness
 * on the way out: an overlay saved while a broken sampling was live renders
 * correctly on the next deploy, with no migration and no second visit to the
 * editor. Every other field is emitted verbatim.
 */
function resolveValue(field: keyof VisualSettings, value: string): string {
  return field === 'textShadow' ? canonicalizeTextShadow(value) : value
}

/** Renders the set OVERRIDE_RULES as CSS rule blocks; empty when none apply. */
function overrideRules(settings: Partial<VisualSettings>): string[] {
  const blocks: string[] = []

  for (const { field, targets, properties } of OVERRIDE_RULES) {
    const stored = settings[field]
    if (typeof stored !== 'string' || stored === '') continue
    if (!hasBalancedParens(stored)) continue
    const value = resolveValue(field, stored)

    const selectors = FEED_SCOPES.flatMap((scope) =>
      targets.map((target) => (target === '' ? scope : `${scope} ${target}`))
    )
    blocks.push(
      [
        selectors.map((selector) => `  ${selector}`).join(',\n') + ' {',
        ...properties.map((property) => `    ${property}: ${value} !important;`),
        '  }',
      ].join('\n')
    )
  }

  return blocks
}

/**
 * Converts a VisualSettings object to a @layer visual-customizer CSS string.
 *
 * - Empty or all-undefined input → returns ""
 * - Only set (non-undefined) properties are emitted
 * - Output is placed inside @layer visual-customizer { ... }, holding a `:root`
 *   block of custom properties plus the OVERRIDE_RULES that apply
 */
export function visualSettingsToCss(settings: Partial<VisualSettings>): string {
  const declarations: string[] = []

  for (const [field, cssVar] of PROPERTY_MAP) {
    const stored = settings[field]
    if (typeof stored !== 'string') continue
    if (!hasBalancedParens(stored)) continue
    declarations.push(`    ${cssVar}: ${resolveValue(field, stored)};`)
  }

  for (const [field, cssVar] of PRESENCE_FLAGS) {
    const value = settings[field]
    if (value === undefined || value === '') continue
    declarations.push(`    ${cssVar}: 1;`)
  }

  const blocks: string[] = []
  if (declarations.length > 0) {
    blocks.push(['  :root {', ...declarations, '  }'].join('\n'))
  }
  blocks.push(...overrideRules(settings), ...bubbleFillRules(settings))

  if (blocks.length === 0) {
    return ''
  }

  return ['@layer visual-customizer {', blocks.join('\n\n'), '}'].join('\n')
}
