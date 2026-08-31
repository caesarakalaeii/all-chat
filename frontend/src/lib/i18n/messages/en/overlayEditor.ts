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
 * The overlay editor: canvas, inspector, appearance controls and the theme
 * marketplace.
 */

export const overlayEditor = {
  nav: {
    settingsLabel: 'Overlay settings',
    // The count is a placeholder rather than an appended `({n})`: the
    // parentheses are punctuation a language may render differently.
    advancedCount: 'Advanced ({count})',
  },
  previewBackdrop: {
    heading: 'Backdrop',
    appBackground: 'Preview on app background',
    lightBackground: 'Preview on light background',
    chromaGreen: 'Preview on chroma green',
    customColor: 'Custom preview background color',
  },
  settingsSearch: {
    label: 'Search settings',
    placeholder: 'Search settings… (e.g. badge, fade, banned words)',
    clearLabel: 'Clear search',
    resultsLabel: 'Matching settings',
    noResults: 'No settings match “{query}”',
  },
  // The appearance groups in src/components/appearance/. Each group is its own
  // second-level namespace, keyed by the group the control is rendered in
  // rather than by the setting it writes. Keys stay three levels deep at most
  // (docs/frontend/I18N.md), so there is no shared `appearance` level.
  background: {
    overlayHeading: 'Overlay background',
    overlayColor: 'Overlay background',
    bubbleHeading: 'Bubble background',
    bubbleColor: 'Bubble background',
    borderColor: 'Border color',
    borderRadius: 'Border radius',
    borderWidth: 'Border width',
    padding: 'Padding',
    messageGap: 'Message gap',
    backdropBlur: 'Backdrop blur',
  },
  colors: {
    message: 'Message color',
    username: 'Username color',
    timestamp: 'Timestamp color',
  },
  sizing: {
    avatarSize: 'Avatar size',
    badgeSize: 'Badge size',
    emoteScale: 'Emote scale',
    emoteScaleNote:
      'Emote scale applies to third-party emotes (7TV, BTTV, FFZ). Standard emoji are not affected.',
  },
  events: {
    sizeModifier: 'Size modifier',
    // Keyed by the VisualSettings field the row toggles, so the render site
    // looks the label up from the row it is already iterating.
    showSuperChat: 'Super Chat',
    showSubscriptions: 'Subscriptions',
    showRaids: 'Raids',
    showBits: 'Bits',
    showMembershipGift: 'Membership Gift',
  },
  typography: {
    bodyFont: 'Body Font',
    usernameFont: 'Username Font',
    timestampFont: 'Timestamp Font',
    fontWeight: 'Font Weight',
    fontWeightPlaceholder: 'Select weight…',
    // Suffixed with the CSS font-weight value the option writes, so the picker
    // looks each label up from the value it already has.
    fontWeight100: '100 Thin',
    fontWeight300: '300 Light',
    fontWeight400: '400 Regular',
    fontWeight500: '500 Medium',
    fontWeight600: '600 SemiBold',
    fontWeight700: '700 Bold',
    fontWeight800: '800 ExtraBold',
    fontWeight900: '900 Black',
    bodySize: 'Body Size',
    usernameSize: 'Username Size',
    timestampSize: 'Timestamp Size',
    // Describes each size input to a screen reader, so it is copy: a language
    // may abbreviate or order the unit differently.
    pixelUnit: 'px',
    textShadow: 'Text Shadow',
    textShadowNone: 'None (default)',
    textShadowSoft: 'Soft shadow',
    textShadowStrong: 'Strong shadow',
    textShadowOutline: 'Outline',
    textShadowCustom: 'Custom',
    textShadowNote:
      'Keeps chat readable over bright gameplay. Try it with a light preview backdrop.',
    lineHeight: 'Line Height',
    letterSpacing: 'Letter Spacing',
  },
  visibility: {
    // Keyed by the VisualSettings field each toggle writes.
    showAvatars: 'Show avatars',
    showBadges: 'Show badges',
    showTimestamps: 'Show timestamps',
    showEmotes: 'Show emotes',
    showUsername: 'Show username',
    showPlatformBadge: 'Show platform badge',
    showPlatformIndicators: 'Show platform indicators',
    showPronouns: 'Show pronouns',
    position: 'Position',
    style: 'Style',
    beforeUsername: 'Before username',
    afterUsername: 'After username',
    styleText: 'Text',
    styleIcon: 'Icon',
    pronounPillColor: 'Pill color',
  },
  colorPicker: {
    swatchLabel: 'Pick color for {label}',
    popoverTitle: 'Color for {label}',
    hexLabel: 'Hex value for {label}',
  },
  fontPicker: {
    placeholder: 'Select font…',
    // Fallback accessible name when a caller passes no aria-label.
    defaultLabel: 'Font family',
    openLabel: 'Open font picker',
    empty: 'No fonts found',
    systemGroup: 'System Fonts',
    googleGroup: 'Google Fonts',
  },
  themeMarketplace: {
    // Two title keys, not a qualifier concatenated onto a shared noun phrase:
    // see the comment in __tests__/overlayEditor.test.ts.
    title: 'Theme Marketplace',
    titleCreditRoll: 'Credit Roll Theme Marketplace',
    description: 'Browse and apply custom CSS themes for your overlay',
    descriptionCreditRoll: 'Browse and apply custom CSS themes for your credit roll',
    loading: 'Loading themes',
    loadingEllipsis: 'Loading themes...',
    errorTitle: 'Error Loading Themes',
    emptyTitle: 'No themes found',
    emptyBody: 'Try adjusting your filters',
    showingCount: 'Showing {shown} of {total} themes',
    applyTheme: 'Apply Theme',
    clearFilters: 'Clear Filters',
    searchLabel: 'Search themes',
    searchPlaceholder: 'Search themes...',
    sync: 'Sync',
    syncTitleInline: 'Force refresh themes from GitHub (Admin)',
    syncLabel: 'Force refresh themes from GitHub',
    syncTitle: 'Force refresh themes (Admin)',
    closeLabel: 'Close theme marketplace',
  },
  // Sample copy the credit-roll theme preview renders so a streamer can see a
  // theme applied to something. It mirrors the real credits overlay.
  creditRollPreview: {
    heading: '🎬 Stream Credits',
    subheading: 'Thank you for your support!',
    leaderboardHeading: 'Top Subscribers',
    footerHeading: 'Thank you! ❤️',
    footerBody: 'See you next stream!',
  },
} as const
