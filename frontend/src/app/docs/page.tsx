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
import { AppNav } from '@/components/AppNav'
import { Code, Pre } from '@/components/docs/prose'
import { JsonLd } from '@/components/JsonLd'
import { DISCORD_INVITE_URL } from '@/lib/constants'

export const metadata = {
  title: 'Documentation',
  description:
    'Get your All-Chat overlay live in OBS, pick from 16 built-in themes, and make it your own — no CSS required to start, full CSS control when you want it.',
  alternates: { canonical: '/docs' },
}

// Structured data: the "Get your overlay live" section is a genuine step-by-step
// procedure, so it maps to schema.org HowTo (rich-result eligible). Breadcrumbs
// give Google the Home > Documentation trail. Emitted from this server page, so
// both land in the initial HTML for crawlers.
const howToLd = {
  '@context': 'https://schema.org',
  '@type': 'HowTo',
  name: 'Get your All-Chat overlay live in OBS',
  description:
    'Set up a multi-platform chat overlay in OBS with All-Chat, combining Twitch, YouTube, Kick, TikTok and Discord chat into one browser source.',
  step: [
    {
      '@type': 'HowToStep',
      position: 1,
      name: 'Sign in',
      text: 'Sign in at allch.at with your Twitch, YouTube, or Kick account.',
    },
    {
      '@type': 'HowToStep',
      position: 2,
      name: 'Create an overlay and connect your platforms',
      text: 'In the dashboard, create an overlay and connect the platforms you stream on. Each connected platform becomes a chat source on that overlay.',
    },
    {
      '@type': 'HowToStep',
      position: 3,
      name: 'Add the overlay to OBS',
      text: 'Copy the overlay browser-source URL and add it to OBS as a Browser Source. Chat appears the moment the overlay connects.',
    },
  ],
}

const breadcrumbLd = {
  '@context': 'https://schema.org',
  '@type': 'BreadcrumbList',
  itemListElement: [
    { '@type': 'ListItem', position: 1, name: 'Home', item: 'https://allch.at' },
    { '@type': 'ListItem', position: 2, name: 'Documentation', item: 'https://allch.at/docs' },
  ],
}

/** A "variable / default / effect" row table for the CSS custom properties. */
interface CssVar {
  name: string
  default: string
  effect: string
}
function CssVarTable({ rows }: { rows: CssVar[] }) {
  return (
    <div className="my-4 overflow-x-auto rounded-lg border border-border">
      <table className="w-full border-collapse text-left text-sm">
        <thead>
          <tr className="bg-surface-2 text-text">
            <th className="px-4 py-2 font-semibold">Variable</th>
            <th className="px-4 py-2 font-semibold">Default</th>
            <th className="px-4 py-2 font-semibold">What it changes</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((r) => (
            <tr key={r.name} className="border-t border-border align-top">
              <td className="px-4 py-2">
                <span className="font-mono text-text">{r.name}</span>
              </td>
              <td className="px-4 py-2 font-mono text-text-dim">{r.default}</td>
              <td className="px-4 py-2 text-text-sub">{r.effect}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

/** A "selector / kind / targets" hook table. */
interface Hook {
  selector: string
  kind: string
  targets: string
}
function HookTable({ rows }: { rows: Hook[] }) {
  return (
    <div className="my-4 overflow-x-auto rounded-lg border border-border">
      <table className="w-full border-collapse text-left text-sm">
        <thead>
          <tr className="bg-surface-2 text-text">
            <th className="px-4 py-2 font-semibold">Selector</th>
            <th className="px-4 py-2 font-semibold">Kind</th>
            <th className="px-4 py-2 font-semibold">Targets</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((r) => (
            <tr key={r.selector} className="border-t border-border align-top">
              <td className="px-4 py-2">
                <span className="font-mono text-text">{r.selector}</span>
              </td>
              <td className="px-4 py-2 font-mono text-text-dim">{r.kind}</td>
              <td className="px-4 py-2 text-text-sub">{r.targets}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

function DevCallout({ children }: { children: ReactNode }) {
  return (
    <div className="rounded-lg border border-border bg-surface-2 p-5 text-sm text-text-sub">
      {children}
    </div>
  )
}

const toc = [
  ['what-is-all-chat', 'What is All-Chat'],
  ['getting-started', 'Get your overlay live'],
  ['24-7-irl', '24/7 & IRL streams'],
  ['monitor', 'The chat monitor'],
  ['moderation', 'Moderate your chat'],
  ['engagement', 'Polls, predictions & points'],
  ['events-credits', 'Events & credit roll'],
  ['sharing', 'Share an overlay'],
  ['themes', 'Pick a theme'],
  ['customize', 'Make it your own'],
  ['custom-css', 'Go further with CSS'],
  ['fonts', 'Custom fonts'],
  ['premium', 'Premium'],
]

export default function DocsPage() {
  return (
    <div className="min-h-screen bg-bg transition-colors duration-300">
      <JsonLd data={howToLd} />
      <JsonLd data={breadcrumbLd} />
      <AppNav />
      <main id="main-content" tabIndex={-1} className="mx-auto max-w-4xl px-4 py-12">
        <div className="rounded-xl border border-border bg-surface p-8 transition-colors duration-300 md:p-12">
          <div className="mb-8 space-y-2">
            <p className="text-xs font-semibold tracking-[0.2em] text-twitch uppercase">
              All-Chat Docs
            </p>
            <h1 className="text-3xl font-bold text-text">Streamer guide</h1>
            <p className="text-sm text-text-dim">
              Set it up in OBS, run polls, predictions and moderation from the chat monitor, and
              make it your own.
            </p>
            <p className="text-sm text-text-sub">
              Building a bot, alert box, or analytics tool?{' '}
              <Link href="/docs/api" className="text-twitch underline underline-offset-2">
                See the Developer API →
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
            {/* What is All-Chat */}
            <section id="what-is-all-chat">
              <h2>What is All-Chat</h2>
              <p>
                All-Chat pulls your live chat from <strong>Twitch</strong>, <strong>YouTube</strong>
                , <strong>Kick</strong>, <strong>TikTok</strong> and <strong>Discord</strong> into a
                single overlay you drop into OBS. Every message lands in one feed — plus events like
                subs, bits, raids, super chats and gifts — and 7TV, BTTV and FFZ emotes show up
                automatically. No bots to invite, no chat widget to wire up.
              </p>
            </section>

            {/* Getting started */}
            <section id="getting-started">
              <h2>Get your overlay live</h2>
              <ol className="list-decimal space-y-2 pl-6 text-text-sub">
                <li>
                  Sign in at <Link href="/">allch.at</Link> with your Twitch, YouTube, or Kick
                  account.
                </li>
                <li>
                  In the dashboard, create an <strong>overlay</strong> and connect the platforms you
                  stream on. Each connected platform becomes a <em>chat source</em> on that overlay
                  — mix and match as many as you like.
                </li>
                <li>
                  Copy the overlay&apos;s browser-source URL and add it to OBS as a{' '}
                  <strong>Browser Source</strong>. Chat appears the moment the overlay connects.
                </li>
                <li>
                  Sources are demand-driven: All-Chat starts listening when the overlay is open and
                  winds down when nothing is connected, so you never pay for idle listeners.
                </li>
              </ol>
            </section>

            {/* 24/7 & IRL streams */}
            <section id="24-7-irl">
              <h2>24/7 &amp; IRL streams</h2>
              <p>
                Running an OBS instance around the clock — a common setup for IRL streamers who want
                disconnect protection — needs one extra step so YouTube chat behaves. Add{' '}
                <Code>?passive=true</Code> to your overlay&apos;s browser-source URL:
              </p>
              <Pre>{`https://allch.at/overlay/<overlay-id>?passive=true`}</Pre>
              <p>
                A <strong>passive</strong> overlay renders chat exactly like a normal one, but it
                does not ask All-Chat to start capturing on its own. That matters because YouTube
                discovery gives up after an hour of not finding a live stream (so it never hammers
                YouTube for an offline channel). A normal 24/7 overlay would trip that timeout while
                you are offline and sit parked by the time you go live — a passive overlay never
                starts that clock, so nothing gets stuck.
              </p>
              <h3>When you go live</h3>
              <ol className="list-decimal space-y-2 pl-6 text-text-sub">
                <li>Leave your 24/7 OBS browser source on the passive URL.</li>
                <li>
                  Open your overlay&apos;s <strong>chat monitor</strong> (the monitor / view page).
                </li>
                <li>
                  If chat is not already flowing, hit <strong>Rediscover</strong> on the monitor —
                  capture starts within about a minute.
                </li>
                <li>
                  Keep the chat monitor open while you stream; that keeps capture running for the
                  session, and it winds down a few minutes after you close it.
                </li>
              </ol>
              <p>
                A plain browser-source refresh does <em>not</em> restart a parked YouTube channel —
                use the monitor&apos;s <strong>Rediscover</strong> button. While a channel is
                parked, its platform dot shows an <strong>indigo &ldquo;paused&rdquo;</strong> state
                (waiting for you to trigger it), not a red error.
              </p>
            </section>

            {/* Chat monitor */}
            <section id="monitor">
              <h2>The chat monitor</h2>
              <p>
                Open <strong>Monitor View</strong> from the overlay editor for a live control room,
                separate from the OBS overlay. It shows every message in one panel with an activity
                feed beside it, and it&apos;s where you send messages and moderate.
              </p>
              <ul>
                <li>
                  <strong>Send messages</strong> as yourself to Twitch, YouTube or Kick straight
                  from the monitor (TikTok and Discord have no send API).
                </li>
                <li>
                  <strong>Re-discover YouTube</strong> forces a fresh look for your live stream —
                  use it if YouTube chat stops after a crash or restart, or to start capture for a{' '}
                  <a href="#24-7-irl">passive 24/7 overlay</a> when you go live.
                </li>
                <li>
                  <strong>Display</strong> settings toggle what you see (avatars, badges, pronouns,
                  timestamps, platform icons, moderation controls). These are your personal monitor
                  preferences and don&apos;t change the OBS overlay.
                </li>
              </ul>
            </section>

            {/* Moderation */}
            <section id="moderation">
              <h2>Moderate your chat</h2>
              <p>
                With <strong>Moderation controls</strong> on in the monitor, hover a message to{' '}
                <strong>Delete</strong> it, or open a chatter to <strong>Timeout</strong>,{' '}
                <strong>Ban</strong> or <strong>Unban</strong> them — applied on the source
                platform: Twitch and Discord do delete, timeout, ban and unban; Kick does timeout,
                ban and unban (no single-message delete); YouTube is ban-only; TikTok has no
                moderation API.
              </p>
              <p>
                The first time, use <strong>Enable moderation &amp; chat sending</strong> to grant
                the extra permissions (for Discord you re-invite the bot). Moderating from your
                overlay is a <a href="#premium">Premium</a> feature.
              </p>
            </section>

            {/* Engagement */}
            <section id="engagement">
              <h2>Polls, predictions &amp; points</h2>
              <p>
                All-Chat has its own <strong>polls</strong>, <strong>predictions</strong> and{' '}
                <strong>viewer points</strong> that work across every connected platform, not just
                Twitch.
              </p>
              <ul>
                <li>
                  Turn them on in the editor&apos;s <strong>Engagement</strong> section:{' '}
                  <strong>Enable viewer points</strong>, set your <strong>Points name</strong>, and
                  choose how points are earned (per bit, per sub, per gifted sub, and so on).
                </li>
                <li>
                  Run rounds live from the monitor&apos;s <strong>Engagement</strong> panel:{' '}
                  <strong>Start poll</strong> / <strong>Close poll</strong>, and{' '}
                  <strong>Start prediction</strong> / <strong>Lock wagers</strong> / pay out or{' '}
                  <strong>Cancel &amp; refund</strong>.
                </li>
                <li>
                  Viewers join from chat (e.g. <Code>!vote 2</Code>, <Code>!predict 1 500</Code>) or
                  on the <strong>Viewer participation page</strong>.
                </li>
                <li>
                  Add the <strong>OBS poll widget</strong> and{' '}
                  <strong>OBS prediction widget</strong> as their own browser sources so results
                  show on stream — copy their URLs from the Engagement section.
                </li>
              </ul>
            </section>

            {/* Events & credit roll */}
            <section id="events-credits">
              <h2>Events &amp; credit roll</h2>
              <p>
                Your overlay shows <strong>events</strong> too — subs, resubs, gift subs,
                bits/cheers, raids, Super Chats, memberships and follows. Use{' '}
                <strong>Event Settings</strong> in the editor to choose exactly which events appear,
                per platform.
              </p>
              <p>
                For the end of a stream, open <strong>Credits</strong> to set up a{' '}
                <strong>Credit Roll</strong> — a scrolling thank-you of your top subscribers,
                gifters, cheerers, raiders and new followers. It&apos;s its own browser source (
                <strong>Copy Credits OBS URL</strong>).
              </p>
            </section>

            {/* Share an overlay */}
            <section id="sharing">
              <h2>Share an overlay</h2>
              <p>
                <strong>Share Overlay</strong> lets another streamer pull your overlay&apos;s chat
                into theirs — handy for collabs and raids. Send a request to their Twitch username;
                once they accept, your overlay appears in their editor under{' '}
                <strong>Shared Overlays</strong> to add as a source, and either of you can revoke it
                later. Sharing is a <a href="#premium">Premium</a> feature.
              </p>
            </section>

            {/* Themes */}
            <section id="themes">
              <h2>Pick a theme</h2>
              <p>
                All-Chat ships with <strong>16 built-in themes</strong> — from Modern Dark and
                Minimal to Trading Card, Comic Speech, Sticky Notes, Vaporwave, Cyberpunk and more.
                Applying one takes a click and needs <strong>no CSS at all</strong>.
              </p>
              <ol className="list-decimal space-y-2 pl-6 text-text-sub">
                <li>Open your overlay in the dashboard to edit it.</li>
                <li>
                  In the <strong>Theme</strong> section, browse the themes and apply one. The
                  preview updates instantly.
                </li>
                <li>Save. Your OBS browser source picks up the new look on its next refresh.</li>
              </ol>
              <p>
                You can also preview every theme live on the <Link href="/">home page</Link> before
                you sign in.
              </p>
            </section>

            {/* Make it your own (no-code) */}
            <section id="customize">
              <h2>Make it your own</h2>
              <p>
                A theme is a starting point — most of the time you can get exactly the look you want
                <strong> without writing any code</strong>. In the overlay editor&apos;s{' '}
                <strong>Appearance</strong> panel you can adjust, with sliders, toggles and color
                pickers:
              </p>
              <ul>
                <li>Font family, text size, weight, line height and letter spacing</li>
                <li>Spacing, padding and message-bubble corners</li>
                <li>Avatar and badge size</li>
                <li>What to show or hide — avatars, badges, timestamps, usernames</li>
                <li>Colors, including a name color per platform</li>
                <li>How events like subs, raids and super chats appear</li>
              </ul>
              <p>
                Start from the theme closest to what you want, then tweak these until it feels like
                yours. If you need something the panel doesn&apos;t cover, drop down to CSS below.
              </p>
            </section>

            {/* Custom CSS */}
            <section id="custom-css">
              <h2>Go further with CSS</h2>
              <p>
                For full control, enable <strong>Custom CSS</strong> in the overlay editor&apos;s{' '}
                <strong>Expert</strong> section and write your own styles. A few things worth
                knowing:
              </p>
              <ul>
                <li>Your CSS only affects your overlay — nothing else.</li>
                <li>
                  It loads <em>after</em> the theme, so you can start from a built-in theme and
                  override just the parts you want.
                </li>
                <li>
                  There&apos;s a live preview right next to the editor, so you see changes as you
                  type.
                </li>
              </ul>

              <h3>Quick wins: style variables</h3>
              <p>
                The easiest lever needs no knowledge of class names. Set any of these variables on{' '}
                <Code>:root</Code> and the overlay picks them up:
              </p>
              <CssVarTable
                rows={[
                  { name: '--chat-font-size', default: '1rem', effect: 'Message text size.' },
                  { name: '--chat-font-family', default: 'inherit', effect: 'Message text font.' },
                  {
                    name: '--chat-message-color',
                    default: '#ffffff',
                    effect: 'Message text color.',
                  },
                  {
                    name: '--chat-message-gap',
                    default: '0.75rem',
                    effect: 'Vertical space between messages.',
                  },
                  {
                    name: '--chat-bubble-border-radius',
                    default: '0.5rem',
                    effect: 'Roundness of the message bubble.',
                  },
                  {
                    name: '--chat-bubble-padding',
                    default: '0.75rem',
                    effect: 'Padding inside each message.',
                  },
                  {
                    name: '--chat-avatar-size',
                    default: '2.5rem',
                    effect: 'Avatar width and height.',
                  },
                  {
                    name: '--chat-username-font-size',
                    default: '0.875rem',
                    effect: 'Username size.',
                  },
                  { name: '--chat-emote-scale', default: '1', effect: 'Emote size multiplier.' },
                  {
                    name: '--chat-show-avatars',
                    default: 'block',
                    effect: 'Set to none to hide avatars.',
                  },
                  {
                    name: '--chat-show-badges',
                    default: 'flex',
                    effect: 'Set to none to hide badges.',
                  },
                  {
                    name: '--chat-show-timestamps',
                    default: 'block',
                    effect: 'Set to none to hide timestamps.',
                  },
                ]}
              />
              <Pre lang="css">{`:root {
  --chat-font-size: 20px;
  --chat-message-gap: 12px;
  --chat-show-timestamps: none;   /* hide timestamps */
}`}</Pre>

              <h3>Finer control: target the chat parts</h3>
              <p>
                For anything the variables don&apos;t cover, style the overlay&apos;s elements
                directly. These class names and data attributes are stable and safe to target:
              </p>
              <HookTable
                rows={[
                  {
                    selector: '.overlay-live-body',
                    kind: 'class',
                    targets: 'The whole message list container.',
                  },
                  { selector: '.chat-message', kind: 'class', targets: 'One chat message bubble.' },
                  { selector: '.chat-username', kind: 'class', targets: 'The author name.' },
                  {
                    selector: '.platform-badge',
                    kind: 'class',
                    targets: 'The platform tag/icon next to the name.',
                  },
                  {
                    selector: '.event-message',
                    kind: 'class',
                    targets: 'A sub / raid / super chat / gift alert.',
                  },
                  {
                    selector: '[data-platform="twitch"]',
                    kind: 'attribute',
                    targets: 'Messages from a platform (twitch, youtube, kick, tiktok, discord).',
                  },
                  {
                    selector: '[data-username="…"]',
                    kind: 'attribute',
                    targets: 'Messages from a specific user.',
                  },
                ]}
              />
              <p>Example — give each platform its own accent stripe:</p>
              <Pre lang="css">{`.chat-message[data-platform="twitch"]  { border-left: 4px solid #9146FF; }
.chat-message[data-platform="youtube"] { border-left: 4px solid #FF0000; }
.chat-message[data-platform="kick"]    { border-left: 4px solid #53FC18; }

/* Make raid alerts pop */
.event-message[class*="raid"] { transform: scale(1.1); }`}</Pre>
              <DevCallout>
                Want ready-made examples? Every built-in theme is just a CSS file you can read and
                borrow from on{' '}
                <a
                  href="https://github.com/caesarakalaeii/all-chat/tree/main/docs/overlay-themes"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-twitch hover:underline"
                >
                  GitHub
                </a>
                . Already have a theme from another tool, or want help writing one? Ask in our{' '}
                <a
                  href={DISCORD_INVITE_URL}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-twitch hover:underline"
                >
                  Discord
                </a>{' '}
                and we&apos;re happy to help.
              </DevCallout>
            </section>

            {/* Fonts */}
            <section id="fonts">
              <h2>Custom fonts</h2>
              <p>
                You can pull in a Google Font with a normal <Code>@import</Code>, then use it
                anywhere in your CSS. To protect your viewers&apos; privacy, fonts are served
                through All-Chat rather than directly from Google, so only these families are
                available:
              </p>
              <p className="text-sm text-text-sub">
                Barlow, Barlow Condensed, Bebas Neue, Exo 2, Inter, Monoton, Montserrat, Nunito,
                Open Sans, Orbitron, Oswald, Poppins, Press Start 2P, Rajdhani, Roboto, Share Tech
                Mono, Source Code Pro, Space Grotesk, VT323.
              </p>
              <Pre lang="css">{`@import url('https://fonts.googleapis.com/css2?family=Press+Start+2P&display=swap');

:root { --chat-font-family: 'Press Start 2P', monospace; }`}</Pre>
              <p>
                Requesting a family that isn&apos;t on the list simply won&apos;t load — pick
                another or leave the default.
              </p>
            </section>

            {/* Premium */}
            <section id="premium">
              <h2>Premium</h2>
              <p>
                All-Chat is free and open source. A Premium subscription — via Patreon, connected in{' '}
                <strong>Settings → Premium</strong> — unlocks:
              </p>
              <ul>
                <li>
                  <strong>Moderate from your overlay</strong> — delete, timeout and ban from the
                  chat monitor.
                </li>
                <li>
                  <strong>ElevenLabs text-to-speech</strong> — premium TTS voices (basic browser TTS
                  is free).
                </li>
                <li>
                  <strong>YouTube stream selection</strong> — choose which stream to follow on
                  multi-stream channels (the free default follows the first one found).
                </li>
                <li>
                  <strong>Shared chat</strong> — share your overlay with other streamers.
                </li>
                <li>
                  <strong>Viewer flairs</strong> — animated gradient name colors.
                </li>
              </ul>
            </section>
          </div>

          {/* Footer */}
          <div className="mt-12 flex flex-col gap-3 border-t border-border pt-6 text-sm text-text-dim sm:flex-row sm:items-center sm:justify-between">
            <span>&copy; {new Date().getFullYear()} All-Chat</span>
            <div className="flex flex-wrap items-center gap-4">
              <Link href="/docs/api" className="transition-colors hover:text-text">
                Developer API
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
      </main>
    </div>
  )
}
