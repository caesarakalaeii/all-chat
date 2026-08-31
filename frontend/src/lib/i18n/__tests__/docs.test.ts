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
