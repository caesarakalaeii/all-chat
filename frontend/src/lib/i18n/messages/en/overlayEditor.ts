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
    // The preset display names are in common.soundPresets.*: the monitor view's
    // private activity sound renders the same three.
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
  // The list of platforms feeding this overlay, and the per-source card.
  sources: {
    chatViaEventsub: 'Chat via EventSub',
    reconnectChat: 'Reconnect to enable chat',
    revoke: 'Revoke',
    configureRelay: 'Configure relay',
    streamSelection: 'Stream selection',
    removeLabel: 'Remove {channel}',
    removeConfirmTitle: 'Remove source?',
    // {emphasis} is the channel name, which the render site wraps in <strong>.
    removeConfirmBody:
      'Remove {emphasis} from this overlay. Chat messages from this source will stop appearing.',
    remove: 'Remove',
    cancel: 'Cancel',
    empty: 'No sources added yet. Add a platform below.',
    sharedHeading: 'Shared Overlays',
    sharedOwner: "{owner}'s overlay",
    add: '+ Add',
  },
  // YouTube channels can have several concurrent live streams; this panel picks
  // which one the overlay follows.
  streamSelection: {
    strategyLabel: 'Stream selection strategy',
    strategyHint:
      'When this channel has multiple concurrent live streams, choose which one to monitor.',
    // Appended to a non-default option's label while the account is not
    // premium. The leading space is part of the copy.
    premiumSuffix: ' (Premium)',
    // Follows the <PremiumUpsellLink /> element, so it opens with a space.
    upsellSuffix: ' to use advanced stream selection.',
    locked: 'Non-default strategies require a premium subscription.',
    matchLabel: 'Title keyword',
    matchPlaceholder: 'e.g. synthwave, lofi, jazz',
    firstFoundLabel: 'First found',
    firstFoundDescription: 'Picks the first live stream (default)',
    mostViewersLabel: 'Most viewers',
    mostViewersDescription: 'Picks the stream with the highest viewer count',
    fewestViewersLabel: 'Fewest viewers',
    fewestViewersDescription: 'Picks the stream with the lowest viewer count',
    titleMatchLabel: 'Title match',
    titleMatchDescription: 'Picks the first stream whose title contains a keyword',
    titleMatchAllLabel: 'Title match (all)',
    titleMatchAllDescription: 'Monitors all streams whose title contains a keyword',
    allLabel: 'All streams',
    allDescription: 'Monitors all concurrent live streams simultaneously',
  },
  // Relaying All-Chat messages back out to a Discord channel.
  relay: {
    loopFilter: 'Loop filter: active — Discord messages are never relayed back to Discord.',
    enable: 'Enable relay',
    outboundChannelLabel: 'Outbound channel',
    selectChannel: 'Select a channel...',
  },
  // Viewer points, the earn-rate grid, the OBS poll/prediction widgets and the
  // Twitch native-mirroring opt-in. Saved against the engagement service, so
  // this panel is independent of Save Configuration.
  engagement: {
    loadError: 'Could not load engagement settings. Reload the page to try again.',
    enablePoints: 'Enable viewer points',
    announceRounds: 'Announce new rounds in chat',
    announceRoundsHint:
      'Posts the question, numbered options and the participate link to your chat when a round starts. Needs the “advanced controls” send permission (the same grant the Monitor view’s chat sending uses) — without it the announcement is skipped.',
    // The four command placeholders are chat commands, not copy. The render site
    // re-wraps each in <code>; see the copy lock for why the sentence is whole.
    pointsExplainer:
      'Viewers earn {pointsName} by supporting the stream (subs, bits, donations, gifts) and by keeping the participation page open, and wager them on predictions. Run polls and predictions from the Monitor View; viewers join straight from chat ({voteCommand} or just {bareVote}, {predictCommand}) or the participation page — no install required.',
    pointsNameLabel: 'Points name',
    pointsNamePlaceholder: 'Points',
    save: 'Save Engagement Settings',
    saving: 'Saving...',
    bitsMultiplierLabel: 'Points per bit',
    bitsMultiplierHint: 'Twitch cheers',
    usdMultiplierLabel: 'Points per USD',
    usdMultiplierHint: 'donations & Super Chats',
    subHighLabel: 'Tier 3 sub',
    subHighHint: 'Twitch Tier 3',
    subMediumLabel: 'Tier 2 sub',
    subMediumHint: 'Twitch Tier 2',
    subLowLabel: 'Base sub / member',
    subLowHint: 'Tier 1, Prime, Kick & YouTube members',
    giftPerSubLabel: 'Per gifted sub',
    giftPerSubHint: 'awarded to the gifter',
    chatPerMinuteLabel: 'Chatting, per minute',
    chatPerMinuteHint: 'active chatters',
    watchPerMinuteLabel: 'Participation page, per min',
    watchPerMinuteHint: 'while the viewer keeps the participate page open (not stream-watch time)',
    // Appended to a hint for a dimension that has no producer yet. The leading
    // space is part of the copy.
    comingSoonSuffix: ' (coming soon)',
    invalidValue: 'Invalid value for "{field}"',
    mustBeWhole: '"{field}" must be a whole number',
    linksHeading: 'Widget & viewer links',
    pollWidgetLabel: 'OBS poll widget',
    pollWidgetDescription: 'Browser source that shows the live poll',
    predictionWidgetLabel: 'OBS prediction widget',
    predictionWidgetDescription: 'Browser source that shows the live prediction',
    participateLabel: 'Viewer participation page',
    participateDescription: 'Viewers vote, wager and check their balance — no install needed',
    copyLink: 'Copy link',
    copiedLink: 'Copied!',
    copyLinkFailed: 'Could not copy the link',
    browserSourceHint:
      'In OBS/Streamlabs: add a {emphasis}, paste a widget URL, and set it to your canvas size (e.g. 1920×1080). The widgets are transparent and only appear while a round is live.',
    browserSourceHintEmphasis: 'Browser Source',
    participateShareHint:
      'Share the participation link with mobile viewers — put it on-screen or in your channel panels so they can join without the extension.',
    mirroringHeading: 'Twitch native mirroring',
    mirroringBody:
      'Mirror your native Twitch polls & predictions onto All-Chat overlays (read-only — viewers still vote in Twitch). Opt-in; it adds read-only Twitch scopes and takes effect after the next channel sync (a stream restart or re-adding the source).',
    enableMirroring: 'Enable Twitch mirroring',
  },
  // Attaching a new platform to the overlay: the OAuth buttons, the Discord
  // guild/channel dialogs, the TikTok username dialog and the admin escape hatch.
  addSource: {
    intro: 'Connect a platform to this overlay.',
    connectTwitch: 'Connect Twitch',
    connectYoutube: 'Connect YouTube',
    connectKick: 'Connect Kick',
    connectTiktok: 'Connect TikTok',
    connectDiscord: 'Connect Discord',
    // {emphasis} is the link to /settings.
    discordNeedsServer: 'Connect a Discord server in {emphasis} first to add Discord sources.',
    discordNeedsServerEmphasis: 'Settings',
    channelLabel: 'Channel',
    selectChannel: 'Select a channel...',
    back: 'Back',
    cancel: 'Cancel',
    add: 'Add',
    // Two spellings of the same word, deliberately. The Discord dialog rendered
    // three dots and the TikTok one a real ellipsis; unifying them would change
    // what is on screen, which this migration does not do.
    addingEllipsis: 'Adding...',
    adding: 'Adding\u2026',
    tiktokTitle: 'Connect TikTok',
    // The apostrophes are U+0027: the render site wrote &apos;, not ’.
    tiktokBody:
      "TikTok has no login step here. Enter the creator's username and we'll pull their live chat.",
    tiktokPlaceholder: '@username',
    adminSummary: 'Admin: manual channel ID',
    adminYoutubePlaceholder: '@handle, channel URL, or UC\u2026',
    adminChannelPlaceholder: 'Channel ID or username',
    adminAdd: 'Add manually',
    adminResolving: 'Resolving\u2026',
  },
  // The editor page itself: its load states, the header actions, the OBS URL
  // controls, the browser-extension card and the two overlay-sharing dialogs.
  page: {
    loadingEditor: 'Loading editor...',
    notFound: 'Overlay not found',
    returnToDashboard: 'Return to Dashboard',
    back: 'Back',
    monitorView: 'Monitor View',
    monitorViewTitle: 'Open the readable chat & activity monitor in a new tab',
    eventSettings: 'Event Settings',
    credits: 'Credits',
    clone: 'Clone',
    cloning: 'Cloning\u2026',
    copyObsUrl: 'Copy OBS URL',
    copiedObsUrl: 'Copied!',
    obsHelpTrigger: 'How do I add this to OBS?',
    obsHelpTitle: 'Add the overlay to OBS',
    shareOverlay: 'Share Overlay',
    resetToThemeDefaults: 'Reset to theme defaults',
    extensionHeading: 'Browser Extension Overlay',
    extensionActive: 'Active',
    extensionActiveBody:
      'This overlay is shown to viewers via the browser extension at allch.at/c/caesarlp.',
    extensionInactiveBody: 'Set this as the overlay shown to viewers via the browser extension.',
    extensionDeactivate: 'Deactivate',
    extensionSetActive: 'Set Active',
    premiumRequiredTitle: 'Premium Feature',
    // {upgradeLink} and {discordLink} are rendered as links by the call site,
    // with their text coming from the two sibling keys below.
    premiumRequiredBody:
      'Sharing your overlay is a premium feature. {upgradeLink} to share your chat with other streamers.',
    premiumUpgradeLink: 'Upgrade your account',
    questionsJoin: 'Questions? Join our {discordLink}.',
    discordCommunityLink: 'Discord community',
    close: 'Close',
    upgrade: 'Upgrade',
    shareTitle: 'Share Overlay',
    // {emphasis} is the overlay name, which the render site emboldens. The
    // apostrophe is U+0027: the render site wrote &apos;.
    shareBody:
      "Enter the Twitch username of the person you want to share {emphasis} with. They'll receive a request they can accept or decline.",
    shareRecipientLabel: 'Twitch username',
    shareRecipientPlaceholder: 'e.g. somestreamer',
    shareCancel: 'Cancel',
    shareSend: 'Send Request',
    shareSending: 'Sending...',
    applyThemeTitle: 'Apply theme?',
    applyThemeBody: 'Loading this theme will reset your visual customizations. Continue?',
    applyThemeCancel: 'Cancel',
    applyThemeContinue: 'Continue',
    saveConfiguration: 'Save Configuration',
    savingConfiguration: 'Saving...',
  },
  // Injecting a fake chat message so a streamer can see their overlay react.
  testing: {
    platformLabel: 'Platform',
    displayNameLabel: 'Display Name',
    usernameLabel: 'Username',
    avatarUrlLabel: 'Avatar URL',
    avatarUrlPlaceholder: 'https://...',
    nameColorLabel: 'Name Color',
    messageLabel: 'Message',
    messagePlaceholder: 'Type something fun...',
    injectMessage: 'Inject Message',
    // The emoji is part of the label as it renders, so it stays in the string.
    sampleChat: '\u{1F4AC} Sample Chat',
    sampleEvents: '\u2B50 Sample Events',
  },
  // Resetting the overlay ID, which revokes every OBS URL pointing at it.
  dangerZone: {
    explainer:
      'Reset your overlay ID to revoke any leaked OBS URLs. A new overlay with the same configuration will be created and you will be redirected to it. The old overlay and its URL will be permanently deleted.',
    resetOverlayId: 'Reset Overlay ID',
    resetting: 'Resetting\u2026',
    confirmTitle: 'Reset Overlay ID?',
    confirmBody:
      'This will create a new overlay with a fresh ID and permanently delete this one. Any existing OBS URLs will stop working — update your browser source after the reset.',
    cancel: 'Cancel',
    confirmReset: 'Reset ID',
  },
  // The Messages section: how many messages show, how long they last, which
  // edge the feed grows from, how they animate in and which emote sets apply.
  messages: {
    // {value} is rendered in a coloured <span>, so the label stays one sentence.
    maxMessagesLabel: 'Max Messages: {value}',
    messageDurationLabel: 'Message Duration: {value}',
    // The unit is inside the same emphasised span as the number.
    durationSeconds: '{seconds}s',
    disableFade: 'Disable Message Fade Out',
    disableFadeHint: 'Messages stay visible until max is reached',
    invertOrder: 'Invert Message Order',
    invertOrderHint:
      'Reverses the reading order so the newest message is listed first. This is the order only — use Feed Anchor to move the feed to the other edge.',
    feedAnchorLabel: 'Feed Anchor',
    feedAnchorTop: 'Top edge — feed grows downward',
    feedAnchorBottom: 'Bottom edge — feed grows upward',
    feedAnchorHint:
      'Which edge of the overlay the feed sits on when it is not full. Anchor it to the bottom and each new message pushes the older ones up.',
    entryAnimationLabel: 'Entry Animation',
    entryAnimationHint: 'How new messages appear on the overlay',
    animationDefault: 'Fade + slide up (default)',
    animationFlyLeft: 'Fly in from left',
    animationFlyRight: 'Fly in from right',
    animationFlySpring: 'Fly in with overshoot',
    animationPop: 'Pop in',
    animationBounce: 'Bounce up',
    animationFlip: 'Flip in',
    animationSwoosh: 'Swoosh',
    animationSoftFocus: 'Soft focus',
    emoteProvidersLabel: 'Emote Providers',
    // Third-party product names. Not translated, but held here so no render site
    // carries a literal and a language that transliterates names has somewhere
    // to do it.
    seventv: '7TV',
    betterttv: 'BetterTTV',
    frankerfacez: 'FrankerFaceZ',
    seventvOverrideLabel: '7TV Emote Set',
    seventvOverrideHint:
      'Optional. Paste a 7TV emote-set ID, an emote-set URL, or your 7TV profile URL to attach those emotes to this overlay regardless of which platforms you stream on.',
    // The trailing space separates this label from the set name beside it.
    seventvCurrentlyActive: 'Currently active: ',
    seventvQuotedName: '"{name}"',
    seventvEmoteCount: ' ({count} emotes)',
    seventvRemove: 'Remove',
    seventvRemoving: 'Removing\u2026',
    seventvRemoved: '7TV emote set removed',
    seventvRemoveFailed: 'Failed to remove 7TV emote set',
    seventvReplacePlaceholder: 'Paste a new ID/URL to replace\u2026',
    seventvUrlPlaceholder: 'https://7tv.app/users/...',
    seventvVerify: 'Verify',
    seventvChecking: 'Checking\u2026',
    seventvResolveFailed: 'Could not resolve 7TV reference',
    // Four whole sentences rather than one built from optional fragments. The
    // name and count are each optional at the call site, so all four
    // combinations are reachable and each is a sentence a translator can order.
    seventvResolved: 'Resolved — click Save Configuration to apply.',
    seventvResolvedNamed: 'Resolved to "{name}" — click Save Configuration to apply.',
    seventvResolvedCounted: 'Resolved ({count} emotes) — click Save Configuration to apply.',
    seventvResolvedNamedCounted:
      'Resolved to "{name}" ({count} emotes) — click Save Configuration to apply.',
  },
  // The Custom CSS section: the theme-linkage pills, the Monaco editor and the
  // problem summary the CSS language service feeds.
  customCss: {
    heading: 'Custom CSS',
    usingTheme: 'Using “{theme}” theme · auto-updates',
    noThemeApplied: 'No theme applied',
    customPill: 'Custom CSS',
    forkedPill: 'Full copy saved — theme updates paused',
    layeredPill: 'Customized — untouched theme rules still auto-update',
    resetToTheme: 'Reset to theme',
    clear: 'Clear',
    explainer:
      'Edit the CSS below — the preview updates as you type. Only your changes are saved, so fixes we ship to the theme still reach the rules you didn’t touch. Deleting theme rules can’t be layered, so it stores a full copy and pauses theme updates for this overlay; “Reset to theme” re-links it.',
    editorPlaceholder: '/* Enter your custom CSS here */',
    noProblems: '✓ No CSS problems detected.',
    // Singular and plural are separate keys rather than a stem plus an appended
    // 's': that rule is English-specific.
    errorCountOne: '{count} error',
    errorCountMany: '{count} errors',
    warningCountOne: '{count} warning',
    warningCountMany: '{count} warnings',
    problemsSeparator: ' · ',
    // {counts} is the joined error and warning phrases.
    problemsAdvice:
      '{counts} — invalid rules are ignored by the browser, so fix these for your styles to take effect. Incomplete rules aren’t previewed.',
    issueLine: 'L{line}:',
    moreIssues: '…and {count} more',
    // {docsLink} is rendered as a link to the theme docs on GitHub.
    inspiration: 'Need inspiration? Explore {docsLink}.',
    themeDocsLink: 'theme docs',
  },
  // The credit-roll settings page: which events count, how the leaderboards are
  // ranked and displayed, Twitch clips, and its own custom CSS editor.
  credits: {
    loadingEditor: 'Loading editor...',
    notFound: 'Overlay not found',
    returnToDashboard: 'Return to Dashboard',
    backToOverlay: 'Back to Overlay',
    heading: 'Credit Roll Settings',
    intro:
      'Configure end-of-stream credits to showcase viewers who supported your stream with subs, donations, raids, and more.',
    copyObsUrl: 'Copy Credits OBS URL',
    copiedObsUrl: 'Copied!',
    obsUrlHint: 'Add this URL as a Browser Source in OBS to display credits at end of stream',
    enableHeading: 'Enable Credit Roll',
    enableHint: 'Show end-of-stream credits with leaderboards and highlights',
    eventTypesHeading: 'Event Types to Include',
    eventTypesHint: 'Select which types of events should appear in the credit roll leaderboards',
    eventSubs: 'Subscriptions',
    eventResubs: 'Resubscriptions',
    eventGiftSubs: 'Gift Subs',
    eventBits: 'Bits/Cheers',
    eventRaids: 'Raids',
    eventSuperChats: 'Super Chats',
    eventMemberships: 'Memberships',
    eventFollows: 'Follows',
    leaderboardHeading: 'Leaderboard Settings',
    topNLabel: 'Top N Users per Category',
    topNHint: 'Show top 1-50 users in each leaderboard category',
    sortByLabel: 'Sort By',
    sortByTotalValue: 'Total Value (monetary amount)',
    sortByCount: 'Count (number of events)',
    displayHeading: 'Display Settings',
    themeLabel: 'Theme',
    themeClassic: 'Classic',
    themeCinematic: 'Cinematic',
    themeModern: 'Modern',
    scrollSpeedLabel: 'Scroll Speed (1-100)',
    // Shared by the scroll-speed and opacity sliders.
    currentValue: 'Current: {value}',
    durationLabel: 'Display Duration (seconds)',
    durationHint: 'How long to show the credit roll (10-300 seconds)',
    opacityLabel: 'Background Opacity (0-1)',
    clipsHeading: 'Twitch Clips',
    clipsHint: 'Show clips during credit roll',
    maxClipsLabel: 'Maximum Clips',
    fallbackDaysLabel: 'Fallback Days',
    fallbackDaysHint: 'If no clips from this stream, show clips from last N days',
    muteClipsLabel: 'Mute Clips Audio',
    muteClipsHint: 'Required for browser autoplay. Unmuting may require viewer interaction.',
    cssHeading: 'Custom CSS Editor',
    cssEnable: 'Enable Custom CSS',
    cssBrowseThemes: 'Browse Themes',
    cssReset: 'Reset',
    cssEditorPlaceholder: '/* Enter your custom CSS for credit roll */',
    // {docsLink} is rendered as a link to the credit-roll theme docs.
    cssHint:
      'Customize your credit roll appearance with CSS. Browse themes or write your own styles. See {docsLink} for examples and CSS selectors.',
    cssDocsLink: 'credit roll theme docs',
    save: 'Save Settings',
    saving: 'Saving...',
    cancel: 'Cancel',
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
  // The /overlays/[id]/events page: which platform events reach the overlay.
  // Distinct from `events` above, which is the appearance group that sizes them.
  //
  // Each toggle's label and description are keyed by the EventSettings field it
  // writes, with the enable_ prefix dropped and the rest camel-cased, so the
  // render site's tables carry a key stem instead of copy.
  eventSettings: {
    back: 'Back to Overlay',
    heading: 'Event Display Settings',
    subheading: 'Control which platform events appear on your overlay.',
    loadFailed: 'Failed to load event settings',
    save: 'Save Settings',
    saving: 'Saving…',
    cancel: 'Cancel',
    // The four platform tabs read common.platforms.*; only this one is local.
    tabGlobal: 'Global',
    systemEventsHeading: 'System Events',
    tokenWarningsLabel: 'Token Warnings',
    tokenWarningsDescription:
      'Display OAuth authentication errors on overlay (requires token-refresh-service)',
    displaySettingsHeading: 'Display Settings',
    durationMultiplierLabel: 'Event Duration Multiplier',
    durationMultiplierDescription:
      'Multiply all event display durations (0.5 = half time, 2.0 = double time)',
    tiersHeading: 'About event tiers',
    // {tier} is emphasised at the render site. Each bullet stays one sentence:
    // the em dash is mid-clause, not a join between two independent fragments.
    tierHigh: '• {tier} — subs, large donations, raids: 30+ seconds',
    tierHighName: 'High-value',
    tierMedium: '• {tier} — follows, small gifts: 15 seconds',
    tierMediumName: 'Medium-value',
    tierLow: '• {tier} — likes, shares: 5–10 seconds',
    tierLowName: 'Low-value',
    // The two class names are protocol, so they arrive as placeholders.
    tierStyling: '• Style with CSS classes: {tierClass}, {typeClass}',
    twitchSubsLabel: 'Subscriptions',
    twitchSubsDescription: 'New subscriptions and resubscriptions',
    twitchResubsLabel: 'Resubscriptions',
    twitchResubsDescription: 'Monthly resubscription notices with streak information',
    twitchGiftSubsLabel: 'Gift Subscriptions',
    twitchGiftSubsDescription: 'Gift subs and mystery gift bombs',
    twitchBitsLabel: 'Bits / Cheers',
    twitchBitsDescription: 'Bits cheered in chat',
    twitchRaidsLabel: 'Raids',
    twitchRaidsDescription: 'Incoming raids from other channels',
    twitchChannelPointsLabel: 'Channel Points',
    twitchChannelPointsDescription: 'Channel point reward redemptions (requires EventSub service)',
    twitchFollowsLabel: 'Follows',
    twitchFollowsDescription: 'New channel followers (requires EventSub service)',
    twitchWatchStreaksLabel: 'Watch Streaks',
    twitchWatchStreaksDescription:
      "Returning viewers' watch-streak milestones. Turning this off hides the milestone only — their chat message still shows",
    youtubeSuperChatLabel: 'Super Chat',
    youtubeSuperChatDescription: 'Paid Super Chat messages',
    youtubeSuperStickerLabel: 'Super Stickers',
    youtubeSuperStickerDescription: 'Paid Super Sticker purchases',
    youtubeMembersLabel: 'New Members',
    youtubeMembersDescription: 'New channel memberships',
    youtubeMemberMilestonesLabel: 'Member Milestones',
    youtubeMemberMilestonesDescription: 'Membership anniversary celebrations',
    youtubeMemberGiftsLabel: 'Membership Gifts',
    youtubeMemberGiftsDescription: 'Gifted memberships',
    kickSubsLabel: 'Subscriptions',
    kickSubsDescription: 'Kick channel subscriptions',
    // The render site spelled the ampersand &amp;; a catalog string is not HTML.
    kickGiftsLabel: 'Gifts & Donations',
    kickGiftsDescription: 'Gift subscriptions and donations',
    kickCaveat: '⚠️ Kick events require reverse-engineering and may not be available yet.',
    tiktokLikesLabel: 'Likes',
    tiktokLikesDescription: 'Likes sent during stream (aggregated)',
    tiktokGiftsLabel: 'Gifts',
    tiktokGiftsDescription: 'Virtual gifts sent with diamond values',
    tiktokFollowsLabel: 'Follows',
    tiktokFollowsDescription: 'New followers during stream',
    tiktokSharesLabel: 'Shares',
    tiktokSharesDescription: 'Stream shares to other platforms',
    tiktokTreasureChestsLabel: 'Coin Chests',
    // The "best effort" caveat is load bearing — see the comment on the toggle
    // at the render site for what has and has not been observed.
    tiktokTreasureChestsDescription:
      'Treasure boxes of coins dropped by viewers. Best effort: TikTok does not reliably send these to third-party tools, so they may not appear.',
    advancedHeading: 'Advanced',
    likeWindowLabel: 'Like Aggregation Window (seconds)',
    likeWindowDescription: 'Likes are collected in this window to prevent spam',
  },
  // /overlays/new: the name-only create form.
  create: {
    heading: 'Create Overlay',
    body: 'Give your overlay a name. You can add chat sources after creation.',
    nameLabel: 'Overlay Name',
    namePlaceholder: 'e.g. Main Stream, TikTok Only',
    nameRequired: 'Overlay name is required',
    cancel: 'Cancel',
    submit: 'Create Overlay',
  },
  // /overlays/[id]/preview/embed. Chat message content is never copy — only the
  // chrome around it, which here is one empty state.
  embedPreview: {
    // Three full stops, not an ellipsis: transcribed as the render site had it.
    waitingHeading: 'Waiting for messages...',
    waitingBody: 'Messages will appear here when chat is active',
  },
  // Toasts raised from the editor routes.
  toasts: {
    // The name is quoted in the original, U+0022 either side.
    created: '"{name}" created',
    createFailed: 'Failed to create overlay',
    eventSettingsSaved: 'Event settings saved',
    eventSettingsSaveFailed: 'Failed to save event settings',
  },
} as const
