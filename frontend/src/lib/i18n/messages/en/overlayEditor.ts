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
  filters: {
    blockedUsernames: 'Blocked usernames',
    blockedUsernamesPlaceholder: 'Type username, press Enter',
    addCommonBots: 'Add common bots',
    blockedKeywords: 'Blocked keywords',
    blockedKeywordsPlaceholder: 'Type keyword or regex, press Enter',
    removeTag: 'Remove {tag}',
    hideCommands: 'Hide bot commands (!)',
    // sectionRegistry.ts duplicates this label so the editor settings search
    // can index it while the section is unmounted, and sectionRegistry.test.ts
    // fails if the two disagree.
    hideSayHi: 'Hide YouTube "said hi" greetings',
    hideSayHiNote:
      'Only YouTube messages whose entire text is the greeting posted by the vertical-stream “Say hi!” button. Hidden messages also make no sound and are not read by TTS.',
    sayHiPhrases: 'Extra “said hi” phrases',
    sayHiPhrasesPlaceholder: 'Type phrase, press Enter',
    sayHiPhrasesNote:
      'The button’s text is localised, so add what it posts in your language — for example the German phrase.',
    minMessageLength: 'Min message length',
    // Suffixed onto the slider's number by SliderControl. The leading space is
    // the separator and is part of the copy.
    charsUnit: ' chars',
  },
  bubbleColors: {
    lockedNotice:
      'Different bubble colours per platform, or a palette cycled down the feed, are a {emphasis} feature.',
    lockedNoticeEmphasis: 'Premium',
    perPlatformHeading: 'Per platform',
    perPlatformBody:
      'Tell sources apart at a glance. Unset platforms keep the normal bubble background.',
    resetPlatform: 'Reset {platform} bubble colour',
    paletteHeading: 'Palette',
    paletteBody:
      'Two or more colours, cycled down the feed. A row keeps its colour while it is on screen. Needs at least two to take effect.',
    swatchLabel: 'Colour {index}',
    removeSwatch: 'Remove colour {index}',
    addSwatch: 'Add colour',
    singleSwatchNote:
      'One colour behaves the same as Bubble background. Add a second to start cycling.',
  },
  sounds: {
    scopeNote:
      'These sounds play on your public OBS overlay, for everyone watching your stream. Want a private alert only you hear when new activity arrives (channel-point redeems, a TikTok Rose, and so on)? Open the Monitor view and turn on Activity sound in its Display menu. That is a separate setting and stays on that device.',
    enable: 'Enable notification sounds',
    preset: 'Sound preset',
    // Suffixed with the soundPlayer preset name the option writes. Casing rules
    // are language-specific, so the display name cannot be derived from the
    // stored lowercase value.
    presetChime: 'Chime',
    presetPop: 'Pop',
    presetPing: 'Ping',
    volume: 'Volume',
    test: 'Test sound',
    cooldown: 'Cooldown',
    // Suffixed onto the slider's number; the leading space is the separator.
    millisecondsUnit: ' ms',
    customUrl: 'Custom sound URL',
    customUrlUpsell: '— Upload your own notification sound ({emphasis})',
    customUrlUpsellEmphasis: 'Premium',
    customUrlPlaceholder: 'https://example.com/sound.mp3',
  },
  tts: {
    // Rendered by SubSectionHeader, which does not transform the text: the
    // upper case is in the copy, and a language with different casing rules
    // needs to be able to change it.
    sectionVoice: 'VOICE',
    sectionAdvanced: 'ADVANCED (ELEVENLABS)',
    sectionThrottling: 'THROTTLING',
    sectionContent: 'CONTENT',
    sectionPriority: 'PRIORITY',
    enable: 'Enable text-to-speech',
    unsupported: 'This browser does not support text-to-speech.',
    provider: 'Voice provider',
    providerBrowser: 'Browser (free)',
    providerElevenLabs: 'ElevenLabs (premium)',
    // Follows a <PremiumUpsellLink />, which carries its own copy. One key
    // rather than three: the three sites were byte-identical.
    upsellSuffix: 'to use ElevenLabs voices.',
    browserVoice: 'Voice',
    browserVoiceDefault: 'Default',
    browserVoiceNote: 'Browser voice — list depends on your OS/browser.',
    volume: 'Volume',
    rate: 'Speech rate',
    pitch: 'Pitch',
    test: 'Test voice',
    filterMode: 'Which messages are spoken',
    // Suffixed with the tts_filter_mode value the option writes.
    filterModeAll: 'All',
    filterModeSample: 'Sample',
    filterModePriorityOnly: 'Priority-only',
    sampleRate: 'Sample rate',
    sampleRateNote: 'Chance a non-priority message is spoken.',
    maxQueue: 'Max queue length',
    messagesPerMinute: 'Messages per minute',
    userCooldown: 'Per-user cooldown',
    staleness: 'Drop messages older than',
    // Suffixed onto a NumberControl's number; the leading space is the
    // separator and is part of the copy.
    secondsUnit: ' s',
    charsUnit: ' chars',
    readUsername: 'Read username',
    readPlatform: 'Read platform name',
    maxMessageLength: 'Max message length',
    skipEmoteOnly: 'Skip emote-only messages',
    skipLinks: 'Skip link-only messages',
    platforms: 'Platforms',
    priorityEvents: 'Announce priority events',
    priorityBitsMin: 'Minimum bits to announce',
    apiKeyLabel: 'ElevenLabs API key',
    apiKeyPlaceholder: 'sk-...',
    apiKeyEncrypted: 'Your key is encrypted server-side and never returned.',
    saveKey: 'Save key',
    savingKey: 'Saving…',
    keySaved: 'Key saved and encrypted. Click Test key to verify.',
    testKey: 'Test key',
    testingKey: 'Testing…',
    removeKey: 'Remove key',
    removingKey: 'Removing…',
    confirmRemoveKey: 'Confirm remove',
    // One sentence with all three numbers in it. It rendered as five sibling
    // nodes, which a language ordering the count after the unit cannot
    // reassemble. `remaining` and `limit` arrive pre-formatted.
    quota: '{remaining} / {limit} characters this month ({percent}%)',
    quotaUnknown: 'Click Test key to see your remaining quota.',
    elevenLabsVoice: 'ElevenLabs voice',
    voicesLoading: 'Loading voices…',
    voicesError: 'Could not load voices',
    voicesPending: 'Voices will load shortly…',
    voicesNeedKey: 'Enter your API key above to load voices.',
    voicesEmpty: 'No voices available',
    saveVoice: 'Save voice',
    savingVoice: 'Saving voice…',
    obsUrlNote: 'Paste this URL into OBS as your browser source to enable ElevenLabs TTS.',
    obsUrlLabel: 'OBS URL — copy and paste into OBS browser source',
    copyObsUrl: 'Copy OBS URL',
    regenerateObsUrl: 'Regenerate URL',
    regeneratingObsUrl: 'Regenerating…',
    regenerateConfirmTitle: 'Regenerate OBS URL?',
    regenerateConfirmBody:
      'This invalidates the current OBS URL. You’ll need to paste the new URL into OBS.',
    cancel: 'Cancel',
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
