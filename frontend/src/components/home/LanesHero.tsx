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
 * LanesHero — the homepage hero (mockup G "lanes").
 *
 * Fixed mono chrome (brand + nav) over five full-width platform lanes whose
 * heights are proportional to each platform's share of this week's messages.
 * The lanes carry a decorative chat marquee each; the CSS drift animation is
 * collapsed by the global reduced-motion gate, so the marquee is pure markup
 * and needs no JS gating.
 *
 * Marquee rows are generated ONCE at module load: the mockup randomises
 * usernames/messages per row, and React re-rendering (auth store, stats
 * fetch, counter tick) must not re-roll the lane content, or the marquee
 * visibly jumps on every parent render. The seeds below walk both pools
 * deterministically instead of Math.random(), so the generated markup is
 * stable and identical on server and client — hydration never mismatches.
 */

'use client'

import { useEffect, useRef, useState } from 'react'
import { type MessageKey, useTranslations } from '@/lib/i18n'
import { DISCORD_INVITE_URL } from '@/lib/constants'
import { useReducedMotion } from '@/hooks/useReducedMotion'

// Marquee chatter: how many `marketing.flow<Platform>.mN` keys each lane
// draws from. Kept beside the usernames so both stay in sync with the
const MARQUEE_MESSAGE_COUNTS = {
  twitch: 15,
  youtube: 8,
  tiktok: 7,
  kick: 6,
  discord: 6,
} as const

// Catalog group holding each lane's marquee strings. The catalog caps key
// depth at three segments (messages.test.ts), so the chatter lives in flat
// flow* groups instead of a lanes.flow.<platform> nest.
const FLOW_GROUPS = {
  twitch: 'flowTwitch',
  youtube: 'flowYoutube',
  tiktok: 'flowTiktok',
  kick: 'flowKick',
  discord: 'flowDiscord',
} as const

// Decorative handles per lane — mockup fixtures, deliberately not in the
// catalog (translating a username produces a different person, not a
// translation).
const MARQUEE_USERS = {
  twitch: ['xqc_fan88', 'nightowl_tv', 'emotecollector', 'lurkerlou', 'pog_fern'],
  youtube: ['VODEnjoyer', 'PixelPioneer', 'casualfriday', 'superchatter99'],
  tiktok: ['scroll.and.chill', 'fyp_famous', 'livetoker'],
  kick: ['greenmachine', 'clipit_quick', 'w_andy'],
  discord: ['mod_mara', 'sunny__', 'fifthplatfan'],
} as const

type LanePlatform = keyof typeof MARQUEE_MESSAGE_COUNTS

// The message-stem keys of one lane, pulled out of the literal MessageKey
// union — dynamic `m${n}` templates are not part of that union.
type LaneMessageKey = Extract<MessageKey, `marketing.${(typeof FLOW_GROUPS)[LanePlatform]}.`>

/** One item of a marquee row: `<b>user</b> message` with an optional emote. */
type MarqueeItem = { user: string; messageKey: LaneMessageKey; emote?: string }

/** One marquee row: items plus its drift duration/direction. */
type MarqueeRow = { items: MarqueeItem[]; duration: number; reverse: boolean }

// Decorative emote tokens per lane — mockup fixtures like the usernames
// above, deliberately not in the catalog. Rendered as .emote spans so CSS
// can dim them below the text (texture, not content).
const MARQUEE_EMOTES = {
  twitch: ['KEKW', 'POGGERS', 'LULW', 'catJAM'],
  youtube: ['💎', '🔥', '🎉'],
  tiktok: ['😭', '💅', '🔥'],
  kick: ['EZ', 'Crazy', 'GG'],
  discord: ['👋', '🎉', '👀'],
} as const

// Lane order and row count: taller lanes carry more marquee rows. Denser
// than the first pass (feedback: "more text, not readable") — two rows for
// every lane, three for twitch.
const LANES: ReadonlyArray<{ platform: LanePlatform; rows: number }> = [
  { platform: 'twitch', rows: 3 },
  { platform: 'youtube', rows: 2 },
  { platform: 'tiktok', rows: 2 },
  { platform: 'kick', rows: 2 },
  { platform: 'discord', rows: 2 },
]


// The seeds below must not be multiples of any pool size (5, 4, 3, 15, 8, 7,
// 6), or a stride would cycle through only a fraction of a pool. 137 and 7
// are coprime to all of them.
const MARQUEE_ROWS: ReadonlyArray<{ platform: LanePlatform; rows: MarqueeRow[] }> = LANES.map(
  ({ platform, rows }, laneIndex) => {
    const users = MARQUEE_USERS[platform]
    const emotes = MARQUEE_EMOTES[platform]
    const messageCount = MARQUEE_MESSAGE_COUNTS[platform]
    const rowSeeds = Array.from({ length: rows }, (_, rowIndex) => ({
      rowIndex,
      seed: (laneIndex + 1) * 53 + (rowIndex + 1) * 29,
    }))
    return {
      platform,
      rows: rowSeeds.map(({ rowIndex, seed }) => ({
        items: Array.from({ length: 14 }, (_, i) => {
          const itemSeed = seed + i * 137
          return {
            user: users[itemSeed % users.length],
            messageKey: `marketing.${FLOW_GROUPS[platform]}.m${
              (itemSeed % messageCount) + 1
            }` as MarqueeItem['messageKey'],
            // An emote every third item — dense enough to read as chat
            // texture, sparse enough that the rows still look like text.
            ...(itemSeed % 3 === 0 ? { emote: emotes[itemSeed % emotes.length] } : {}),
          }
        }),
        duration: 30 + (seed % 26) + rowIndex * 12,
        reverse: (laneIndex + rowIndex) % 2 === 1,
      })),
    }
  }
)

export interface LanesHeroProps {
  /** Ticking all-time counter, formatted by the parent (shared with Numbers). */
  totalDisplay: string
  /** Display name of the logged-in user; undefined when logged out. */
  userName?: string
  /** Live-overlay count for the welcome-back note. */
  overlaysLive: number
  /** Weekly message counts per platform; lane heights follow these. */
  platformShares: Record<string, number> | null
  onCta: () => void
}

// Fallback lane weights when /stats has not landed (or failed): the mockup
// proportions. Same ranking as the live data usually produces.
const FALLBACK_SHARES: Record<LanePlatform, number> = {
  twitch: 40,
  youtube: 26,
  tiktok: 14,
  kick: 10,
  discord: 7,
}

// Decorative morphing: each lane's flex-grow oscillates around its real
// share by up to ±35% (relative), resampled every MORPH_INTERVAL_MS.
// Feedback intent: the bars visibly compete, showing the heights are live
// data — but this is NOT real history; no weekly series exists server-side.
const MORPH_INTERVAL_MS = 4000
const MORPH_AMPLITUDE = 0.35

function morphedShare(share: number, seed: number, tick: number): number {
  // Pseudo-random but deterministic per (lane, tick): a sine walked through
  // two coprime strides so no two lanes breathe in sync.
  const phase = Math.sin((seed * 31 + tick * 17) * 1.7)
  const factor = 1 + phase * MORPH_AMPLITUDE
  return Math.max(share * factor, 2)
}

export function LanesHero({
  totalDisplay,
  userName,
  overlaysLive,
  platformShares,
  onCta,
}: LanesHeroProps) {
  const t = useTranslations()
  const reducedMotion = useReducedMotion()
  const stageRef = useRef<HTMLDivElement>(null)
  const [scrollY, setScrollY] = useState(0)
  const [morphTick, setMorphTick] = useState(0)
  const isLoggedIn = userName !== undefined

  // The label splits into spans so no JSX carries a literal space between the
  // count and the words; .lanes-total spaces them with a flex gap.
  const totalWords = t('marketing.lanes.totalLabel').split(' ')

  // Hero scroll effect (feedback: "reveal + hero scroll"): as the hero
  // leaves the viewport the stage sinks and dims — the visitor's scroll
  // carries it away. One rAF-coalesced scroll listener; the effect is
  // scroll-driven but cheap (two style writes on one element), so it runs
  // even without the reveal observer. Skipped entirely under reduced
  // motion.
  useEffect(() => {
    if (reducedMotion) return
    let raf = 0
    const onScroll = () => {
      if (raf !== 0) return
      raf = window.requestAnimationFrame(() => {
        raf = 0
        setScrollY(window.scrollY)
      })
    }
    window.addEventListener('scroll', onScroll, { passive: true })
    return () => {
      window.removeEventListener('scroll', onScroll)
      if (raf !== 0) window.cancelAnimationFrame(raf)
    }
  }, [reducedMotion])

  // Decorative lane-height morphing (see morphedShare). Real shares when
  // reduced motion is on — no interval, static heights.
  useEffect(() => {
    if (reducedMotion) return
    const timer = window.setInterval(() => setMorphTick((tick) => tick + 1), MORPH_INTERVAL_MS)
    return () => window.clearInterval(timer)
  }, [reducedMotion])

  // Real weekly shares, zero-guarded: a platform with no messages this
  // week falls back to its mockup weight rather than collapsing to zero
  // (the lane must stay visible; the wordmark carries the brand).
  const laneWeights = LANES.map(({ platform }, i) => {
    const share = platformShares?.[platform]
    const base = share !== undefined && share > 0 ? share : FALLBACK_SHARES[platform]
    return morphedShare(base, i + 1, morphTick)
  })

  // Hero progress: 0 at rest, 1 by the time the stage has scrolled one
  // viewport height. Drives the sink/dim via CSS custom properties.
  const stageH = stageRef.current?.offsetHeight ?? window.innerHeight
  const progress = Math.min(scrollY / Math.max(stageH, 1), 1)


  return (
    <>
      {/* Fixed mono chrome over the lanes — mix-blend-mode: difference. */}
      <div className="lanes-chrome lanes-tl">{t('marketing.lanes.brand')}</div>
      <nav className="lanes-chrome lanes-tr" aria-label={t('marketing.lanes.navLabel')}>
        <a href="/docs#themes">{t('marketing.lanes.navThemes')}</a>
        <a href="/docs">{t('marketing.lanes.navDocs')}</a>
        <a href={DISCORD_INVITE_URL}>{t('marketing.lanes.navDiscord')}</a>
        {isLoggedIn ? (
          <a className="dash-chip" href="/dashboard">
            {t('marketing.lanes.navDashboardChip')}
          </a>
        ) : (
          <a href="#get-started">{t('marketing.lanes.navDashboard')}</a>
        )}
      </nav>

      <header
        className="lanes-stage"
        ref={stageRef}
        style={
          reducedMotion
            ? undefined
            : {
                // Sink + dim as the hero scrolls out; the CSS consumes the
                // progress value, so the exact curve lives in globals.css.
                '--hero-progress': progress,
              } as React.CSSProperties
        }
      >
        {MARQUEE_ROWS.map(({ platform, rows }, laneIndex) => (
          <div
            key={platform}
            className="lane"
            data-p={platform}
            style={{ flexGrow: laneWeights[laneIndex] }}
          >
            <div className="wordmark" aria-hidden="true">
              {platform.toUpperCase()}
            </div>
            {rows.map(({ items, duration, reverse }, i) => (
              <div
                key={i}
                className="flow"
                aria-hidden="true"
                style={{
                  animationDuration: `${duration}s`,
                  ...(reverse ? { animationDirection: 'reverse' } : {}),
                }}
              >
                {items.map((item, j) => (
                  <span key={j}>
                    {j > 0 && <span className="sep">·</span>}
                    <b>{item.user}</b> {t(item.messageKey)}
                    {item.emote !== undefined && <span className="emote">{item.emote}</span>}
                  </span>
                ))}
                <span className="sep">·</span>
                {items.map((item, j) => (
                  <span key={`d${j}`}>
                    {j > 0 && <span className="sep">·</span>}
                    <b>{item.user}</b> {t(item.messageKey)}
                    {item.emote !== undefined && <span className="emote">{item.emote}</span>}
                  </span>
                ))}
              </div>
            ))}
          </div>
        ))}

        {/* Headline chips over the lanes. */}
        <div className="lanes-headline">
          <div className="warkicker">{t('marketing.lanes.kicker')}</div>
          <h1>
            {t('marketing.lanes.titleTop')}
            <br />
            {t('marketing.lanes.titleBottom')}
          </h1>
          <div className="lanes-total">
            <b>{totalDisplay}</b>
            {totalWords.map((word, i) => (
              <span key={i}>{word}</span>
            ))}
          </div>
        </div>

        {/* Bottom CTA bar. */}
        <div className="ctabar">
          <span className="note">
            {isLoggedIn && overlaysLive > 0
              ? t('marketing.lanes.welcomeNote', { name: userName, count: overlaysLive })
              : t('marketing.lanes.ctaNote')}
          </span>
          <button type="button" className="cta-go" onClick={onCta}>
            {t(isLoggedIn ? 'marketing.lanes.backToDashboard' : 'marketing.lanes.cta')}
          </button>
          <a href="#how">{t('marketing.lanes.howItWorks')}</a>
        </div>
      </header>
    </>
  )
}
