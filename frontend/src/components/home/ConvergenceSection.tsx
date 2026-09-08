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
 * along with this program. If not, <https://www.gnu.org/licenses/>.
 */

/**
 * ConvergenceSection — "the whole product" manifesto (mockup G).
 *
 * Five platform curves converge into one white line, then the live overlay
 * preview frame. The demo feed cycles `marketing.feed.*` messages against
 * mockup usernames; emote tokens inside the messages
 * (PogChamp, OMEGALUL, peepoHappy) render as .emote spans, so the frame shows
 * the emote language the copy promises.
 *
 * `id="how"` is the target of the hero's "how it works" link.
 */

'use client'

import { useEffect, useState } from 'react'
import { useReducedMotion } from '@/hooks/useReducedMotion'
import { interpolateElements } from '@/lib/i18n/emphasise'
import { useTranslations } from '@/lib/i18n'

// Emote tokens that may appear inside the feed messages, rendered as .emote
// spans — the copy promises native emotes, so the demo has to show them.
const FEED_EMOTES = ['PogChamp', 'OMEGALUL', 'peepoHappy'] as const

// The demo feed: lane colour, username and message key per entry. Usernames
// are mockup handles (component fixtures, not catalog copy). The initial six
// entries match the mockup's first fill.
const FEED_PEOPLE = [
  { color: 'var(--lanes-twitch)', user: 'nightowl_tv', messageStem: 'm1' },
  { color: 'var(--lanes-youtube)', user: 'PixelPioneer', messageStem: 'm2' },
  { color: 'var(--lanes-kick)', user: 'greenmachine', messageStem: 'm3' },
  { color: 'var(--lanes-twitch)', user: 'lurkerlou', messageStem: 'm4' },
  { color: 'var(--lanes-tiktok)', user: 'scroll.and.chill', messageStem: 'm5' },
  { color: 'var(--lanes-youtube)', user: 'VODEnjoyer', messageStem: 'm6' },
  { color: 'var(--lanes-kick)', user: 'clipit_quick', messageStem: 'm7' },
  { color: 'var(--lanes-twitch)', user: 'emotecollector', messageStem: 'm8' },
  { color: 'var(--lanes-tiktok)', user: 'fyp_famous', messageStem: 'm9' },
  { color: 'var(--lanes-twitch)', user: 'raidboss_rin', messageStem: 'm10' },
] as const

const INITIAL_FEED_COUNT = 6
const MAX_FEED_COUNT = 8

/**
 * Split a feed message into text and .emote runs around the emote tokens.
 * Public for the HomeClient regression test.
 */
export function renderFeedMessage(text: string): React.ReactNode[] {
  return FEED_EMOTES.reduce<React.ReactNode[]>(
    (parts, emote) =>
      parts.flatMap((part): React.ReactNode[] =>
        typeof part === 'string'
          ? part.split(emote).flatMap((segment, i, segments): React.ReactNode[] =>
              i < segments.length - 1
                ? [
                    segment,
                    <span key={`${emote}-${i}`} className="emote">
                      {emote}
                    </span>,
                  ]
                : [segment]
            )
          : [part]
      ),
    [text]
  )
}

export function ConvergenceSection() {
  const t = useTranslations()
  const reducedMotion = useReducedMotion()
  // How many messages the demo has pushed, ever. The visible window is the
  // last MAX_FEED_COUNT of those, cycling through FEED_PEOPLE like the
  // mockup's `idx++ % PEOPLE.length`.
  const [cursor, setCursor] = useState(INITIAL_FEED_COUNT)

  // Mockup loop: a new entry every 900-2700ms, keeping the last 8. Gated on
  // reduced motion — the .msg slide-in animation is covered by the global CSS
  // gate, but the cycling itself is JS and stops here.
  useEffect(() => {
    if (reducedMotion) return
    let timer: number | undefined
    const loop = () => {
      setCursor((count) => count + 1)
      timer = window.setTimeout(loop, 900 + Math.random() * 1800)
    }
    timer = window.setTimeout(loop, 900 + Math.random() * 1800)
    return () => window.clearTimeout(timer)
  }, [reducedMotion])

  const visibleCount = Math.min(cursor, MAX_FEED_COUNT)
  const visible = Array.from(
    { length: visibleCount },
    (_, i) => FEED_PEOPLE[(cursor - visibleCount + i) % FEED_PEOPLE.length]
  )
  // Keyed by absolute push index, so the newest message mounts as a fresh
  // node (the .msg slide-in plays for it) while older ones keep their nodes.
  const firstKey = cursor - visibleCount

  return (
    <section className="lanes-section converge" id="how">
      <span className="mono-label">{t('marketing.convergence.label')}</span>
      <svg viewBox="0 0 640 200" fill="none" aria-hidden="true">
        <path d="M0 15 C 200 15, 320 88, 470 97" stroke="var(--lanes-twitch)" strokeWidth="5" />
        <path d="M0 55 C 200 55, 330 94, 470 98.5" stroke="var(--lanes-youtube)" strokeWidth="5" />
        <path d="M0 100 C 200 100, 330 100, 470 100" stroke="var(--lanes-tiktok)" strokeWidth="5" />
        <path d="M0 145 C 200 145, 330 106, 470 101.5" stroke="var(--lanes-kick)" strokeWidth="5" />
        <path
          d="M0 185 C 200 185, 320 112, 470 103"
          stroke="var(--lanes-discord)"
          strokeWidth="5"
        />
        <path d="M470 100 L 640 100" stroke="#fff" strokeWidth="7" />
        <circle cx="470" cy="100" r="9" fill="#fff" />
      </svg>
      <h2>
        {t('marketing.convergence.headingTop')}
        <br />
        {t('marketing.convergence.headingBottom')}
      </h2>
      <p>
        {interpolateElements(t('marketing.convergence.body'), {
          oneUrl: <b>{t('marketing.convergence.oneUrlEmphasis')}</b>,
          platforms: <b>{t('marketing.convergence.platformsEmphasis')}</b>,
          emotes: <b>{t('marketing.convergence.emotesEmphasis')}</b>,
        })}
      </p>

      <div className="frame">
        <div className="frame-head">
          <span className="live">{t('marketing.convergence.liveLabel')}</span>
          <span>{t('marketing.convergence.frameUrl')}</span>
        </div>
        <div className="chat-feed">
          {visible.map((person, i) => (
            <div
              key={firstKey + i}
              className="msg"
              style={{ '--pc': person.color } as React.CSSProperties}
            >
              <span className="who">{person.user}</span>
              <span className="txt">
                {renderFeedMessage(t(`marketing.feed.${person.messageStem}`))}
              </span>
            </div>
          ))}
        </div>
      </div>
    </section>
  )
}
