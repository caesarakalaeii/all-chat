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
import { Code, Pre, FieldTable } from '@/components/docs/prose'

export const metadata = {
  title: 'Developer API',
  description:
    'Connect third-party tools to the All-Chat unified chat WebSocket stream: message format, platform events, status messages and reconnection (Twitch, YouTube, Kick, TikTok, Discord).',
  alternates: { canonical: '/docs/api' },
}

const TEST_OVERLAY_ID = '00000000-0000-4000-8000-000000000a11'

const toc = [
  ['connect-a-tool', 'Connect a tool'],
  ['message-format', 'Message format'],
  ['chat-messages', 'Chat messages'],
  ['events', 'Events'],
  ['status-messages', 'Status & control messages'],
  ['reconnecting', 'Reconnecting & heartbeats'],
  ['try-it-now', 'Try it now (no setup)'],
]

export default function DeveloperDocsPage() {
  return (
    <div className="min-h-screen bg-bg transition-colors duration-300">
      <AppNav />
      <div className="mx-auto max-w-4xl px-4 py-12">
        <div className="rounded-xl border border-border bg-surface p-8 transition-colors duration-300 md:p-12">
          <div className="mb-8 space-y-2">
            <p className="text-xs font-semibold tracking-[0.2em] text-twitch uppercase">
              Developer API
            </p>
            <h1 className="text-3xl font-bold text-text">Developer API reference</h1>
            <p className="text-sm text-text-dim">
              Read the unified chat stream over one WebSocket to build bots, moderation tools, vote
              counters, analytics, alerts — anything.
            </p>
            <p className="text-sm text-text-sub">
              Just setting up your overlay?{' '}
              <Link href="/docs" className="text-twitch hover:underline">
                See the streamer guide →
              </Link>
            </p>
          </div>

          {/* Table of contents */}
          <nav className="mb-10 rounded-lg border border-border bg-surface-2 p-5">
            <p className="mb-2 text-xs font-semibold tracking-[0.15em] text-text-dim uppercase">
              On this page
            </p>
            <ul className="grid gap-1 sm:grid-cols-2">
              {toc.map(([id, label]) => (
                <li key={id}>
                  <a href={`#${id}`} className="text-sm text-twitch hover:underline">
                    {label}
                  </a>
                </li>
              ))}
            </ul>
          </nav>

          <div className="legal-prose space-y-10 leading-relaxed text-text-sub">
            {/* Connect a tool */}
            <section id="connect-a-tool">
              <h2>Connect a tool</h2>
              <p>
                All-Chat aggregates live chat from <strong>Twitch</strong>, <strong>YouTube</strong>
                , <strong>Kick</strong>, <strong>TikTok</strong> and <strong>Discord</strong> into a
                single normalized stream — every message, plus platform events like subs, bits,
                raids, super chats and gifts, in one format, with 7TV/BTTV/FFZ emotes resolved for
                you. The same stream that powers the browser overlay is the one your tool reads.
              </p>
              <p>
                Open a WebSocket to the overlay endpoint. Reading the stream is{' '}
                <strong>anonymous</strong> — no token or account is required (this is the same
                &quot;OBS mode&quot; the browser overlay uses):
              </p>
              <Pre>{`wss://allch.at/ws/overlay/{overlay_id}`}</Pre>
              <p>
                Your <Code>overlay_id</Code> is the UUID in your overlay&apos;s browser-source URL.
                Want to try without an account first? Use the{' '}
                <a href="#try-it-now">public test overlay</a>.
              </p>

              <h3>Query parameters</h3>
              <FieldTable
                rows={[
                  {
                    name: '?token=',
                    type: 'JWT (optional)',
                    desc: 'Owner token. Only needed for owner-scoped access; omit it for read-only consumption.',
                  },
                  {
                    name: '?since=',
                    type: 'int (optional)',
                    desc: 'Milliseconds since the Unix epoch. On connect, the server replays buffered messages newer than this timestamp so a reconnecting client can backfill the gap. Omit to start live.',
                  },
                ]}
              />

              <h3>Minimal example (JavaScript)</h3>
              <Pre lang="javascript">{`const ws = new WebSocket(
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
}`}</Pre>

              <h3>Minimal example (Python)</h3>
              <Pre lang="python">{`# pip install websocket-client
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
ws.run_forever()`}</Pre>
            </section>

            {/* Message format */}
            <section id="message-format">
              <h2>Message format</h2>
              <p>Every frame is JSON with the same envelope:</p>
              <Pre lang="json">{`{
  "type": "chat_message",
  "data": { ... },          // shape depends on "type" (omitted for ping/pong)
  "timestamp": "2026-06-18T14:30:00Z"
}`}</Pre>
              <FieldTable
                rows={[
                  { name: 'type', type: 'string', desc: 'Message type (see below).' },
                  { name: 'data', type: 'object', desc: 'Payload; shape depends on type. Omitted for ping/pong.' },
                  { name: 'timestamp', type: 'string', desc: 'RFC 3339 timestamp of when the gateway sent the frame.' },
                ]}
              />
              <p>
                <Code>type</Code> is one of:
              </p>
              <FieldTable
                rows={[
                  { name: 'chat_message', type: '', desc: 'A chat message or a platform event. data is the unified message object.' },
                  { name: 'message_update', type: '', desc: 'An update to a previously sent message (e.g. TikTok like aggregates). Same data shape as chat_message.' },
                  { name: 'connected', type: '', desc: 'Sent once on connect: { overlay_id, message }.' },
                  { name: 'platform_status', type: '', desc: 'Connection status of a source platform.' },
                  { name: 'ping', type: '', desc: 'Heartbeat from the server. Reply with { "type": "pong" }.' },
                  { name: 'error', type: '', desc: 'Error notice: { code, message }.' },
                ]}
              />
            </section>

            {/* Chat messages */}
            <section id="chat-messages">
              <h2>Chat messages</h2>
              <p>
                For <Code>chat_message</Code> and <Code>message_update</Code>, <Code>data</Code> is
                the unified message object:
              </p>
              <FieldTable
                rows={[
                  { name: 'id', type: 'string', desc: 'Unique message ID.' },
                  { name: 'overlay_id', type: 'string', desc: 'Overlay this message was delivered to.' },
                  { name: 'platform', type: 'string', desc: '"twitch" | "youtube" | "kick" | "tiktok" | "discord".' },
                  { name: 'channel_id', type: 'string', desc: 'Platform channel identifier.' },
                  { name: 'channel_name', type: 'string', desc: 'Human-readable channel name.' },
                  { name: 'user', type: 'object', desc: 'Author info (see below).' },
                  { name: 'message', type: 'object', desc: '{ text, emotes[], attachments[]? } (see below).' },
                  { name: 'timestamp', type: 'string', desc: 'RFC 3339 message time (UTC).' },
                  { name: 'metadata', type: 'object', desc: 'Free-form, platform-specific extras.' },
                  { name: 'event', type: 'object?', desc: 'Present only when the message is a platform event (see Events). Absent for normal chat.' },
                ]}
              />

              <h3>user</h3>
              <FieldTable
                rows={[
                  { name: 'id', type: 'string', desc: 'Platform user ID.' },
                  { name: 'username', type: 'string', desc: 'Login/handle.' },
                  { name: 'display_name', type: 'string', desc: 'Display name.' },
                  { name: 'color', type: 'string?', desc: 'Name color, hex (e.g. "#FF0000").' },
                  { name: 'badges', type: 'Badge[]', desc: 'Author badges (see below).' },
                  { name: 'avatar_url', type: 'string?', desc: 'Profile image URL when available.' },
                  { name: 'pronouns', type: 'string?', desc: 'e.g. "she/her" when known.' },
                  { name: 'name_gradient', type: 'string?', desc: 'Optional gradient descriptor (JSON string).' },
                  { name: 'source_badges / source_user_id', type: '?', desc: 'Origin-channel identity for shared-chat messages.' },
                  { name: 'avatar_frame_url / avatar_flair_url', type: 'string?', desc: 'Cosmetic frame/flair when set.' },
                ]}
              />

              <h3>message, emotes, and attachments</h3>
              <p>
                <Code>message</Code> is{' '}
                <Code>{`{ "text": string, "emotes": Emote[], "attachments"?: Attachment[] }`}</Code>. Each{' '}
                <Code>Emote</Code>:
              </p>
              <FieldTable
                rows={[
                  { name: 'code', type: 'string', desc: 'Emote text token, e.g. "Kappa".' },
                  { name: 'provider', type: 'string', desc: '"twitch" | "7tv" | "bttv" | "ffz" | "discord" | platform.' },
                  { name: 'url', type: 'string', desc: 'CDN image URL.' },
                  { name: 'positions', type: 'int[][]', desc: 'Array of [start, end] index pairs into text where the emote occurs.' },
                ]}
              />
              <p>
                <Code>attachments</Code> is present only when a message carries media (Discord image/GIF/video
                uploads and Tenor/Giphy link previews today). Each <Code>Attachment</Code>:
              </p>
              <FieldTable
                rows={[
                  { name: 'type', type: 'string', desc: '"image" or "video". GIFs are images that animate.' },
                  { name: 'url', type: 'string', desc: 'Media URL.' },
                  { name: 'content_type', type: 'string?', desc: 'MIME type, e.g. "image/gif", "video/mp4".' },
                  { name: 'width / height', type: 'int?', desc: 'Intrinsic pixel dimensions when known.' },
                  { name: 'thumb_url', type: 'string?', desc: 'Poster frame for videos when available.' },
                  { name: 'spoiler', type: 'bool?', desc: 'True when the sender marked the media a spoiler.' },
                  { name: 'filename', type: 'string?', desc: 'Original filename (used for alt text).' },
                ]}
              />
              <p>
                Each <Code>Badge</Code> is <Code>{`{ "name", "version", "icon_url" }`}</Code> — e.g.{' '}
                <Code>{`{ "name": "subscriber", "version": "12", "icon_url": "..." }`}</Code>.
              </p>

              <h3>Example</h3>
              <Pre lang="json">{`{
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
}`}</Pre>
            </section>

            {/* Events */}
            <section id="events">
              <h2>Events</h2>
              <p>
                Platform events (subs, bits, raids, donations, …) arrive as <Code>chat_message</Code>{' '}
                frames whose <Code>data</Code> includes an <Code>event</Code> object. Normal chat has
                no <Code>event</Code> field.
              </p>
              <FieldTable
                rows={[
                  { name: 'type', type: 'string', desc: 'Event type (see list below).' },
                  { name: 'tier', type: 'string', desc: 'Relative prominence: "high" | "medium" | "low".' },
                  { name: 'value', type: 'object?', desc: '{ amount: number, currency: string, display_text: string } — e.g. amount 250, currency "bits", display_text "250 bits".' },
                  { name: 'duration', type: 'int', desc: 'Suggested on-screen display time, seconds.' },
                  { name: 'is_update', type: 'bool', desc: 'true when this updates a prior event (e.g. TikTok like aggregates, delivered as message_update).' },
                  { name: 'aggregation_id', type: 'string?', desc: 'Groups successive updates of the same aggregate.' },
                  { name: 'metadata', type: 'object?', desc: 'Event-specific raw fields.' },
                ]}
              />
              <h3>Event types by platform</h3>
              <ul>
                <li>
                  <strong className="text-twitch">Twitch</strong>: <Code>subscription</Code>,{' '}
                  <Code>resubscription</Code>, <Code>gift_subscription</Code>,{' '}
                  <Code>mystery_gift</Code>, <Code>bits</Code>, <Code>raid</Code>,{' '}
                  <Code>channel_points</Code>, <Code>message_deletion</Code>
                </li>
                <li>
                  <strong className="text-youtube">YouTube</strong>: <Code>super_chat</Code>,{' '}
                  <Code>super_sticker</Code>, <Code>new_sponsor</Code>,{' '}
                  <Code>member_milestone</Code>, <Code>membership_gift</Code>,{' '}
                  <Code>gift_received</Code>
                </li>
                <li>
                  <strong className="text-tiktok">TikTok</strong>: <Code>gift</Code>,{' '}
                  <Code>like_aggregate</Code>, <Code>follow</Code>, <Code>share</Code>
                </li>
              </ul>
              <h3>Example (Twitch subscription)</h3>
              <Pre lang="json">{`{
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
}`}</Pre>
            </section>

            {/* Status messages */}
            <section id="status-messages">
              <h2>Status &amp; control messages</h2>
              <p>
                On connect the server sends a <Code>connected</Code> frame, then a{' '}
                <Code>platform_status</Code> frame per configured source so your UI can show
                indicators immediately. <Code>platform_status</Code> data:
              </p>
              <FieldTable
                rows={[
                  { name: 'platform', type: 'string', desc: 'Source platform.' },
                  { name: 'channel_id', type: 'string', desc: 'Source channel.' },
                  { name: 'channel_name', type: 'string?', desc: 'Human-readable channel name.' },
                  { name: 'status', type: 'string', desc: '"connected" | "reconnecting" | "offline" | "quota_exceeded".' },
                  { name: 'next_retry_at', type: 'string?', desc: 'RFC 3339 time of the next reconnect attempt, when applicable.' },
                  { name: 'error_message', type: 'string?', desc: 'Human-readable detail when degraded.' },
                ]}
              />
            </section>

            {/* Reconnecting */}
            <section id="reconnecting">
              <h2>Reconnecting &amp; heartbeats</h2>
              <ul>
                <li>
                  The server periodically sends <Code>{`{ "type": "ping" }`}</Code>. Reply with{' '}
                  <Code>{`{ "type": "pong" }`}</Code>. (Most WebSocket libraries also answer
                  low-level ping frames automatically.)
                </li>
                <li>
                  If the socket drops, reconnect with backoff. To backfill messages missed during a
                  brief disconnect, reconnect with <Code>?since=&lt;ms-epoch&gt;</Code> set to just
                  before you lost the connection — the server replays buffered messages newer than
                  that timestamp.
                </li>
                <li>
                  Treat IDs as the dedup key when combining the replay buffer with live messages.
                </li>
              </ul>
            </section>

            {/* Try it now */}
            <section id="try-it-now">
              <h2>Try it now (no setup)</h2>
              <p>
                There is a public test overlay that streams realistic fake traffic — random chat,
                poll votes (the literal messages <Code>1</Code>, <Code>2</Code>, <Code>3</Code>,{' '}
                <Code>4</Code>) and platform events — so you can build and validate an integration
                without an account or a real channel. Just connect; traffic flows while a client is
                connected and stops when the last one disconnects:
              </p>
              <Pre>{`wss://allch.at/ws/overlay/${TEST_OVERLAY_ID}`}</Pre>
              <p>
                Drop that URL into either example above and you should immediately see messages and
                events stream in.
              </p>
            </section>
          </div>

          {/* Footer */}
          <div className="mt-12 flex flex-col gap-3 border-t border-border pt-6 text-sm text-text-dim sm:flex-row sm:items-center sm:justify-between">
            <span>&copy; {new Date().getFullYear()} All-Chat</span>
            <div className="flex flex-wrap items-center gap-4">
              <Link href="/docs" className="transition-colors hover:text-text">
                Streamer guide
              </Link>
              <Link href="/legal/privacy" className="transition-colors hover:text-text">
                Privacy Policy
              </Link>
              <Link href="/legal/terms" className="transition-colors hover:text-text">
                Terms of Service
              </Link>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
