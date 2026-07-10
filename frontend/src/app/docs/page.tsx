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

export const metadata = {
  title: 'Documentation',
  description:
    'Get your All-Chat overlay live in OBS, pick from 16 built-in themes, and make it your own — no CSS required to start, full CSS control when you want it.',
  alternates: { canonical: '/docs' },
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
  ['themes', 'Pick a theme'],
  ['customize', 'Make it your own'],
  ['custom-css', 'Go further with CSS'],
  ['fonts', 'Custom fonts'],
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
            <h1 className="text-3xl font-bold text-text">Streamer guide</h1>
            <p className="text-sm text-text-dim">
              Get your overlay into OBS, choose a look, and make it your own.
            </p>
            <p className="text-sm text-text-sub">
              Building a bot, alert box, or analytics tool?{' '}
              <Link href="/docs/api" className="text-twitch hover:underline">
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
                  stream on. Each connected platform becomes a <em>chat source</em> on that overlay —
                  mix and match as many as you like.
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

            {/* Themes */}
            <section id="themes">
              <h2>Pick a theme</h2>
              <p>
                All-Chat ships with <strong>16 built-in themes</strong> — from Modern Dark and
                Minimal to Trading Card, Comic Speech, Sticky Notes, Vaporwave, Cyberpunk and more.
                Applying
                one takes a click and needs <strong>no CSS at all</strong>.
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
                You can also preview every theme live on the{' '}
                <Link href="/">home page</Link> before you sign in.
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
                <li>There&apos;s a live preview right next to the editor, so you see changes as you type.</li>
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
                  { name: '--chat-message-color', default: '#ffffff', effect: 'Message text color.' },
                  { name: '--chat-message-gap', default: '0.75rem', effect: 'Vertical space between messages.' },
                  { name: '--chat-bubble-border-radius', default: '0.5rem', effect: 'Roundness of the message bubble.' },
                  { name: '--chat-bubble-padding', default: '0.75rem', effect: 'Padding inside each message.' },
                  { name: '--chat-avatar-size', default: '2.5rem', effect: 'Avatar width and height.' },
                  { name: '--chat-username-font-size', default: '0.875rem', effect: 'Username size.' },
                  { name: '--chat-emote-scale', default: '1', effect: 'Emote size multiplier.' },
                  { name: '--chat-show-avatars', default: 'block', effect: 'Set to none to hide avatars.' },
                  { name: '--chat-show-badges', default: 'flex', effect: 'Set to none to hide badges.' },
                  { name: '--chat-show-timestamps', default: 'block', effect: 'Set to none to hide timestamps.' },
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
                  { selector: '.overlay-live-body', kind: 'class', targets: 'The whole message list container.' },
                  { selector: '.chat-message', kind: 'class', targets: 'One chat message bubble.' },
                  { selector: '.chat-username', kind: 'class', targets: 'The author name.' },
                  { selector: '.platform-badge', kind: 'class', targets: 'The platform tag/icon next to the name.' },
                  { selector: '.event-message', kind: 'class', targets: 'A sub / raid / super chat / gift alert.' },
                  { selector: '[data-platform="twitch"]', kind: 'attribute', targets: 'Messages from a platform (twitch, youtube, kick, tiktok, discord).' },
                  { selector: '[data-username="…"]', kind: 'attribute', targets: 'Messages from a specific user.' },
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
                  href="https://discord.gg/xCGBSuz39P"
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
                anywhere in your CSS. To protect your viewers&apos; privacy, fonts are served through
                All-Chat rather than directly from Google, so only these families are available:
              </p>
              <p className="text-sm text-text-sub">
                Barlow, Barlow Condensed, Bebas Neue, Exo 2, Inter, Monoton, Montserrat, Nunito,
                Open Sans, Orbitron, Oswald, Poppins, Press Start 2P, Rajdhani, Roboto, Share Tech
                Mono, Source Code Pro, Space Grotesk, VT323.
              </p>
              <Pre lang="css">{`@import url('https://fonts.googleapis.com/css2?family=Press+Start+2P&display=swap');

:root { --chat-font-family: 'Press Start 2P', monospace; }`}</Pre>
              <p>
                Requesting a family that isn&apos;t on the list simply won&apos;t load — pick another
                or leave the default.
              </p>
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
      </div>
    </div>
  )
}
