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
 * The in-app documentation pages.
 */

export const docs = {
  // FieldTable in components/docs/prose.tsx, shared by both /docs pages.
  fieldTable: {
    columnField: 'Field',
    columnType: 'Type',
    columnDescription: 'Description',
  },
  // /docs/api, the developer WebSocket reference. Wire field names, event type
  // names and the code samples are not copy and stay at the render site: a
  // translated field name names a field the gateway does not send.
  api: {
    eyebrow: 'Developer API',
    heading: 'Developer API reference',
    intro:
      'Read the unified chat stream over one WebSocket to build bots, moderation tools, vote counters, analytics, alerts — anything.',
    guidePrompt: 'Just setting up your overlay? {guide}',
    guideLinkText: 'See the streamer guide →',
    tocHeading: 'On this page',
    tocConnectATool: 'Connect a tool',
    tocMessageFormat: 'Message format',
    tocChatMessages: 'Chat messages',
    tocEvents: 'Events',
    tocStatusMessages: 'Status & control messages',
    tocReconnecting: 'Reconnecting & heartbeats',
    tocTryItNow: 'Try it now (no setup)',
    connectHeading: 'Connect a tool',
    connectBody:
      'All-Chat aggregates live chat from {twitch}, {youtube}, {kick}, {tiktok} and {discord} into a single normalized stream — every message, plus platform events like subs, bits, raids, super chats and gifts, in one format, with 7TV/BTTV/FFZ emotes resolved for you. The same stream that powers the browser overlay is the one your tool reads.',
    connectAnonymous:
      'Open a WebSocket to the overlay endpoint. Reading the stream is {anonymous} — no token or account is required (this is the same "OBS mode" the browser overlay uses):',
    connectAnonymousEmphasis: 'anonymous',
    connectOverlayId:
      "Your {field} is the UUID in your overlay's browser-source URL. Want to try without an account first? Use the {testOverlay}.",
    connectTestOverlayLinkText: 'public test overlay',
    queryParamsHeading: 'Query parameters',
    exampleJavascriptHeading: 'Minimal example (JavaScript)',
    examplePythonHeading: 'Minimal example (Python)',
    formatHeading: 'Message format',
    formatBody: 'Every frame is JSON with the same envelope:',
    formatTypeList: '{field} is one of:',
    chatHeading: 'Chat messages',
    chatBody: 'For {chat} and {update}, {data} is the unified message object:',
    userHeading: 'user',
    messageHeading: 'message, emotes, and attachments',
    messageBody: '{field} is {shape}. Each {emote}:',
    attachmentsBody:
      '{field} is present only when a message carries media (Discord image/GIF/video uploads and Tenor/Giphy link previews today). Each {attachment}:',
    badgeBody: 'Each {badge} is {shape} — e.g. {example}.',
    exampleHeading: 'Example',
    eventsHeading: 'Events',
    eventsBody:
      'Platform events (subs, bits, raids, donations, …) arrive as {chat} frames whose {data} includes an {event} object. Normal chat has no {event} field.',
    eventTypesHeading: 'Event types by platform',
    // One label per list item. What follows it is a list of wire event-type
    // names, which are not copy.
    eventTypesLabel: '{platform}:',
    exampleSubscriptionHeading: 'Example (Twitch subscription)',
    statusHeading: 'Status & control messages',
    statusBody:
      'On connect the server sends a {connected} frame, then a {status} frame per configured source so your UI can show indicators immediately. {status} data:',
    reconnectHeading: 'Reconnecting & heartbeats',
    reconnectPing:
      'The server periodically sends {ping}. Reply with {pong}. (Most WebSocket libraries also answer low-level ping frames automatically.)',
    reconnectBackfill:
      'If the socket drops, reconnect with backoff. To backfill messages missed during a brief disconnect, reconnect with {since} set to just before you lost the connection — the server replays buffered messages newer than that timestamp.',
    reconnectDedup:
      'Treat IDs as the dedup key when combining the replay buffer with live messages.',
    tryItHeading: 'Try it now (no setup)',
    tryItBody:
      'There is a public test overlay that streams realistic fake traffic — random chat, poll votes (the literal messages {one}, {two}, {three}, {four}) and platform events — so you can build and validate an integration without an account or a real channel. Just connect; traffic flows while a client is connected and stops when the last one disconnects:',
    tryItOutro:
      'Drop that URL into either example above and you should immediately see messages and events stream in.',
    // This footer is the docs chrome, not the legal chrome: it links the
    // streamer guide alongside the two policies. legal.layout.* is a different
    // surface with a different link set.
    footerCopyright: '© {year} All-Chat',
    footerGuideLink: 'Streamer guide',
    footerPrivacyLink: 'Privacy Policy',
    footerTermsLink: 'Terms of Service',
  },
  // The FieldTable row descriptions on /docs/api. These live in module-scope
  // object literals, which the lint gate cannot see, so they are easy to miss.
  apiFields: {
    // The two connect query parameters.
    queryParamsToken:
      'Owner token. Only needed for owner-scoped access; omit it for read-only consumption.',
    queryParamsSince:
      'Milliseconds since the Unix epoch. On connect, the server replays buffered messages newer than this timestamp so a reconnecting client can backfill the gap. Omit to start live.',
    // The three frame-envelope fields.
    envelopeType: 'Message type (see below).',
    envelopeData: 'Payload; shape depends on type. Omitted for ping/pong.',
    envelopeTimestamp: 'RFC 3339 timestamp of when the gateway sent the frame.',
    // The six frame types.
    messageTypesChatMessage:
      'A chat message or a platform event. data is the unified message object.',
    messageTypesMessageUpdate:
      'An update to a previously sent message (e.g. TikTok like aggregates). Same data shape as chat_message.',
    messageTypesConnected: 'Sent once on connect: { overlay_id, message }.',
    messageTypesPlatformStatus: 'Connection status of a source platform.',
    messageTypesPing: 'Heartbeat from the server. Reply with { "type": "pong" }.',
    messageTypesError: 'Error notice: { code, message }.',
    // The unified message object.
    chatMessageId: 'Unique message ID.',
    chatMessageOverlayId: 'Overlay this message was delivered to.',
    chatMessagePlatform: '"twitch" | "youtube" | "kick" | "tiktok" | "discord".',
    chatMessageChannelId: 'Platform channel identifier.',
    chatMessageChannelName: 'Human-readable channel name.',
    chatMessageUser: 'Author info (see below).',
    chatMessageMessage: '{ text, emotes[], attachments[]? } (see below).',
    chatMessageTimestamp: 'RFC 3339 message time (UTC).',
    chatMessageMetadata: 'Free-form, platform-specific extras.',
    chatMessageEvent:
      'Present only when the message is a platform event (see Events). Absent for normal chat.',
    // The message author.
    userId: 'Platform user ID.',
    userUsername: 'Login/handle.',
    userDisplayName: 'Display name.',
    userColor: 'Name color, hex (e.g. "#FF0000").',
    userBadges: 'Author badges (see below).',
    userAvatarUrl: 'Profile image URL when available.',
    userPronouns: 'e.g. "she/her" when known.',
    userNameGradient: 'Optional gradient descriptor (JSON string).',
    userSourceBadgesSourceUserId: 'Origin-channel identity for shared-chat messages.',
    userAvatarFrameUrlAvatarFlairUrl: 'Cosmetic frame/flair when set.',
    // One resolved emote.
    emoteCode: 'Emote text token, e.g. "Kappa".',
    emoteProvider: '"twitch" | "7tv" | "bttv" | "ffz" | "discord" | platform.',
    emoteUrl: 'CDN image URL.',
    emotePositions: 'Array of [start, end] index pairs into text where the emote occurs.',
    // One media attachment.
    attachmentType: '"image" or "video". GIFs are images that animate.',
    attachmentUrl: 'Media URL.',
    attachmentContentType: 'MIME type, e.g. "image/gif", "video/mp4".',
    attachmentWidthHeight: 'Intrinsic pixel dimensions when known.',
    attachmentThumbUrl: 'Poster frame for videos when available.',
    attachmentSpoiler: 'True when the sender marked the media a spoiler.',
    attachmentFilename: 'Original filename (used for alt text).',
    // The platform-event object.
    eventType: 'Event type (see list below).',
    eventTier: 'Relative prominence: "high" | "medium" | "low".',
    eventValue:
      '{ amount: number, currency: string, display_text: string } — e.g. amount 250, currency "bits", display_text "250 bits".',
    eventDuration: 'Suggested on-screen display time, seconds.',
    eventIsUpdate:
      'true when this updates a prior event (e.g. TikTok like aggregates, delivered as message_update).',
    eventAggregationId: 'Groups successive updates of the same aggregate.',
    eventMetadata: 'Event-specific raw fields.',
    // The platform_status payload.
    platformStatusPlatform: 'Source platform.',
    platformStatusChannelId: 'Source channel.',
    platformStatusChannelName: 'Human-readable channel name.',
    platformStatusStatus: '"connected" | "reconnecting" | "offline" | "quota_exceeded".',
    platformStatusNextRetryAt: 'RFC 3339 time of the next reconnect attempt, when applicable.',
    platformStatusErrorMessage: 'Human-readable detail when degraded.',
  },
  // /docs, the streamer guide. Product-surface names (Rediscover, Monitor View,
  // Engagement) are copy: they are what the reader sees in the UI, so they move
  // when the UI moves. CSS selectors, custom-property names and chat commands
  // are not, and stay at the render site.
  guide: {
    eyebrow: 'All-Chat Docs',
    heading: 'Streamer guide',
    intro:
      'Set it up in OBS, run polls, predictions and moderation from the chat monitor, and make it your own.',
    apiPrompt: 'Building a bot, alert box, or analytics tool? {api}',
    apiLinkText: 'See the Developer API →',
    tocHeading: 'On this page',
    tocWhatIsAllChat: 'What is All-Chat',
    tocGettingStarted: 'Get your overlay live',
    tocIrl: '24/7 & IRL streams',
    tocMonitor: 'The chat monitor',
    tocModeration: 'Moderate your chat',
    tocEngagement: 'Polls, predictions & points',
    tocEventsCredits: 'Events & credit roll',
    tocSharing: 'Share an overlay',
    tocThemes: 'Pick a theme',
    tocCustomize: 'Make it your own',
    tocCustomCss: 'Go further with CSS',
    tocFonts: 'Custom fonts',
    tocPremium: 'Premium',
    whatIsHeading: 'What is All-Chat',
    whatIsBody:
      'All-Chat pulls your live chat from {twitch}, {youtube}, {kick}, {tiktok} and {discord} into a single overlay you drop into OBS. Every message lands in one feed — plus events like subs, bits, raids, super chats and gifts — and 7TV, BTTV and FFZ emotes show up automatically. No bots to invite, no chat widget to wire up.',
    startHeading: 'Get your overlay live',
    startSignIn: 'Sign in at {home} with your Twitch, YouTube, or Kick account.',
    startSignInLinkText: 'allch.at',
    startCreate:
      'In the dashboard, create an {overlay} and connect the platforms you stream on. Each connected platform becomes a {source} on that overlay — mix and match as many as you like.',
    startCreateOverlayEmphasis: 'overlay',
    startCreateSourceEmphasis: 'chat source',
    startObs:
      "Copy the overlay's browser-source URL and add it to OBS as a {browserSource}. Chat appears the moment the overlay connects.",
    startObsEmphasis: 'Browser Source',
    startDemandDriven:
      'Sources are demand-driven: All-Chat starts listening when the overlay is open and winds down when nothing is connected, so you never pay for idle listeners.',
    irlHeading: '24/7 & IRL streams',
    irlIntro:
      "Running an OBS instance around the clock — a common setup for IRL streamers who want disconnect protection — needs one extra step so YouTube chat behaves. Add {passiveParam} to your overlay's browser-source URL:",
    irlExplainer:
      'A {passive} overlay renders chat exactly like a normal one, but it does not ask All-Chat to start capturing on its own. That matters because YouTube discovery gives up after an hour of not finding a live stream (so it never hammers YouTube for an offline channel). A normal 24/7 overlay would trip that timeout while you are offline and sit parked by the time you go live — a passive overlay never starts that clock, so nothing gets stuck.',
    irlExplainerEmphasis: 'passive',
    irlWhenLiveHeading: 'When you go live',
    irlStepPassiveUrl: 'Leave your 24/7 OBS browser source on the passive URL.',
    irlStepOpenMonitor: "Open your overlay's {monitor} (the monitor / view page).",
    irlStepOpenMonitorEmphasis: 'chat monitor',
    irlStepRediscover:
      'If chat is not already flowing, hit {rediscover} on the monitor — capture starts within about a minute.',
    irlStepRediscoverEmphasis: 'Rediscover',
    irlStepKeepOpen:
      'Keep the chat monitor open while you stream; that keeps capture running for the session, and it winds down a few minutes after you close it.',
    irlRefreshNote:
      "A plain browser-source refresh does {negation} restart a parked YouTube channel — use the monitor's {rediscover} button. While a channel is parked, its platform dot shows an {paused} state (waiting for you to trigger it), not a red error.",
    irlRefreshNoteNegationEmphasis: 'not',
    irlRefreshNotePausedEmphasis: 'indigo “paused”',
    monitorHeading: 'The chat monitor',
    monitorIntro:
      "Open {monitorView} from the overlay editor for a live control room, separate from the OBS overlay. It shows every message in one panel with an activity feed beside it, and it's where you send messages and moderate.",
    monitorIntroEmphasis: 'Monitor View',
    monitorSend:
      '{send} as yourself to Twitch, YouTube or Kick straight from the monitor (TikTok and Discord have no send API).',
    monitorSendEmphasis: 'Send messages',
    monitorRediscover:
      '{rediscover} forces a fresh look for your live stream — use it if YouTube chat stops after a crash or restart, or to start capture for a {passiveOverlay} when you go live.',
    monitorRediscoverEmphasis: 'Re-discover YouTube',
    monitorPassiveOverlayEmphasis: 'passive 24/7 overlay',
    monitorDisplay:
      "{display} settings toggle what you see (avatars, badges, pronouns, timestamps, platform icons, moderation controls). These are your personal monitor preferences and don't change the OBS overlay.",
    monitorDisplayEmphasis: 'Display',
    moderationHeading: 'Moderate your chat',
    moderationBody:
      'With {controls} on in the monitor, hover a message to {del} it, or open a chatter to {timeout}, {ban} or {unban} them — applied on the source platform: Twitch, Kick and Discord do delete, timeout, ban and unban; YouTube does timeout and ban; TikTok has no moderation API.',
    moderationControlsEmphasis: 'Moderation controls',
    moderationDeleteEmphasis: 'Delete',
    moderationTimeoutEmphasis: 'Timeout',
    moderationBanEmphasis: 'Ban',
    moderationUnbanEmphasis: 'Unban',
    moderationEnable:
      'The first time, use {enable} to grant the extra permissions (for Discord you re-invite the bot). Moderating from your overlay is a {premium} feature.',
    moderationEnableEmphasis: 'Enable moderation & chat sending',
    engagementHeading: 'Polls, predictions & points',
    engagementIntro:
      'All-Chat has its own {polls}, {predictions} and {points} that work across every connected platform, not just Twitch.',
    engagementPollsEmphasis: 'polls',
    engagementPredictionsEmphasis: 'predictions',
    engagementPointsEmphasis: 'viewer points',
    engagementSetup:
      "Turn them on in the editor's {section} section: {enablePoints}, set your {pointsName}, and choose how points are earned (per bit, per sub, per gifted sub, and so on).",
    engagementSectionEmphasis: 'Engagement',
    engagementEnablePointsEmphasis: 'Enable viewer points',
    engagementPointsNameEmphasis: 'Points name',
    engagementRun:
      "Run rounds live from the monitor's {section} panel: {startPoll} / {closePoll}, and {startPrediction} / {lockWagers} / pay out or {cancelRefund}.",
    engagementStartPollEmphasis: 'Start poll',
    engagementClosePollEmphasis: 'Close poll',
    engagementStartPredictionEmphasis: 'Start prediction',
    engagementLockWagersEmphasis: 'Lock wagers',
    engagementCancelRefundEmphasis: 'Cancel & refund',
    engagementJoin:
      'Viewers join from chat (e.g. {voteCommand}, {predictCommand}) or on the {participationPage}.',
    engagementParticipationPageEmphasis: 'Viewer participation page',
    engagementWidgets:
      'Add the {pollWidget} and {predictionWidget} as their own browser sources so results show on stream — copy their URLs from the Engagement section.',
    engagementPollWidgetEmphasis: 'OBS poll widget',
    engagementPredictionWidgetEmphasis: 'OBS prediction widget',
    eventsHeading: 'Events & credit roll',
    eventsBody:
      'Your overlay shows {events} too — subs, resubs, gift subs, bits/cheers, raids, Super Chats, memberships and follows. Use {eventSettings} in the editor to choose exactly which events appear, per platform.',
    eventsEmphasis: 'events',
    eventSettingsEmphasis: 'Event Settings',
    creditsBody:
      "For the end of a stream, open {credits} to set up a {creditRoll} — a scrolling thank-you of your top subscribers, gifters, cheerers, raiders and new followers. It's its own browser source ({copyUrl}).",
    creditsEmphasis: 'Credits',
    creditRollEmphasis: 'Credit Roll',
    creditsCopyUrlEmphasis: 'Copy Credits OBS URL',
    sharingHeading: 'Share an overlay',
    sharingBody:
      "{share} lets another streamer pull your overlay's chat into theirs — handy for collabs and raids. Send a request to their Twitch username; once they accept, your overlay appears in their editor under {sharedOverlays} to add as a source, and either of you can revoke it later. Sharing is a {premium} feature.",
    sharingShareEmphasis: 'Share Overlay',
    sharingSharedOverlaysEmphasis: 'Shared Overlays',
    themesHeading: 'Pick a theme',
    themesIntro:
      'All-Chat ships with {count} — from Modern Dark and Minimal to Trading Card, Comic Speech, Sticky Notes, Vaporwave, Cyberpunk and more. Applying one takes a click and needs {noCss}.',
    themesCountEmphasis: '16 built-in themes',
    themesNoCssEmphasis: 'no CSS at all',
    themesStepOpen: 'Open your overlay in the dashboard to edit it.',
    themesStepApply:
      'In the {theme} section, browse the themes and apply one. The preview updates instantly.',
    themesStepApplyEmphasis: 'Theme',
    themesStepSave: 'Save. Your OBS browser source picks up the new look on its next refresh.',
    themesPreview: 'You can also preview every theme live on the {home} before you sign in.',
    themesPreviewLinkText: 'home page',
    customizeHeading: 'Make it your own',
    customizeIntro:
      "A theme is a starting point — most of the time you can get exactly the look you want{noCode}. In the overlay editor's {appearance} panel you can adjust, with sliders, toggles and color pickers:",
    customizeNoCodeEmphasis: ' without writing any code',
    customizeAppearanceEmphasis: 'Appearance',
    customizeFont: 'Font family, text size, weight, line height and letter spacing',
    customizeSpacing: 'Spacing, padding and message-bubble corners',
    customizeAvatar: 'Avatar and badge size',
    customizeVisibility: 'What to show or hide — avatars, badges, timestamps, usernames',
    customizeColors: 'Colors, including a name color per platform',
    customizeEvents: 'How events like subs, raids and super chats appear',
    customizeOutro:
      "Start from the theme closest to what you want, then tweak these until it feels like yours. If you need something the panel doesn't cover, drop down to CSS below.",
    cssHeading: 'Go further with CSS',
    cssIntro:
      "For full control, enable {customCss} in the overlay editor's {expert} section and write your own styles. A few things worth knowing:",
    cssCustomCssEmphasis: 'Custom CSS',
    cssExpertEmphasis: 'Expert',
    cssScope: 'Your CSS only affects your overlay — nothing else.',
    cssOrder:
      'It loads {after} the theme, so you can start from a built-in theme and override just the parts you want.',
    cssOrderEmphasis: 'after',
    cssPreview: "There's a live preview right next to the editor, so you see changes as you type.",
    cssVarsHeading: 'Quick wins: style variables',
    cssVarsIntro:
      'The easiest lever needs no knowledge of class names. Set any of these variables on {root} and the overlay picks them up:',
    cssVarsColumnVariable: 'Variable',
    cssVarsColumnDefault: 'Default',
    cssVarsColumnEffect: 'What it changes',
    cssHooksHeading: 'Finer control: target the chat parts',
    cssHooksIntro:
      "For anything the variables don't cover, style the overlay's elements directly. These class names and data attributes are stable and safe to target:",
    cssHooksColumnSelector: 'Selector',
    cssHooksColumnKind: 'Kind',
    cssHooksColumnTargets: 'Targets',
    cssFeedAnchor:
      '{feedAnchor} (Messages settings) decides which {edge} the feed rests on — anchor it to the bottom and new messages push the older ones upward, with the blank space collecting at the top. {invertOrder} is a separate setting for a separate axis: it only changes which {endOfList} is newest. All four combinations work.',
    cssFeedAnchorEmphasis: 'Feed Anchor',
    cssFeedAnchorEdgeEmphasis: 'edge',
    cssInvertOrderEmphasis: 'Invert Message Order',
    cssEndOfListEmphasis: 'end of the list',
    cssExampleCaption: 'Example — give each platform its own accent stripe:',
    cssCallout:
      "Want ready-made examples? Every built-in theme is just a CSS file you can read and borrow from on {github}. Already have a theme from another tool, or want help writing one? Ask in our {discord} and we're happy to help.",
    cssCalloutGithubLinkText: 'GitHub',
    fontsHeading: 'Custom fonts',
    fontsIntro:
      "You can pull in a Google Font with a normal {importRule}, then use it anywhere in your CSS. To protect your viewers' privacy, fonts are served through All-Chat rather than directly from Google, so only these families are available:",
    fontsFamilies:
      'Barlow, Barlow Condensed, Bebas Neue, Exo 2, Inter, Monoton, Montserrat, Nunito, Open Sans, Orbitron, Oswald, Poppins, Press Start 2P, Rajdhani, Roboto, Share Tech Mono, Source Code Pro, Space Grotesk, VT323.',
    fontsOutro:
      "Requesting a family that isn't on the list simply won't load — pick another or leave the default.",
    premiumHeading: 'Premium',
    premiumIntro:
      'All-Chat is free and open source. A Premium subscription — via Patreon, connected in {settings} — unlocks:',
    premiumSettingsEmphasis: 'Settings → Premium',
    premiumModeration: '{label} — delete, timeout and ban from the chat monitor.',
    premiumModerationEmphasis: 'Moderate from your overlay',
    premiumTts: '{label} — premium TTS voices (basic browser TTS is free).',
    premiumTtsEmphasis: 'ElevenLabs text-to-speech',
    premiumYoutube:
      '{label} — choose which stream to follow on multi-stream channels (the free default follows the first one found).',
    premiumYoutubeEmphasis: 'YouTube stream selection',
    premiumSharedChat: '{label} — share your overlay with other streamers.',
    premiumSharedChatEmphasis: 'Shared chat',
    premiumFlairs: '{label} — animated gradient name colors.',
    premiumFlairsEmphasis: 'Viewer flairs',
    footerCopyright: '© {year} All-Chat',
    footerApiLink: 'Developer API',
    footerPrivacyLink: 'Privacy Policy',
    footerTermsLink: 'Terms of Service',
  },
  // The style-variable reference table. Module-scope rows, invisible to the
  // lint gate, so a literal here would have stayed behind silently.
  guideCssVars: {
    chatFontSize: 'Message text size.',
    chatFontFamily: 'Message text font.',
    chatMessageColor: 'Message text color.',
    chatMessageGap: 'Vertical space between messages.',
    chatBubbleBorderRadius: 'Roundness of the message bubble.',
    chatBubblePadding: 'Padding inside each message.',
    chatAvatarSize: 'Avatar width and height.',
    chatUsernameFontSize: 'Username size.',
    chatEmoteScale: 'Emote size multiplier.',
    chatShowAvatars: 'Set to none to hide avatars.',
    chatShowBadges: 'Set to none to hide badges.',
    chatShowTimestamps: 'Set to none to hide timestamps.',
  },
  // The CSS hook reference table. Same reasoning as guideCssVars.
  guideCssHooks: {
    overlayLiveBody: 'The whole message list container.',
    chatMessage: 'One chat message bubble.',
    chatUsername: 'The author name.',
    platformBadge: 'The platform tag/icon next to the name.',
    eventMessage: 'A sub / raid / super chat / gift alert.',
    dataPlatformTwitch: 'Messages from a platform (twitch, youtube, kick, tiktok, discord).',
    dataUsername: 'Messages from a specific user.',
    dataFeedAnchorTopBottom:
      'The overlay wrapper, carrying the Feed Anchor setting. Read it to adapt; don’t override the wrapper’s flex-direction or the list’s margin-top, which are what move the feed.',
    dataFeedOrderNewestLastNewestFirst:
      'The overlay wrapper, carrying the Invert Message Order setting. It also flips --msg-enter-dir and --msg-enter-origin, so entry animations come in from the end the newest message lands on.',
  },
  // /dev/theme-contrast, the developer harness the WCAG contrast gate measures.
  themeContrast: {
    heading: 'Theme contrast harness',
    // One sentence: 'themes.' was a JSX run after the number, which a language
    // putting the noun first cannot move.
    intro: 'Dev-only. Renders every bundled theme for the message-text WCAG gate. {count} themes.',
  },
} as const
