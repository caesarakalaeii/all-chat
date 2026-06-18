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
import type { ReactNode } from 'react'
import hljs from 'highlight.js/lib/core'
import javascript from 'highlight.js/lib/languages/javascript'
import python from 'highlight.js/lib/languages/python'
import json from 'highlight.js/lib/languages/json'
import { AppNav } from '@/components/AppNav'

// Register only the languages used below. Highlighting runs server-side (this is
// a server component), so nothing ships to the client except the rendered HTML.
hljs.registerLanguage('javascript', javascript)
hljs.registerLanguage('python', python)
hljs.registerLanguage('json', json)

export const metadata = {
  title: 'Documentation | All-Chat',
  description:
    'How to use All-Chat and how to connect third-party tools to the unified chat WebSocket stream (Twitch, YouTube, Kick, TikTok, Discord).',
  alternates: { canonical: '/docs' },
}

const TEST_OVERLAY_ID = '00000000-0000-4000-8000-000000000a11'

function Code({ children }: { children: ReactNode }) {
  return (
    <code className="rounded bg-surface-2 px-1.5 py-0.5 font-mono text-[0.85em] text-text">
      {children}
    </code>
  )
}

function Pre({ children, lang }: { children: string; lang?: 'javascript' | 'python' | 'json' }) {
  const className =
    'my-4 overflow-x-auto rounded-lg border border-border bg-surface-2 p-4 text-sm leading-relaxed text-text-sub'
  if (!lang) {
    return (
      <pre className={className}>
        <code className="font-mono">{children}</code>
      </pre>
    )
  }
  const highlighted = hljs.highlight(children, { language: lang, ignoreIllegals: true }).value
  return (
    <pre className={className}>
      <code className="hljs font-mono" dangerouslySetInnerHTML={{ __html: highlighted }} />
    </pre>
  )
}

interface Field {
  name: string
  type: string
  desc: string
}

function FieldTable({ rows }: { rows: Field[] }) {
  return (
    <div className="my-4 overflow-x-auto rounded-lg border border-border">
      <table className="w-full border-collapse text-left text-sm">
        <thead>
          <tr className="bg-surface-2 text-text">
            <th className="px-4 py-2 font-semibold">Field</th>
            <th className="px-4 py-2 font-semibold">Type</th>
            <th className="px-4 py-2 font-semibold">Description</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((r) => (
            <tr key={r.name} className="border-t border-border align-top">
              <td className="px-4 py-2">
                <span className="font-mono text-text">{r.name}</span>
              </td>
              <td className="px-4 py-2 font-mono text-text-dim">{r.type}</td>
              <td className="px-4 py-2 text-text-sub">{r.desc}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

const toc = [
  ['what-is-all-chat', 'What is All-Chat'],
  ['getting-started', 'Getting started (streamers)'],
  ['connect-a-tool', 'Connect a third-party tool'],
  ['message-format', 'Message format'],
  ['chat-messages', 'Chat messages'],
  ['events', 'Events'],
  ['status-messages', 'Status & control messages'],
  ['reconnecting', 'Reconnecting & heartbeats'],
  ['try-it-now', 'Try it now (no setup)'],
]

export default function DocsPage() {
  return (
    <div className="min-h-screen bg-bg transition-colors duration-300">
      <AppNav />
      <div className="mx-auto max-w-4xl px-4 py-12">
        <div className="rounded-xl border border-border bg-surface p-8 transition-colors duration-300 md:p-12">
          <div className="mb-8 space-y-2">
            <p className="text-xs font-semibold tracking-[0.2em] text-twitch uppercase">
              All-Chat Docs
            </p>
            <h1 className="text-3xl font-bold text-text">Documentation</h1>
            <p className="text-sm text-text-dim">
              Using All-Chat and connecting third-party tools to the chat stream.
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
            {/* What is All-Chat */}
            <section id="what-is-all-chat">
              <h2>What is All-Chat</h2>
              <p>
                All-Chat aggregates live chat from <strong>Twitch</strong>, <strong>YouTube</strong>
                , <strong>Kick</strong>, <strong>TikTok</strong> and <strong>Discord</strong> into a
                single, normalized stream. You create an <em>overlay</em>, attach one or more chat
                sources to it, and All-Chat delivers every message — and platform events like subs,
                bits, raids, super chats and gifts — over one WebSocket in one unified format. 7TV,
                BTTV and FFZ emotes are resolved and attached for you.
              </p>
              <p>
                That single stream powers the browser overlay you add to OBS, and it is the same
                stream any third-party tool can read to build bots, moderation tools, vote counters,
                analytics, alerts, or anything else.
              </p>
            </section>

            {/* Getting started */}
            <section id="getting-started">
              <h2>Getting started (streamers)</h2>
              <ol className="list-decimal space-y-2 pl-6 text-text-sub">
                <li>
                  Sign in at <Link href="/">allch.at</Link> with your Twitch account.
                </li>
                <li>
                  In the dashboard, create an <strong>overlay</strong> and connect the platforms you
                  stream on (Twitch, YouTube, Kick, TikTok, Discord). Each connected platform becomes
                  a <em>chat source</em> on that overlay.
                </li>
                <li>
                  Copy the overlay&apos;s browser-source URL and add it to OBS as a{' '}
                  <strong>Browser Source</strong>. Chat appears as soon as the overlay connects.
                </li>
                <li>
                  Sources are demand-driven: All-Chat starts listening when the overlay is open and
                  winds down when nothing is connected, so you do not pay for idle listeners.
                </li>
              </ol>
            </section>

            {/* Connect a third-party tool */}
            <section id="connect-a-tool">
              <h2>Connect a third-party tool</h2>
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
                  { name: 'message', type: 'object', desc: '{ text, emotes[] } (see below).' },
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

              <h3>message and emotes</h3>
              <p>
                <Code>message</Code> is <Code>{`{ "text": string, "emotes": Emote[] }`}</Code>. Each{' '}
                <Code>Emote</Code>:
              </p>
              <FieldTable
                rows={[
                  { name: 'code', type: 'string', desc: 'Emote text token, e.g. "Kappa".' },
                  { name: 'provider', type: 'string', desc: '"twitch" | "7tv" | "bttv" | "ffz" | platform.' },
                  { name: 'url', type: 'string', desc: 'CDN image URL.' },
                  { name: 'positions', type: 'int[][]', desc: 'Array of [start, end] index pairs into text where the emote occurs.' },
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
              <Link href="/" className="transition-colors hover:text-text">
                Home
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
