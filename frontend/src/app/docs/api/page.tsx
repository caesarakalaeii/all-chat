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

import Link from 'next/link'
import { AppNav } from '@/components/AppNav'
import { Code, Pre, FieldTable, type Field } from '@/components/docs/prose'
import { JsonLd } from '@/components/JsonLd'
import { getTranslations, type MessageKey } from '@/lib/i18n'
import { interpolateElements } from '@/lib/i18n/emphasise'

// getTranslations, not useTranslations: this is a Server Component.
const t = getTranslations()

export const metadata = {
  title: t('metadata.docsApi.title'),
  description: t('metadata.docsApi.description'),
  alternates: { canonical: '/docs/api' },
}

// Structured data: a technical reference page maps to schema.org TechArticle,
// plus a Home > Documentation > Developer API breadcrumb trail. Emitted server
// side so both are in the initial HTML for crawlers.
const techArticleLd = {
  '@context': 'https://schema.org',
  '@type': 'TechArticle',
  headline: 'All-Chat Developer API reference',
  description:
    'Read the All-Chat unified chat WebSocket stream: message format, platform events, status messages and reconnection for Twitch, YouTube, Kick, TikTok and Discord.',
  url: 'https://allch.at/docs/api',
  about: 'All-Chat unified chat WebSocket API',
}

const breadcrumbLd = {
  '@context': 'https://schema.org',
  '@type': 'BreadcrumbList',
  itemListElement: [
    { '@type': 'ListItem', position: 1, name: 'Home', item: 'https://allch.at' },
    { '@type': 'ListItem', position: 2, name: 'Documentation', item: 'https://allch.at/docs' },
    {
      '@type': 'ListItem',
      position: 3,
      name: 'Developer API',
      item: 'https://allch.at/docs/api',
    },
  ],
}

const TEST_OVERLAY_ID = '00000000-0000-4000-8000-000000000a11'

// The anchor is a URL fragment people bookmark and link to, so it stays a
// literal; only the label is copy. `as const satisfies`, not an annotation: an
// annotation widens labelKey to string and a typo stops failing tsc.
const toc = [
  { id: 'connect-a-tool', labelKey: 'docs.api.tocConnectATool' },
  { id: 'message-format', labelKey: 'docs.api.tocMessageFormat' },
  { id: 'chat-messages', labelKey: 'docs.api.tocChatMessages' },
  { id: 'events', labelKey: 'docs.api.tocEvents' },
  { id: 'status-messages', labelKey: 'docs.api.tocStatusMessages' },
  { id: 'reconnecting', labelKey: 'docs.api.tocReconnecting' },
  { id: 'try-it-now', labelKey: 'docs.api.tocTryItNow' },
] as const satisfies readonly { id: string; labelKey: MessageKey }[]

// Wire values the gateway sends. Not copy: a translated field name names a
// field that does not exist. Named here so the prose below can interpolate the
// same string the reference tables document.
const FIELD_TYPE = 'type'
const FIELD_DATA = 'data'
const FIELD_EVENT = 'event'
const FIELD_MESSAGE = 'message'
const FIELD_ATTACHMENTS = 'attachments'
const FIELD_OVERLAY_ID = 'overlay_id'
const TYPE_CHAT_MESSAGE = 'chat_message'
const TYPE_MESSAGE_UPDATE = 'message_update'
const TYPE_CONNECTED = 'connected'
const TYPE_PLATFORM_STATUS = 'platform_status'
const SHAPE_EMOTE = 'Emote'
const SHAPE_ATTACHMENT = 'Attachment'
const SHAPE_BADGE = 'Badge'
const SHAPE_MESSAGE = '{ "text": string, "emotes": Emote[], "attachments"?: Attachment[] }'
const SHAPE_BADGE_FIELDS = '{ "name", "version", "icon_url" }'
const SHAPE_BADGE_EXAMPLE = '{ "name": "subscriber", "version": "12", "icon_url": "..." }'
const FRAME_PING = '{ "type": "ping" }'
const FRAME_PONG = '{ "type": "pong" }'
const QUERY_SINCE = '?since=<ms-epoch>'

// The poll votes a viewer types into the test overlay's chat. Chat commands
// cross a process boundary, so they are not copy either.
const POLL_VOTES = ['1', '2', '3', '4'] as const

// Event type names, by platform. Same reasoning as the field names above: these
// are the literal strings the gateway puts in event.type.
const EVENT_TYPES_BY_PLATFORM = [
  {
    platformKey: 'common.platforms.twitch',
    className: 'text-twitch',
    types: [
      'subscription',
      'resubscription',
      'gift_subscription',
      'mystery_gift',
      'bits',
      'raid',
      'channel_points',
      'watch_streak',
      'announcement',
      'bits_badge_tier',
      'unraid',
      'modiversary',
      'charity_donation',
      'gift_paid_upgrade',
      'prime_paid_upgrade',
      'pay_it_forward',
      'twitch_notice',
      'message_deletion',
    ],
  },
  {
    platformKey: 'common.platforms.youtube',
    className: 'text-youtube',
    types: [
      'super_chat',
      'super_sticker',
      'new_sponsor',
      'member_milestone',
      'membership_gift',
      'gift_received',
    ],
  },
  {
    platformKey: 'common.platforms.tiktok',
    className: 'text-tiktok',
    types: ['gift', 'like_aggregate', 'follow', 'share'],
  },
] as const satisfies readonly {
  platformKey: MessageKey
  className: string
  types: readonly string[]
}[]

const QUERY_PARAMS_FIELDS: readonly Field[] = [
  { name: '?token=', type: 'JWT (optional)', descKey: 'docs.apiFields.queryParamsToken' },
  { name: '?since=', type: 'int (optional)', descKey: 'docs.apiFields.queryParamsSince' },
]

const ENVELOPE_FIELDS: readonly Field[] = [
  { name: 'type', type: 'string', descKey: 'docs.apiFields.envelopeType' },
  { name: 'data', type: 'object', descKey: 'docs.apiFields.envelopeData' },
  { name: 'timestamp', type: 'string', descKey: 'docs.apiFields.envelopeTimestamp' },
]

const MESSAGE_TYPES_FIELDS: readonly Field[] = [
  { name: 'chat_message', type: '', descKey: 'docs.apiFields.messageTypesChatMessage' },
  { name: 'message_update', type: '', descKey: 'docs.apiFields.messageTypesMessageUpdate' },
  { name: 'connected', type: '', descKey: 'docs.apiFields.messageTypesConnected' },
  { name: 'platform_status', type: '', descKey: 'docs.apiFields.messageTypesPlatformStatus' },
  { name: 'ping', type: '', descKey: 'docs.apiFields.messageTypesPing' },
  { name: 'error', type: '', descKey: 'docs.apiFields.messageTypesError' },
]

const CHAT_MESSAGE_FIELDS: readonly Field[] = [
  { name: 'id', type: 'string', descKey: 'docs.apiFields.chatMessageId' },
  { name: 'overlay_id', type: 'string', descKey: 'docs.apiFields.chatMessageOverlayId' },
  { name: 'platform', type: 'string', descKey: 'docs.apiFields.chatMessagePlatform' },
  { name: 'channel_id', type: 'string', descKey: 'docs.apiFields.chatMessageChannelId' },
  { name: 'channel_name', type: 'string', descKey: 'docs.apiFields.chatMessageChannelName' },
  { name: 'user', type: 'object', descKey: 'docs.apiFields.chatMessageUser' },
  { name: 'message', type: 'object', descKey: 'docs.apiFields.chatMessageMessage' },
  { name: 'timestamp', type: 'string', descKey: 'docs.apiFields.chatMessageTimestamp' },
  { name: 'metadata', type: 'object', descKey: 'docs.apiFields.chatMessageMetadata' },
  { name: 'event', type: 'object?', descKey: 'docs.apiFields.chatMessageEvent' },
]

const USER_FIELDS: readonly Field[] = [
  { name: 'id', type: 'string', descKey: 'docs.apiFields.userId' },
  { name: 'username', type: 'string', descKey: 'docs.apiFields.userUsername' },
  { name: 'display_name', type: 'string', descKey: 'docs.apiFields.userDisplayName' },
  { name: 'color', type: 'string?', descKey: 'docs.apiFields.userColor' },
  { name: 'badges', type: 'Badge[]', descKey: 'docs.apiFields.userBadges' },
  { name: 'avatar_url', type: 'string?', descKey: 'docs.apiFields.userAvatarUrl' },
  { name: 'pronouns', type: 'string?', descKey: 'docs.apiFields.userPronouns' },
  { name: 'name_gradient', type: 'string?', descKey: 'docs.apiFields.userNameGradient' },
  {
    name: 'source_badges / source_user_id',
    type: '?',
    descKey: 'docs.apiFields.userSourceBadgesSourceUserId',
  },
  {
    name: 'avatar_frame_url / avatar_flair_url',
    type: 'string?',
    descKey: 'docs.apiFields.userAvatarFrameUrlAvatarFlairUrl',
  },
]

const EMOTE_FIELDS: readonly Field[] = [
  { name: 'code', type: 'string', descKey: 'docs.apiFields.emoteCode' },
  { name: 'provider', type: 'string', descKey: 'docs.apiFields.emoteProvider' },
  { name: 'url', type: 'string', descKey: 'docs.apiFields.emoteUrl' },
  { name: 'positions', type: 'int[][]', descKey: 'docs.apiFields.emotePositions' },
]

const ATTACHMENT_FIELDS: readonly Field[] = [
  { name: 'type', type: 'string', descKey: 'docs.apiFields.attachmentType' },
  { name: 'url', type: 'string', descKey: 'docs.apiFields.attachmentUrl' },
  { name: 'content_type', type: 'string?', descKey: 'docs.apiFields.attachmentContentType' },
  { name: 'width / height', type: 'int?', descKey: 'docs.apiFields.attachmentWidthHeight' },
  { name: 'thumb_url', type: 'string?', descKey: 'docs.apiFields.attachmentThumbUrl' },
  { name: 'spoiler', type: 'bool?', descKey: 'docs.apiFields.attachmentSpoiler' },
  { name: 'filename', type: 'string?', descKey: 'docs.apiFields.attachmentFilename' },
]

const EVENT_FIELDS: readonly Field[] = [
  { name: 'type', type: 'string', descKey: 'docs.apiFields.eventType' },
  { name: 'tier', type: 'string', descKey: 'docs.apiFields.eventTier' },
  { name: 'value', type: 'object?', descKey: 'docs.apiFields.eventValue' },
  { name: 'duration', type: 'int', descKey: 'docs.apiFields.eventDuration' },
  { name: 'is_update', type: 'bool', descKey: 'docs.apiFields.eventIsUpdate' },
  { name: 'aggregation_id', type: 'string?', descKey: 'docs.apiFields.eventAggregationId' },
  { name: 'metadata', type: 'object?', descKey: 'docs.apiFields.eventMetadata' },
]

const PLATFORM_STATUS_FIELDS: readonly Field[] = [
  { name: 'platform', type: 'string', descKey: 'docs.apiFields.platformStatusPlatform' },
  { name: 'channel_id', type: 'string', descKey: 'docs.apiFields.platformStatusChannelId' },
  { name: 'channel_name', type: 'string?', descKey: 'docs.apiFields.platformStatusChannelName' },
  { name: 'status', type: 'string', descKey: 'docs.apiFields.platformStatusStatus' },
  { name: 'next_retry_at', type: 'string?', descKey: 'docs.apiFields.platformStatusNextRetryAt' },
  { name: 'error_message', type: 'string?', descKey: 'docs.apiFields.platformStatusErrorMessage' },
]

// The code samples. Not copy: translating an identifier or a JSON key
// produces a sample that does not run against the gateway. Hoisted out of
// the JSX so the i18n gate, which cannot tell a <Pre> child from a
// paragraph, does not have to.
const WS_URL_TEMPLATE = `wss://allch.at/ws/overlay/{overlay_id}`

const JS_EXAMPLE = `const ws = new WebSocket(
  "wss://allch.at/ws/overlay/${TEST_OVERLAY_ID}"
)

ws.onmessage = (e) => {
  const msg = JSON.parse(e.data)
  switch (msg.type) {
    case "chat_message":
    case "message_update": {
      const d = msg.data
      if (d.event) {
        console.log("event", d.platform, d.event.type, d.event.value?.display_text)
      } else {
        console.log("chat", d.platform, d.user.display_name + ":", d.message.text)
      }
      break
    }
    case "ping":
      ws.send(JSON.stringify({ type: "pong" }))
      break
    case "connected":
    case "platform_status":
      break
  }
}`

const PYTHON_EXAMPLE = `# pip install websocket-client
import json, websocket

def on_message(ws, raw):
    msg = json.loads(raw)
    if msg["type"] in ("chat_message", "message_update"):
        d = msg["data"]
        if d.get("event"):
            print("event", d["platform"], d["event"]["type"])
        else:
            print("chat", d["platform"], d["user"]["display_name"], d["message"]["text"])
    elif msg["type"] == "ping":
        ws.send(json.dumps({"type": "pong"}))

ws = websocket.WebSocketApp(
    "wss://allch.at/ws/overlay/${TEST_OVERLAY_ID}",
    on_message=on_message,
)
ws.run_forever()`

const ENVELOPE_EXAMPLE = `{
  "type": "chat_message",
  "data": { ... },          // shape depends on "type" (omitted for ping/pong)
  "timestamp": "2026-06-18T14:30:00Z"
}`

const CHAT_MESSAGE_EXAMPLE = `{
  "type": "chat_message",
  "data": {
    "id": "abc123",
    "overlay_id": "${TEST_OVERLAY_ID}",
    "platform": "twitch",
    "channel_id": "12345",
    "channel_name": "examplestreamer",
    "user": {
      "id": "67890",
      "username": "viewer123",
      "display_name": "Viewer123",
      "color": "#1E90FF",
      "badges": [{ "name": "subscriber", "version": "12", "icon_url": "https://.../sub.png" }]
    },
    "message": {
      "text": "that was insane PogChamp",
      "emotes": [
        { "code": "PogChamp", "provider": "twitch", "url": "https://.../pogchamp.png", "positions": [[13, 20]] }
      ]
    },
    "timestamp": "2026-06-18T14:30:00Z",
    "metadata": {}
  },
  "timestamp": "2026-06-18T14:30:00Z"
}`

const SUBSCRIPTION_EVENT_EXAMPLE = `{
  "type": "chat_message",
  "data": {
    "id": "evt-sub-1",
    "overlay_id": "${TEST_OVERLAY_ID}",
    "platform": "twitch",
    "channel_id": "12345",
    "channel_name": "examplestreamer",
    "user": { "id": "999", "username": "newsub", "display_name": "NewSub", "badges": [] },
    "message": { "text": "just subscribed!", "emotes": [] },
    "timestamp": "2026-06-18T14:35:00Z",
    "metadata": {},
    "event": {
      "type": "subscription",
      "tier": "medium",
      "duration": 8,
      "is_update": false,
      "value": { "amount": 1, "currency": "months", "display_text": "Tier 1 sub" }
    }
  },
  "timestamp": "2026-06-18T14:35:00Z"
}`

const WS_URL_TEST_OVERLAY = `wss://allch.at/ws/overlay/${TEST_OVERLAY_ID}`

/**
 * Renders 'Platform: a, b, c', the platform name emphasised and each event type
 * in a <Code>.
 *
 * The label keeps the colon so it stays one translatable unit: a language that
 * spaces punctuation differently (French puts a space before a colon) needs to
 * move it, and a bare ':' at the render site is not something a translator can
 * see, let alone change.
 */
function EventTypeList({
  platformKey,
  className,
  types,
}: {
  platformKey: MessageKey
  className: string
  types: readonly string[]
}) {
  return (
    <li>
      {interpolateElements(t('docs.api.eventTypesLabel'), {
        platform: <strong className={className}>{t(platformKey)}</strong>,
      })}{' '}
      {types.map((type, index) => (
        <span key={type}>
          {index > 0 ? ', ' : ''}
          <Code>{type}</Code>
        </span>
      ))}
    </li>
  )
}

export default function DeveloperDocsPage() {
  return (
    <div className="min-h-screen bg-bg transition-colors duration-300">
      <JsonLd data={techArticleLd} />
      <JsonLd data={breadcrumbLd} />
      <AppNav />
      <div className="mx-auto max-w-4xl px-4 py-12">
        <div className="rounded-xl border border-border bg-surface p-8 transition-colors duration-300 md:p-12">
          <div className="mb-8 space-y-2">
            <p className="text-xs font-semibold tracking-[0.2em] text-twitch uppercase">
              {t('docs.api.eyebrow')}
            </p>
            <h1 className="text-3xl font-bold text-text">{t('docs.api.heading')}</h1>
            <p className="text-sm text-text-dim">{t('docs.api.intro')}</p>
            <p className="text-sm text-text-sub">
              {interpolateElements(t('docs.api.guidePrompt'), {
                guide: (
                  <Link href="/docs" className="text-twitch hover:underline">
                    {t('docs.api.guideLinkText')}
                  </Link>
                ),
              })}
            </p>
          </div>

          {/* Table of contents */}
          <nav className="mb-10 rounded-lg border border-border bg-surface-2 p-5">
            <p className="mb-2 text-xs font-semibold tracking-[0.15em] text-text-dim uppercase">
              {t('docs.api.tocHeading')}
            </p>
            <ul className="grid gap-1 sm:grid-cols-2">
              {toc.map(({ id, labelKey }) => (
                <li key={id}>
                  <a href={`#${id}`} className="text-sm text-twitch hover:underline">
                    {t(labelKey)}
                  </a>
                </li>
              ))}
            </ul>
          </nav>

          <div className="legal-prose space-y-10 leading-relaxed text-text-sub">
            {/* Connect a tool */}
            <section id="connect-a-tool">
              <h2>{t('docs.api.connectHeading')}</h2>
              <p>
                {interpolateElements(t('docs.api.connectBody'), {
                  twitch: <strong>{t('common.platforms.twitch')}</strong>,
                  youtube: <strong>{t('common.platforms.youtube')}</strong>,
                  kick: <strong>{t('common.platforms.kick')}</strong>,
                  tiktok: <strong>{t('common.platforms.tiktok')}</strong>,
                  discord: <strong>{t('common.platforms.discord')}</strong>,
                })}
              </p>
              <p>
                {interpolateElements(t('docs.api.connectAnonymous'), {
                  anonymous: <strong>{t('docs.api.connectAnonymousEmphasis')}</strong>,
                })}
              </p>
              <Pre>{WS_URL_TEMPLATE}</Pre>
              <p>
                {interpolateElements(t('docs.api.connectOverlayId'), {
                  field: <Code>{FIELD_OVERLAY_ID}</Code>,
                  testOverlay: <a href="#try-it-now">{t('docs.api.connectTestOverlayLinkText')}</a>,
                })}
              </p>

              <h3>{t('docs.api.queryParamsHeading')}</h3>
              <FieldTable rows={QUERY_PARAMS_FIELDS} />

              <h3>{t('docs.api.exampleJavascriptHeading')}</h3>
              <Pre lang="javascript">{JS_EXAMPLE}</Pre>

              <h3>{t('docs.api.examplePythonHeading')}</h3>
              <Pre lang="python">{PYTHON_EXAMPLE}</Pre>
            </section>

            {/* Message format */}
            <section id="message-format">
              <h2>{t('docs.api.formatHeading')}</h2>
              <p>{t('docs.api.formatBody')}</p>
              <Pre lang="json">{ENVELOPE_EXAMPLE}</Pre>
              <FieldTable rows={ENVELOPE_FIELDS} />
              <p>
                {interpolateElements(t('docs.api.formatTypeList'), {
                  field: <Code>{FIELD_TYPE}</Code>,
                })}
              </p>
              <FieldTable rows={MESSAGE_TYPES_FIELDS} />
            </section>

            {/* Chat messages */}
            <section id="chat-messages">
              <h2>{t('docs.api.chatHeading')}</h2>
              <p>
                {interpolateElements(t('docs.api.chatBody'), {
                  chat: <Code>{TYPE_CHAT_MESSAGE}</Code>,
                  update: <Code>{TYPE_MESSAGE_UPDATE}</Code>,
                  data: <Code>{FIELD_DATA}</Code>,
                })}
              </p>
              <FieldTable rows={CHAT_MESSAGE_FIELDS} />

              <h3>{t('docs.api.userHeading')}</h3>
              <FieldTable rows={USER_FIELDS} />

              <h3>{t('docs.api.messageHeading')}</h3>
              <p>
                {interpolateElements(t('docs.api.messageBody'), {
                  field: <Code>{FIELD_MESSAGE}</Code>,
                  shape: <Code>{SHAPE_MESSAGE}</Code>,
                  emote: <Code>{SHAPE_EMOTE}</Code>,
                })}
              </p>
              <FieldTable rows={EMOTE_FIELDS} />
              <p>
                {interpolateElements(t('docs.api.attachmentsBody'), {
                  field: <Code>{FIELD_ATTACHMENTS}</Code>,
                  attachment: <Code>{SHAPE_ATTACHMENT}</Code>,
                })}
              </p>
              <FieldTable rows={ATTACHMENT_FIELDS} />
              <p>
                {interpolateElements(t('docs.api.badgeBody'), {
                  badge: <Code>{SHAPE_BADGE}</Code>,
                  shape: <Code>{SHAPE_BADGE_FIELDS}</Code>,
                  example: <Code>{SHAPE_BADGE_EXAMPLE}</Code>,
                })}
              </p>

              <h3>{t('docs.api.exampleHeading')}</h3>
              <Pre lang="json">{CHAT_MESSAGE_EXAMPLE}</Pre>
            </section>

            {/* Events */}
            <section id="events">
              <h2>{t('docs.api.eventsHeading')}</h2>
              <p>
                {interpolateElements(t('docs.api.eventsBody'), {
                  chat: <Code>{TYPE_CHAT_MESSAGE}</Code>,
                  data: <Code>{FIELD_DATA}</Code>,
                  event: <Code>{FIELD_EVENT}</Code>,
                })}
              </p>
              <FieldTable rows={EVENT_FIELDS} />
              <h3>{t('docs.api.eventTypesHeading')}</h3>
              <ul>
                {EVENT_TYPES_BY_PLATFORM.map((platform) => (
                  <EventTypeList key={platform.platformKey} {...platform} />
                ))}
              </ul>
              <h3>{t('docs.api.exampleSubscriptionHeading')}</h3>
              <Pre lang="json">{SUBSCRIPTION_EVENT_EXAMPLE}</Pre>
            </section>

            {/* Status messages */}
            <section id="status-messages">
              <h2>{t('docs.api.statusHeading')}</h2>
              <p>
                {interpolateElements(t('docs.api.statusBody'), {
                  connected: <Code>{TYPE_CONNECTED}</Code>,
                  status: <Code>{TYPE_PLATFORM_STATUS}</Code>,
                })}
              </p>
              <FieldTable rows={PLATFORM_STATUS_FIELDS} />
            </section>

            {/* Reconnecting */}
            <section id="reconnecting">
              <h2>{t('docs.api.reconnectHeading')}</h2>
              <ul>
                <li>
                  {interpolateElements(t('docs.api.reconnectPing'), {
                    ping: <Code>{FRAME_PING}</Code>,
                    pong: <Code>{FRAME_PONG}</Code>,
                  })}
                </li>
                <li>
                  {interpolateElements(t('docs.api.reconnectBackfill'), {
                    since: <Code>{QUERY_SINCE}</Code>,
                  })}
                </li>
                <li>{t('docs.api.reconnectDedup')}</li>
              </ul>
            </section>

            {/* Try it now */}
            <section id="try-it-now">
              <h2>{t('docs.api.tryItHeading')}</h2>
              <p>
                {interpolateElements(t('docs.api.tryItBody'), {
                  one: <Code>{POLL_VOTES[0]}</Code>,
                  two: <Code>{POLL_VOTES[1]}</Code>,
                  three: <Code>{POLL_VOTES[2]}</Code>,
                  four: <Code>{POLL_VOTES[3]}</Code>,
                })}
              </p>
              <Pre>{WS_URL_TEST_OVERLAY}</Pre>
              <p>{t('docs.api.tryItOutro')}</p>
            </section>
          </div>

          {/* Footer */}
          <div className="mt-12 flex flex-col gap-3 border-t border-border pt-6 text-sm text-text-dim sm:flex-row sm:items-center sm:justify-between">
            <span>{t('docs.api.footerCopyright', { year: new Date().getFullYear() })}</span>
            <div className="flex flex-wrap items-center gap-4">
              <Link href="/docs" className="transition-colors hover:text-text">
                {t('docs.api.footerGuideLink')}
              </Link>
              <Link href="/legal/privacy" className="transition-colors hover:text-text">
                {t('docs.api.footerPrivacyLink')}
              </Link>
              <Link href="/legal/terms" className="transition-colors hover:text-text">
                {t('docs.api.footerTermsLink')}
              </Link>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
