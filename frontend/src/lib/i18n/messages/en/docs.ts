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
  // /dev/theme-contrast, the developer harness the WCAG contrast gate measures.
  themeContrast: {
    heading: 'Theme contrast harness',
    // One sentence: 'themes.' was a JSX run after the number, which a language
    // putting the noun first cannot move.
    intro: 'Dev-only. Renders every bundled theme for the message-text WCAG gate. {count} themes.',
  },
} as const
