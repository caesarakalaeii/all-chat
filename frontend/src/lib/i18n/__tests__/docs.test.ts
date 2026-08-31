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
 * Copy lock for the documentation surfaces.
 *
 * The migration's one hard rule is that copy moves byte-identically: no
 * rewording, no re-capitalising, no normalised punctuation. A rendered-output
 * diff across 229 files is not reviewable, so the strings that were at the
 * render sites are pinned here instead, transcribed from the pre-migration
 * source. If a key's text drifts, this fails and names the key.
 */

import { describe, expect, it } from 'vitest'

import { getTranslations } from '@/lib/i18n'

const t = getTranslations()

describe('docs field table copy', () => {
  it('keeps the three column headers', () => {
    // FieldTable is the shared field-reference table used by both /docs pages.
    expect(t('docs.fieldTable.columnField')).toBe('Field')
    expect(t('docs.fieldTable.columnType')).toBe('Type')
    expect(t('docs.fieldTable.columnDescription')).toBe('Description')
  })
})

describe('theme contrast harness copy', () => {
  it('keeps the dev harness header', () => {
    // /dev/theme-contrast is a developer tool, so its copy sits with the other
    // developer-facing surfaces rather than in a user namespace.
    expect(t('docs.themeContrast.heading')).toBe('Theme contrast harness')
    // One sentence with the count as a param: 'themes.' was a separate JSX run
    // after the number, which a language that puts the noun first cannot move.
    expect(t('docs.themeContrast.intro', { count: 34 })).toBe(
      'Dev-only. Renders every bundled theme for the message-text WCAG gate. 34 themes.'
    )
  })
})

describe('developer API reference copy', () => {
  it('keeps the page header and its guide cross-link', () => {
    expect(t('docs.api.eyebrow')).toBe('Developer API')
    expect(t('docs.api.heading')).toBe('Developer API reference')
    expect(t('docs.api.intro')).toBe(
      'Read the unified chat stream over one WebSocket to build bots, moderation tools, vote counters, analytics, alerts — anything.'
    )
    // 'Just setting up your overlay?' and the link were two JSX runs; the link
    // text is a sibling key so the sentence stays whole.
    expect(t('docs.api.guidePrompt', { guide: 'See the streamer guide →' })).toBe(
      'Just setting up your overlay? See the streamer guide →'
    )
    expect(t('docs.api.guideLinkText')).toBe('See the streamer guide →')
  })

  it('keeps the table of contents', () => {
    expect(t('docs.api.tocHeading')).toBe('On this page')
    expect(t('docs.api.tocConnectATool')).toBe('Connect a tool')
    expect(t('docs.api.tocMessageFormat')).toBe('Message format')
    expect(t('docs.api.tocChatMessages')).toBe('Chat messages')
    expect(t('docs.api.tocEvents')).toBe('Events')
    expect(t('docs.api.tocStatusMessages')).toBe('Status & control messages')
    expect(t('docs.api.tocReconnecting')).toBe('Reconnecting & heartbeats')
    expect(t('docs.api.tocTryItNow')).toBe('Try it now (no setup)')
  })

  it('keeps the connect-a-tool section', () => {
    expect(t('docs.api.connectHeading')).toBe('Connect a tool')
    // The five platform names were <strong> runs inside the sentence, so they
    // are placeholders resolved from common.platforms.*.
    expect(
      t('docs.api.connectBody', {
        twitch: 'Twitch',
        youtube: 'YouTube',
        kick: 'Kick',
        tiktok: 'TikTok',
        discord: 'Discord',
      })
    ).toBe(
      'All-Chat aggregates live chat from Twitch, YouTube, Kick, TikTok and Discord into a single normalized stream — every message, plus platform events like subs, bits, raids, super chats and gifts, in one format, with 7TV/BTTV/FFZ emotes resolved for you. The same stream that powers the browser overlay is the one your tool reads.'
    )
    expect(t('docs.api.connectAnonymous', { anonymous: 'anonymous' })).toBe(
      'Open a WebSocket to the overlay endpoint. Reading the stream is anonymous — no token or account is required (this is the same "OBS mode" the browser overlay uses):'
    )
    expect(t('docs.api.connectAnonymousEmphasis')).toBe('anonymous')
    expect(
      t('docs.api.connectOverlayId', { field: 'overlay_id', testOverlay: 'public test overlay' })
    ).toBe(
      "Your overlay_id is the UUID in your overlay's browser-source URL. Want to try without an account first? Use the public test overlay."
    )
    expect(t('docs.api.connectTestOverlayLinkText')).toBe('public test overlay')
    expect(t('docs.api.queryParamsHeading')).toBe('Query parameters')
    expect(t('docs.api.exampleJavascriptHeading')).toBe('Minimal example (JavaScript)')
    expect(t('docs.api.examplePythonHeading')).toBe('Minimal example (Python)')
  })

  it('keeps the message-format section', () => {
    expect(t('docs.api.formatHeading')).toBe('Message format')
    expect(t('docs.api.formatBody')).toBe('Every frame is JSON with the same envelope:')
    expect(t('docs.api.formatTypeList', { field: 'type' })).toBe('type is one of:')
  })

  it('keeps the chat-messages section', () => {
    expect(t('docs.api.chatHeading')).toBe('Chat messages')
    expect(
      t('docs.api.chatBody', { chat: 'chat_message', update: 'message_update', data: 'data' })
    ).toBe('For chat_message and message_update, data is the unified message object:')
    expect(t('docs.api.userHeading')).toBe('user')
    expect(t('docs.api.messageHeading')).toBe('message, emotes, and attachments')
    expect(
      t('docs.api.messageBody', {
        field: 'message',
        shape: '{ "text": string, "emotes": Emote[], "attachments"?: Attachment[] }',
        emote: 'Emote',
      })
    ).toBe(
      'message is { "text": string, "emotes": Emote[], "attachments"?: Attachment[] }. Each Emote:'
    )
    expect(t('docs.api.attachmentsBody', { field: 'attachments', attachment: 'Attachment' })).toBe(
      'attachments is present only when a message carries media (Discord image/GIF/video uploads and Tenor/Giphy link previews today). Each Attachment:'
    )
    expect(
      t('docs.api.badgeBody', {
        badge: 'Badge',
        shape: '{ "name", "version", "icon_url" }',
        example: '{ "name": "subscriber", "version": "12", "icon_url": "..." }',
      })
    ).toBe(
      'Each Badge is { "name", "version", "icon_url" } — e.g. { "name": "subscriber", "version": "12", "icon_url": "..." }.'
    )
    expect(t('docs.api.exampleHeading')).toBe('Example')
  })

  it('keeps the events section', () => {
    expect(t('docs.api.eventsHeading')).toBe('Events')
    expect(t('docs.api.eventsBody', { chat: 'chat_message', data: 'data', event: 'event' })).toBe(
      'Platform events (subs, bits, raids, donations, …) arrive as chat_message frames whose data includes an event object. Normal chat has no event field.'
    )
    expect(t('docs.api.eventTypesHeading')).toBe('Event types by platform')
    // Each list item is 'Platform: ' followed by wire event-type names, which
    // are not copy. The label is the only translatable run.
    expect(t('docs.api.eventTypesLabel', { platform: 'Twitch' })).toBe('Twitch:')
    expect(t('docs.api.exampleSubscriptionHeading')).toBe('Example (Twitch subscription)')
  })

  it('keeps the status-messages section', () => {
    expect(t('docs.api.statusHeading')).toBe('Status & control messages')
    expect(t('docs.api.statusBody', { connected: 'connected', status: 'platform_status' })).toBe(
      'On connect the server sends a connected frame, then a platform_status frame per configured source so your UI can show indicators immediately. platform_status data:'
    )
  })

  it('keeps the reconnecting section', () => {
    expect(t('docs.api.reconnectHeading')).toBe('Reconnecting & heartbeats')
    expect(
      t('docs.api.reconnectPing', { ping: '{ "type": "ping" }', pong: '{ "type": "pong" }' })
    ).toBe(
      'The server periodically sends { "type": "ping" }. Reply with { "type": "pong" }. (Most WebSocket libraries also answer low-level ping frames automatically.)'
    )
    expect(t('docs.api.reconnectBackfill', { since: '?since=<ms-epoch>' })).toBe(
      'If the socket drops, reconnect with backoff. To backfill messages missed during a brief disconnect, reconnect with ?since=<ms-epoch> set to just before you lost the connection — the server replays buffered messages newer than that timestamp.'
    )
    expect(t('docs.api.reconnectDedup')).toBe(
      'Treat IDs as the dedup key when combining the replay buffer with live messages.'
    )
  })

  it('keeps the try-it-now section', () => {
    expect(t('docs.api.tryItHeading')).toBe('Try it now (no setup)')
    // The four poll-vote numbers were <Code> runs. They are chat commands a
    // viewer types, not copy, so they stay as placeholders.
    expect(t('docs.api.tryItBody', { one: '1', two: '2', three: '3', four: '4' })).toBe(
      'There is a public test overlay that streams realistic fake traffic — random chat, poll votes (the literal messages 1, 2, 3, 4) and platform events — so you can build and validate an integration without an account or a real channel. Just connect; traffic flows while a client is connected and stops when the last one disconnects:'
    )
    expect(t('docs.api.tryItOutro')).toBe(
      'Drop that URL into either example above and you should immediately see messages and events stream in.'
    )
  })

  it('keeps the footer links', () => {
    expect(t('docs.api.footerCopyright', { year: 2026 })).toBe('© 2026 All-Chat')
    expect(t('docs.api.footerGuideLink')).toBe('Streamer guide')
    expect(t('docs.api.footerPrivacyLink')).toBe('Privacy Policy')
    expect(t('docs.api.footerTermsLink')).toBe('Terms of Service')
  })
})

describe('developer API field tables', () => {
  it('keeps the two connect query parameters', () => {
    expect(t('docs.apiFields.queryParamsToken')).toBe(
      'Owner token. Only needed for owner-scoped access; omit it for read-only consumption.'
    )
    expect(t('docs.apiFields.queryParamsSince')).toBe(
      'Milliseconds since the Unix epoch. On connect, the server replays buffered messages newer than this timestamp so a reconnecting client can backfill the gap. Omit to start live.'
    )
  })

  it('keeps the three envelope fields', () => {
    expect(t('docs.apiFields.envelopeType')).toBe('Message type (see below).')
    expect(t('docs.apiFields.envelopeData')).toBe(
      'Payload; shape depends on type. Omitted for ping/pong.'
    )
    expect(t('docs.apiFields.envelopeTimestamp')).toBe(
      'RFC 3339 timestamp of when the gateway sent the frame.'
    )
  })

  it('keeps the six frame types', () => {
    expect(t('docs.apiFields.messageTypesChatMessage')).toBe(
      'A chat message or a platform event. data is the unified message object.'
    )
    expect(t('docs.apiFields.messageTypesMessageUpdate')).toBe(
      'An update to a previously sent message (e.g. TikTok like aggregates). Same data shape as chat_message.'
    )
    expect(t('docs.apiFields.messageTypesConnected')).toBe(
      'Sent once on connect: { overlay_id, message }.'
    )
    expect(t('docs.apiFields.messageTypesPlatformStatus')).toBe(
      'Connection status of a source platform.'
    )
    expect(t('docs.apiFields.messageTypesPing')).toBe(
      'Heartbeat from the server. Reply with { "type": "pong" }.'
    )
    expect(t('docs.apiFields.messageTypesError')).toBe('Error notice: { code, message }.')
  })

  it('keeps the ten unified-message fields', () => {
    expect(t('docs.apiFields.chatMessageId')).toBe('Unique message ID.')
    expect(t('docs.apiFields.chatMessageOverlayId')).toBe('Overlay this message was delivered to.')
    expect(t('docs.apiFields.chatMessagePlatform')).toBe(
      '"twitch" | "youtube" | "kick" | "tiktok" | "discord".'
    )
    expect(t('docs.apiFields.chatMessageChannelId')).toBe('Platform channel identifier.')
    expect(t('docs.apiFields.chatMessageChannelName')).toBe('Human-readable channel name.')
    expect(t('docs.apiFields.chatMessageUser')).toBe('Author info (see below).')
    expect(t('docs.apiFields.chatMessageMessage')).toBe(
      '{ text, emotes[], attachments[]? } (see below).'
    )
    expect(t('docs.apiFields.chatMessageTimestamp')).toBe('RFC 3339 message time (UTC).')
    expect(t('docs.apiFields.chatMessageMetadata')).toBe('Free-form, platform-specific extras.')
    expect(t('docs.apiFields.chatMessageEvent')).toBe(
      'Present only when the message is a platform event (see Events). Absent for normal chat.'
    )
  })

  it('keeps the ten author fields', () => {
    expect(t('docs.apiFields.userId')).toBe('Platform user ID.')
    expect(t('docs.apiFields.userUsername')).toBe('Login/handle.')
    expect(t('docs.apiFields.userDisplayName')).toBe('Display name.')
    expect(t('docs.apiFields.userColor')).toBe('Name color, hex (e.g. "#FF0000").')
    expect(t('docs.apiFields.userBadges')).toBe('Author badges (see below).')
    expect(t('docs.apiFields.userAvatarUrl')).toBe('Profile image URL when available.')
    expect(t('docs.apiFields.userPronouns')).toBe('e.g. "she/her" when known.')
    expect(t('docs.apiFields.userNameGradient')).toBe('Optional gradient descriptor (JSON string).')
    expect(t('docs.apiFields.userSourceBadgesSourceUserId')).toBe(
      'Origin-channel identity for shared-chat messages.'
    )
    expect(t('docs.apiFields.userAvatarFrameUrlAvatarFlairUrl')).toBe(
      'Cosmetic frame/flair when set.'
    )
  })

  it('keeps the four emote fields', () => {
    expect(t('docs.apiFields.emoteCode')).toBe('Emote text token, e.g. "Kappa".')
    expect(t('docs.apiFields.emoteProvider')).toBe(
      '"twitch" | "7tv" | "bttv" | "ffz" | "discord" | platform.'
    )
    expect(t('docs.apiFields.emoteUrl')).toBe('CDN image URL.')
    expect(t('docs.apiFields.emotePositions')).toBe(
      'Array of [start, end] index pairs into text where the emote occurs.'
    )
  })

  it('keeps the seven attachment fields', () => {
    expect(t('docs.apiFields.attachmentType')).toBe(
      '"image" or "video". GIFs are images that animate.'
    )
    expect(t('docs.apiFields.attachmentUrl')).toBe('Media URL.')
    expect(t('docs.apiFields.attachmentContentType')).toBe(
      'MIME type, e.g. "image/gif", "video/mp4".'
    )
    expect(t('docs.apiFields.attachmentWidthHeight')).toBe('Intrinsic pixel dimensions when known.')
    expect(t('docs.apiFields.attachmentThumbUrl')).toBe('Poster frame for videos when available.')
    expect(t('docs.apiFields.attachmentSpoiler')).toBe(
      'True when the sender marked the media a spoiler.'
    )
    expect(t('docs.apiFields.attachmentFilename')).toBe('Original filename (used for alt text).')
  })

  it('keeps the seven event fields', () => {
    expect(t('docs.apiFields.eventType')).toBe('Event type (see list below).')
    expect(t('docs.apiFields.eventTier')).toBe('Relative prominence: "high" | "medium" | "low".')
    expect(t('docs.apiFields.eventValue')).toBe(
      '{ amount: number, currency: string, display_text: string } — e.g. amount 250, currency "bits", display_text "250 bits".'
    )
    expect(t('docs.apiFields.eventDuration')).toBe('Suggested on-screen display time, seconds.')
    expect(t('docs.apiFields.eventIsUpdate')).toBe(
      'true when this updates a prior event (e.g. TikTok like aggregates, delivered as message_update).'
    )
    expect(t('docs.apiFields.eventAggregationId')).toBe(
      'Groups successive updates of the same aggregate.'
    )
    expect(t('docs.apiFields.eventMetadata')).toBe('Event-specific raw fields.')
  })

  it('keeps the six platform_status fields', () => {
    expect(t('docs.apiFields.platformStatusPlatform')).toBe('Source platform.')
    expect(t('docs.apiFields.platformStatusChannelId')).toBe('Source channel.')
    expect(t('docs.apiFields.platformStatusChannelName')).toBe('Human-readable channel name.')
    expect(t('docs.apiFields.platformStatusStatus')).toBe(
      '"connected" | "reconnecting" | "offline" | "quota_exceeded".'
    )
    expect(t('docs.apiFields.platformStatusNextRetryAt')).toBe(
      'RFC 3339 time of the next reconnect attempt, when applicable.'
    )
    expect(t('docs.apiFields.platformStatusErrorMessage')).toBe(
      'Human-readable detail when degraded.'
    )
  })
})

describe('streamer guide copy', () => {
  it('keeps the page header and its API cross-link', () => {
    expect(t('docs.guide.eyebrow')).toBe('All-Chat Docs')
    expect(t('docs.guide.heading')).toBe('Streamer guide')
    expect(t('docs.guide.intro')).toBe(
      'Set it up in OBS, run polls, predictions and moderation from the chat monitor, and make it your own.'
    )
    expect(t('docs.guide.apiPrompt', { api: 'See the Developer API →' })).toBe(
      'Building a bot, alert box, or analytics tool? See the Developer API →'
    )
    expect(t('docs.guide.apiLinkText')).toBe('See the Developer API →')
  })

  it('keeps the table of contents', () => {
    expect(t('docs.guide.tocHeading')).toBe('On this page')
    expect(t('docs.guide.tocWhatIsAllChat')).toBe('What is All-Chat')
    expect(t('docs.guide.tocGettingStarted')).toBe('Get your overlay live')
    expect(t('docs.guide.tocIrl')).toBe('24/7 & IRL streams')
    expect(t('docs.guide.tocMonitor')).toBe('The chat monitor')
    expect(t('docs.guide.tocModeration')).toBe('Moderate your chat')
    expect(t('docs.guide.tocEngagement')).toBe('Polls, predictions & points')
    expect(t('docs.guide.tocEventsCredits')).toBe('Events & credit roll')
    expect(t('docs.guide.tocSharing')).toBe('Share an overlay')
    expect(t('docs.guide.tocThemes')).toBe('Pick a theme')
    expect(t('docs.guide.tocCustomize')).toBe('Make it your own')
    expect(t('docs.guide.tocCustomCss')).toBe('Go further with CSS')
    expect(t('docs.guide.tocFonts')).toBe('Custom fonts')
    expect(t('docs.guide.tocPremium')).toBe('Premium')
  })

  it('keeps the what-is-All-Chat section', () => {
    expect(t('docs.guide.whatIsHeading')).toBe('What is All-Chat')
    expect(
      t('docs.guide.whatIsBody', {
        twitch: 'Twitch',
        youtube: 'YouTube',
        kick: 'Kick',
        tiktok: 'TikTok',
        discord: 'Discord',
      })
    ).toBe(
      'All-Chat pulls your live chat from Twitch, YouTube, Kick, TikTok and Discord into a single overlay you drop into OBS. Every message lands in one feed — plus events like subs, bits, raids, super chats and gifts — and 7TV, BTTV and FFZ emotes show up automatically. No bots to invite, no chat widget to wire up.'
    )
  })

  it('keeps the getting-started steps', () => {
    expect(t('docs.guide.startHeading')).toBe('Get your overlay live')
    expect(t('docs.guide.startSignIn', { home: 'allch.at' })).toBe(
      'Sign in at allch.at with your Twitch, YouTube, or Kick account.'
    )
    expect(t('docs.guide.startSignInLinkText')).toBe('allch.at')
    expect(t('docs.guide.startCreate', { overlay: 'overlay', source: 'chat source' })).toBe(
      'In the dashboard, create an overlay and connect the platforms you stream on. Each connected platform becomes a chat source on that overlay — mix and match as many as you like.'
    )
    expect(t('docs.guide.startCreateOverlayEmphasis')).toBe('overlay')
    expect(t('docs.guide.startCreateSourceEmphasis')).toBe('chat source')
    expect(t('docs.guide.startObs', { browserSource: 'Browser Source' })).toBe(
      "Copy the overlay's browser-source URL and add it to OBS as a Browser Source. Chat appears the moment the overlay connects."
    )
    expect(t('docs.guide.startObsEmphasis')).toBe('Browser Source')
    expect(t('docs.guide.startDemandDriven')).toBe(
      'Sources are demand-driven: All-Chat starts listening when the overlay is open and winds down when nothing is connected, so you never pay for idle listeners.'
    )
  })

  it('keeps the 24/7 and IRL section', () => {
    expect(t('docs.guide.irlHeading')).toBe('24/7 & IRL streams')
    expect(t('docs.guide.irlIntro', { passiveParam: '?passive=true' })).toBe(
      "Running an OBS instance around the clock — a common setup for IRL streamers who want disconnect protection — needs one extra step so YouTube chat behaves. Add ?passive=true to your overlay's browser-source URL:"
    )
    expect(t('docs.guide.irlExplainer', { passive: 'passive' })).toBe(
      'A passive overlay renders chat exactly like a normal one, but it does not ask All-Chat to start capturing on its own. That matters because YouTube discovery gives up after an hour of not finding a live stream (so it never hammers YouTube for an offline channel). A normal 24/7 overlay would trip that timeout while you are offline and sit parked by the time you go live — a passive overlay never starts that clock, so nothing gets stuck.'
    )
    expect(t('docs.guide.irlExplainerEmphasis')).toBe('passive')
    expect(t('docs.guide.irlWhenLiveHeading')).toBe('When you go live')
    expect(t('docs.guide.irlStepPassiveUrl')).toBe(
      'Leave your 24/7 OBS browser source on the passive URL.'
    )
    expect(t('docs.guide.irlStepOpenMonitor', { monitor: 'chat monitor' })).toBe(
      "Open your overlay's chat monitor (the monitor / view page)."
    )
    expect(t('docs.guide.irlStepOpenMonitorEmphasis')).toBe('chat monitor')
    expect(t('docs.guide.irlStepRediscover', { rediscover: 'Rediscover' })).toBe(
      'If chat is not already flowing, hit Rediscover on the monitor — capture starts within about a minute.'
    )
    expect(t('docs.guide.irlStepRediscoverEmphasis')).toBe('Rediscover')
    expect(t('docs.guide.irlStepKeepOpen')).toBe(
      'Keep the chat monitor open while you stream; that keeps capture running for the session, and it winds down a few minutes after you close it.'
    )
    expect(
      t('docs.guide.irlRefreshNote', {
        negation: 'not',
        rediscover: 'Rediscover',
        paused: 'indigo “paused”',
      })
    ).toBe(
      "A plain browser-source refresh does not restart a parked YouTube channel — use the monitor's Rediscover button. While a channel is parked, its platform dot shows an indigo “paused” state (waiting for you to trigger it), not a red error."
    )
    expect(t('docs.guide.irlRefreshNoteNegationEmphasis')).toBe('not')
    expect(t('docs.guide.irlRefreshNotePausedEmphasis')).toBe('indigo “paused”')
  })

  it('keeps the chat monitor section', () => {
    expect(t('docs.guide.monitorHeading')).toBe('The chat monitor')
    expect(t('docs.guide.monitorIntro', { monitorView: 'Monitor View' })).toBe(
      "Open Monitor View from the overlay editor for a live control room, separate from the OBS overlay. It shows every message in one panel with an activity feed beside it, and it's where you send messages and moderate."
    )
    expect(t('docs.guide.monitorIntroEmphasis')).toBe('Monitor View')
    expect(t('docs.guide.monitorSend', { send: 'Send messages' })).toBe(
      'Send messages as yourself to Twitch, YouTube or Kick straight from the monitor (TikTok and Discord have no send API).'
    )
    expect(t('docs.guide.monitorSendEmphasis')).toBe('Send messages')
    expect(
      t('docs.guide.monitorRediscover', {
        rediscover: 'Re-discover YouTube',
        passiveOverlay: 'passive 24/7 overlay',
      })
    ).toBe(
      'Re-discover YouTube forces a fresh look for your live stream — use it if YouTube chat stops after a crash or restart, or to start capture for a passive 24/7 overlay when you go live.'
    )
    expect(t('docs.guide.monitorRediscoverEmphasis')).toBe('Re-discover YouTube')
    expect(t('docs.guide.monitorPassiveOverlayEmphasis')).toBe('passive 24/7 overlay')
    expect(t('docs.guide.monitorDisplay', { display: 'Display' })).toBe(
      "Display settings toggle what you see (avatars, badges, pronouns, timestamps, platform icons, moderation controls). These are your personal monitor preferences and don't change the OBS overlay."
    )
    expect(t('docs.guide.monitorDisplayEmphasis')).toBe('Display')
  })

  it('keeps the moderation section', () => {
    expect(t('docs.guide.moderationHeading')).toBe('Moderate your chat')
    expect(
      t('docs.guide.moderationBody', {
        controls: 'Moderation controls',
        del: 'Delete',
        timeout: 'Timeout',
        ban: 'Ban',
        unban: 'Unban',
      })
    ).toBe(
      'With Moderation controls on in the monitor, hover a message to Delete it, or open a chatter to Timeout, Ban or Unban them — applied on the source platform: Twitch, Kick and Discord do delete, timeout, ban and unban; YouTube does timeout and ban; TikTok has no moderation API.'
    )
    expect(t('docs.guide.moderationControlsEmphasis')).toBe('Moderation controls')
    expect(t('docs.guide.moderationDeleteEmphasis')).toBe('Delete')
    expect(t('docs.guide.moderationTimeoutEmphasis')).toBe('Timeout')
    expect(t('docs.guide.moderationBanEmphasis')).toBe('Ban')
    expect(t('docs.guide.moderationUnbanEmphasis')).toBe('Unban')
    expect(
      t('docs.guide.moderationEnable', {
        enable: 'Enable moderation & chat sending',
        premium: 'Premium',
      })
    ).toBe(
      'The first time, use Enable moderation & chat sending to grant the extra permissions (for Discord you re-invite the bot). Moderating from your overlay is a Premium feature.'
    )
    expect(t('docs.guide.moderationEnableEmphasis')).toBe('Enable moderation & chat sending')
  })

  it('keeps the engagement section', () => {
    expect(t('docs.guide.engagementHeading')).toBe('Polls, predictions & points')
    expect(
      t('docs.guide.engagementIntro', {
        polls: 'polls',
        predictions: 'predictions',
        points: 'viewer points',
      })
    ).toBe(
      'All-Chat has its own polls, predictions and viewer points that work across every connected platform, not just Twitch.'
    )
    expect(t('docs.guide.engagementPollsEmphasis')).toBe('polls')
    expect(t('docs.guide.engagementPredictionsEmphasis')).toBe('predictions')
    expect(t('docs.guide.engagementPointsEmphasis')).toBe('viewer points')
    expect(
      t('docs.guide.engagementSetup', {
        section: 'Engagement',
        enablePoints: 'Enable viewer points',
        pointsName: 'Points name',
      })
    ).toBe(
      "Turn them on in the editor's Engagement section: Enable viewer points, set your Points name, and choose how points are earned (per bit, per sub, per gifted sub, and so on)."
    )
    expect(t('docs.guide.engagementSectionEmphasis')).toBe('Engagement')
    expect(t('docs.guide.engagementEnablePointsEmphasis')).toBe('Enable viewer points')
    expect(t('docs.guide.engagementPointsNameEmphasis')).toBe('Points name')
    expect(
      t('docs.guide.engagementRun', {
        section: 'Engagement',
        startPoll: 'Start poll',
        closePoll: 'Close poll',
        startPrediction: 'Start prediction',
        lockWagers: 'Lock wagers',
        cancelRefund: 'Cancel & refund',
      })
    ).toBe(
      "Run rounds live from the monitor's Engagement panel: Start poll / Close poll, and Start prediction / Lock wagers / pay out or Cancel & refund."
    )
    expect(t('docs.guide.engagementStartPollEmphasis')).toBe('Start poll')
    expect(t('docs.guide.engagementClosePollEmphasis')).toBe('Close poll')
    expect(t('docs.guide.engagementStartPredictionEmphasis')).toBe('Start prediction')
    expect(t('docs.guide.engagementLockWagersEmphasis')).toBe('Lock wagers')
    expect(t('docs.guide.engagementCancelRefundEmphasis')).toBe('Cancel & refund')
    expect(
      t('docs.guide.engagementJoin', {
        voteCommand: '!vote 2',
        predictCommand: '!predict 1 500',
        participationPage: 'Viewer participation page',
      })
    ).toBe(
      'Viewers join from chat (e.g. !vote 2, !predict 1 500) or on the Viewer participation page.'
    )
    expect(t('docs.guide.engagementParticipationPageEmphasis')).toBe('Viewer participation page')
    expect(
      t('docs.guide.engagementWidgets', {
        pollWidget: 'OBS poll widget',
        predictionWidget: 'OBS prediction widget',
      })
    ).toBe(
      'Add the OBS poll widget and OBS prediction widget as their own browser sources so results show on stream — copy their URLs from the Engagement section.'
    )
    expect(t('docs.guide.engagementPollWidgetEmphasis')).toBe('OBS poll widget')
    expect(t('docs.guide.engagementPredictionWidgetEmphasis')).toBe('OBS prediction widget')
  })

  it('keeps the events and credits section', () => {
    expect(t('docs.guide.eventsHeading')).toBe('Events & credit roll')
    expect(t('docs.guide.eventsBody', { events: 'events', eventSettings: 'Event Settings' })).toBe(
      'Your overlay shows events too — subs, resubs, gift subs, bits/cheers, raids, Super Chats, memberships and follows. Use Event Settings in the editor to choose exactly which events appear, per platform.'
    )
    expect(t('docs.guide.eventsEmphasis')).toBe('events')
    expect(t('docs.guide.eventSettingsEmphasis')).toBe('Event Settings')
    expect(
      t('docs.guide.creditsBody', {
        credits: 'Credits',
        creditRoll: 'Credit Roll',
        copyUrl: 'Copy Credits OBS URL',
      })
    ).toBe(
      "For the end of a stream, open Credits to set up a Credit Roll — a scrolling thank-you of your top subscribers, gifters, cheerers, raiders and new followers. It's its own browser source (Copy Credits OBS URL)."
    )
    expect(t('docs.guide.creditsEmphasis')).toBe('Credits')
    expect(t('docs.guide.creditRollEmphasis')).toBe('Credit Roll')
    expect(t('docs.guide.creditsCopyUrlEmphasis')).toBe('Copy Credits OBS URL')
  })

  it('keeps the sharing section', () => {
    expect(t('docs.guide.sharingHeading')).toBe('Share an overlay')
    expect(
      t('docs.guide.sharingBody', {
        share: 'Share Overlay',
        sharedOverlays: 'Shared Overlays',
        premium: 'Premium',
      })
    ).toBe(
      "Share Overlay lets another streamer pull your overlay's chat into theirs — handy for collabs and raids. Send a request to their Twitch username; once they accept, your overlay appears in their editor under Shared Overlays to add as a source, and either of you can revoke it later. Sharing is a Premium feature."
    )
    expect(t('docs.guide.sharingShareEmphasis')).toBe('Share Overlay')
    expect(t('docs.guide.sharingSharedOverlaysEmphasis')).toBe('Shared Overlays')
  })

  it('keeps the themes section', () => {
    expect(t('docs.guide.themesHeading')).toBe('Pick a theme')
    expect(
      t('docs.guide.themesIntro', { count: '16 built-in themes', noCss: 'no CSS at all' })
    ).toBe(
      'All-Chat ships with 16 built-in themes — from Modern Dark and Minimal to Trading Card, Comic Speech, Sticky Notes, Vaporwave, Cyberpunk and more. Applying one takes a click and needs no CSS at all.'
    )
    expect(t('docs.guide.themesCountEmphasis')).toBe('16 built-in themes')
    expect(t('docs.guide.themesNoCssEmphasis')).toBe('no CSS at all')
    expect(t('docs.guide.themesStepOpen')).toBe('Open your overlay in the dashboard to edit it.')
    expect(t('docs.guide.themesStepApply', { theme: 'Theme' })).toBe(
      'In the Theme section, browse the themes and apply one. The preview updates instantly.'
    )
    expect(t('docs.guide.themesStepApplyEmphasis')).toBe('Theme')
    expect(t('docs.guide.themesStepSave')).toBe(
      'Save. Your OBS browser source picks up the new look on its next refresh.'
    )
    expect(t('docs.guide.themesPreview', { home: 'home page' })).toBe(
      'You can also preview every theme live on the home page before you sign in.'
    )
    expect(t('docs.guide.themesPreviewLinkText')).toBe('home page')
  })

  it('keeps the customize section', () => {
    expect(t('docs.guide.customizeHeading')).toBe('Make it your own')
    // The emphasised run opens with a space, which lives inside the <strong>
    // in the source. Preserved so the rendered text is unchanged.
    expect(
      t('docs.guide.customizeIntro', {
        noCode: ' without writing any code',
        appearance: 'Appearance',
      })
    ).toBe(
      "A theme is a starting point — most of the time you can get exactly the look you want without writing any code. In the overlay editor's Appearance panel you can adjust, with sliders, toggles and color pickers:"
    )
    expect(t('docs.guide.customizeNoCodeEmphasis')).toBe(' without writing any code')
    expect(t('docs.guide.customizeAppearanceEmphasis')).toBe('Appearance')
    expect(t('docs.guide.customizeFont')).toBe(
      'Font family, text size, weight, line height and letter spacing'
    )
    expect(t('docs.guide.customizeSpacing')).toBe('Spacing, padding and message-bubble corners')
    expect(t('docs.guide.customizeAvatar')).toBe('Avatar and badge size')
    expect(t('docs.guide.customizeVisibility')).toBe(
      'What to show or hide — avatars, badges, timestamps, usernames'
    )
    expect(t('docs.guide.customizeColors')).toBe('Colors, including a name color per platform')
    expect(t('docs.guide.customizeEvents')).toBe(
      'How events like subs, raids and super chats appear'
    )
    expect(t('docs.guide.customizeOutro')).toBe(
      "Start from the theme closest to what you want, then tweak these until it feels like yours. If you need something the panel doesn't cover, drop down to CSS below."
    )
  })

  it('keeps the custom CSS section', () => {
    expect(t('docs.guide.cssHeading')).toBe('Go further with CSS')
    expect(t('docs.guide.cssIntro', { customCss: 'Custom CSS', expert: 'Expert' })).toBe(
      "For full control, enable Custom CSS in the overlay editor's Expert section and write your own styles. A few things worth knowing:"
    )
    expect(t('docs.guide.cssCustomCssEmphasis')).toBe('Custom CSS')
    expect(t('docs.guide.cssExpertEmphasis')).toBe('Expert')
    expect(t('docs.guide.cssScope')).toBe('Your CSS only affects your overlay — nothing else.')
    expect(t('docs.guide.cssOrder', { after: 'after' })).toBe(
      'It loads after the theme, so you can start from a built-in theme and override just the parts you want.'
    )
    expect(t('docs.guide.cssOrderEmphasis')).toBe('after')
    expect(t('docs.guide.cssPreview')).toBe(
      "There's a live preview right next to the editor, so you see changes as you type."
    )
    expect(t('docs.guide.cssVarsHeading')).toBe('Quick wins: style variables')
    expect(t('docs.guide.cssVarsIntro', { root: ':root' })).toBe(
      'The easiest lever needs no knowledge of class names. Set any of these variables on :root and the overlay picks them up:'
    )
    expect(t('docs.guide.cssVarsColumnVariable')).toBe('Variable')
    expect(t('docs.guide.cssVarsColumnDefault')).toBe('Default')
    expect(t('docs.guide.cssVarsColumnEffect')).toBe('What it changes')
    expect(t('docs.guide.cssHooksHeading')).toBe('Finer control: target the chat parts')
    expect(t('docs.guide.cssHooksIntro')).toBe(
      "For anything the variables don't cover, style the overlay's elements directly. These class names and data attributes are stable and safe to target:"
    )
    expect(t('docs.guide.cssHooksColumnSelector')).toBe('Selector')
    expect(t('docs.guide.cssHooksColumnKind')).toBe('Kind')
    expect(t('docs.guide.cssHooksColumnTargets')).toBe('Targets')
    expect(
      t('docs.guide.cssFeedAnchor', {
        feedAnchor: 'Feed Anchor',
        edge: 'edge',
        invertOrder: 'Invert Message Order',
        endOfList: 'end of the list',
      })
    ).toBe(
      'Feed Anchor (Messages settings) decides which edge the feed rests on — anchor it to the bottom and new messages push the older ones upward, with the blank space collecting at the top. Invert Message Order is a separate setting for a separate axis: it only changes which end of the list is newest. All four combinations work.'
    )
    expect(t('docs.guide.cssFeedAnchorEmphasis')).toBe('Feed Anchor')
    expect(t('docs.guide.cssFeedAnchorEdgeEmphasis')).toBe('edge')
    expect(t('docs.guide.cssInvertOrderEmphasis')).toBe('Invert Message Order')
    expect(t('docs.guide.cssEndOfListEmphasis')).toBe('end of the list')
    expect(t('docs.guide.cssExampleCaption')).toBe(
      'Example — give each platform its own accent stripe:'
    )
    expect(t('docs.guide.cssCallout', { github: 'GitHub', discord: 'Discord' })).toBe(
      "Want ready-made examples? Every built-in theme is just a CSS file you can read and borrow from on GitHub. Already have a theme from another tool, or want help writing one? Ask in our Discord and we're happy to help."
    )
    expect(t('docs.guide.cssCalloutGithubLinkText')).toBe('GitHub')
  })

  it('keeps the fonts section', () => {
    expect(t('docs.guide.fontsHeading')).toBe('Custom fonts')
    expect(t('docs.guide.fontsIntro', { importRule: '@import' })).toBe(
      "You can pull in a Google Font with a normal @import, then use it anywhere in your CSS. To protect your viewers' privacy, fonts are served through All-Chat rather than directly from Google, so only these families are available:"
    )
    // The available families. A list of proper nouns, but it is rendered copy
    // and a second language may reorder or annotate it, so it is one key.
    expect(t('docs.guide.fontsFamilies')).toBe(
      'Barlow, Barlow Condensed, Bebas Neue, Exo 2, Inter, Monoton, Montserrat, Nunito, Open Sans, Orbitron, Oswald, Poppins, Press Start 2P, Rajdhani, Roboto, Share Tech Mono, Source Code Pro, Space Grotesk, VT323.'
    )
    expect(t('docs.guide.fontsOutro')).toBe(
      "Requesting a family that isn't on the list simply won't load — pick another or leave the default."
    )
  })

  it('keeps the premium section', () => {
    expect(t('docs.guide.premiumHeading')).toBe('Premium')
    expect(t('docs.guide.premiumIntro', { settings: 'Settings → Premium' })).toBe(
      'All-Chat is free and open source. A Premium subscription — via Patreon, connected in Settings → Premium — unlocks:'
    )
    expect(t('docs.guide.premiumSettingsEmphasis')).toBe('Settings → Premium')
    expect(t('docs.guide.premiumModeration', { label: 'Moderate from your overlay' })).toBe(
      'Moderate from your overlay — delete, timeout and ban from the chat monitor.'
    )
    expect(t('docs.guide.premiumModerationEmphasis')).toBe('Moderate from your overlay')
    expect(t('docs.guide.premiumTts', { label: 'ElevenLabs text-to-speech' })).toBe(
      'ElevenLabs text-to-speech — premium TTS voices (basic browser TTS is free).'
    )
    expect(t('docs.guide.premiumTtsEmphasis')).toBe('ElevenLabs text-to-speech')
    expect(t('docs.guide.premiumYoutube', { label: 'YouTube stream selection' })).toBe(
      'YouTube stream selection — choose which stream to follow on multi-stream channels (the free default follows the first one found).'
    )
    expect(t('docs.guide.premiumYoutubeEmphasis')).toBe('YouTube stream selection')
    expect(t('docs.guide.premiumSharedChat', { label: 'Shared chat' })).toBe(
      'Shared chat — share your overlay with other streamers.'
    )
    expect(t('docs.guide.premiumSharedChatEmphasis')).toBe('Shared chat')
    expect(t('docs.guide.premiumFlairs', { label: 'Viewer flairs' })).toBe(
      'Viewer flairs — animated gradient name colors.'
    )
    expect(t('docs.guide.premiumFlairsEmphasis')).toBe('Viewer flairs')
  })

  it('keeps the footer links', () => {
    expect(t('docs.guide.footerCopyright', { year: 2026 })).toBe('© 2026 All-Chat')
    expect(t('docs.guide.footerApiLink')).toBe('Developer API')
    expect(t('docs.guide.footerPrivacyLink')).toBe('Privacy Policy')
    expect(t('docs.guide.footerTermsLink')).toBe('Terms of Service')
  })
})

describe('streamer guide CSS reference tables', () => {
  it('keeps the twelve CSS custom-property descriptions', () => {
    expect(t('docs.guideCssVars.chatFontSize')).toBe('Message text size.')
    expect(t('docs.guideCssVars.chatFontFamily')).toBe('Message text font.')
    expect(t('docs.guideCssVars.chatMessageColor')).toBe('Message text color.')
    expect(t('docs.guideCssVars.chatMessageGap')).toBe('Vertical space between messages.')
    expect(t('docs.guideCssVars.chatBubbleBorderRadius')).toBe('Roundness of the message bubble.')
    expect(t('docs.guideCssVars.chatBubblePadding')).toBe('Padding inside each message.')
    expect(t('docs.guideCssVars.chatAvatarSize')).toBe('Avatar width and height.')
    expect(t('docs.guideCssVars.chatUsernameFontSize')).toBe('Username size.')
    expect(t('docs.guideCssVars.chatEmoteScale')).toBe('Emote size multiplier.')
    expect(t('docs.guideCssVars.chatShowAvatars')).toBe('Set to none to hide avatars.')
    expect(t('docs.guideCssVars.chatShowBadges')).toBe('Set to none to hide badges.')
    expect(t('docs.guideCssVars.chatShowTimestamps')).toBe('Set to none to hide timestamps.')
  })

  it('keeps the nine CSS hook descriptions', () => {
    expect(t('docs.guideCssHooks.overlayLiveBody')).toBe('The whole message list container.')
    expect(t('docs.guideCssHooks.chatMessage')).toBe('One chat message bubble.')
    expect(t('docs.guideCssHooks.chatUsername')).toBe('The author name.')
    expect(t('docs.guideCssHooks.platformBadge')).toBe('The platform tag/icon next to the name.')
    expect(t('docs.guideCssHooks.eventMessage')).toBe('A sub / raid / super chat / gift alert.')
    expect(t('docs.guideCssHooks.dataPlatformTwitch')).toBe(
      'Messages from a platform (twitch, youtube, kick, tiktok, discord).'
    )
    expect(t('docs.guideCssHooks.dataUsername')).toBe('Messages from a specific user.')
    expect(t('docs.guideCssHooks.dataFeedAnchorTopBottom')).toBe(
      'The overlay wrapper, carrying the Feed Anchor setting. Read it to adapt; don’t override the wrapper’s flex-direction or the list’s margin-top, which are what move the feed.'
    )
    expect(t('docs.guideCssHooks.dataFeedOrderNewestLastNewestFirst')).toBe(
      'The overlay wrapper, carrying the Invert Message Order setting. It also flips --msg-enter-dir and --msg-enter-origin, so entry animations come in from the end the newest message lands on.'
    )
  })
})
