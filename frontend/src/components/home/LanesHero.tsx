/**
 * This file is part of All-Chat.
 * Copyright (C) 2026 caesarakalaeii
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published
 * by the Free Software Foundation, either version 3 of the License, or
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
 * LanesHero — the homepage hero (mockup G "lanes").
 *
 * Fixed mono chrome (brand + nav) over five full-width platform lanes whose
 * heights track live traffic: weekly share scaled by each platform's
 * current message rate (HomeClient polls /api/v1/stats). A WebGL canvas
 * behind the lanes draws the fills and animates their boundaries with a
 * small shimmer (useLaneWaves); the DOM lanes keep only the text (marquee,
 * wordmarks) and follow the same weights from a shared rAF loop. The
 * counter is React state only — delivered totals must never move down.
 * Without WebGL2 the CSS lane backgrounds remain — color without waves.
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
import { type MessageKey, formatNumber, useTranslations } from '@/lib/i18n'
import { DISCORD_INVITE_URL } from '@/lib/constants'
import { useReducedMotion } from '@/hooks/useReducedMotion'
import { LANE_COUNT, useLaneWaves } from '@/hooks/useLaneWaves'

// Marquee chatter: how many `marketing.flow<Platform>.mN` keys each lane
// draws from. Kept beside the usernames so both stay in sync with the
// curated pools in marketing.ts.
const MARQUEE_MESSAGE_COUNTS = {
  twitch: 22,
  youtube: 16,
  tiktok: 14,
  kick: 13,
  discord: 12,
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
  twitch: ['xqc', 'forsen', 'ludwig', 'nymn', 'sodapoppin', 'peakd', 'zentreya'],
  youtube: ['ludwig', 'moistcr1tikal', 'veibae', 'sykkuno', 'filian'],
  tiktok: ['khaby', 'zachking', 'charli', 'bella', 'spencer'],
  kick: ['xqc', 'amouranth', 'ross', 'ac7ionman', 'gerard'],
  discord: ['groque', 'caesar', 'moers', 'lana', 'pixi'],
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
// above, deliberately not in the catalog. Real 7TV emote names (feedback:
// the emoji read off-platform; the product promises native 7TV/BTTV/FFZ
// rendering, so the marquee should speak it). Rendered as .emote spans so
// CSS can dim them below the text (texture, not content).
const MARQUEE_EMOTES = {
  twitch: ['KEKW', 'POGGERS', 'catJAM', 'peepoHappy', 'SourPls', 'LULW', 'monkaS', 'pog'],
  youtube: ['COZYME', 'PogChamp', 'BASED', 'peepoSad', 'heart', 'GIGACHAD'],
  tiktok: ['pog', 'KEKW', 'catJAM', 'Shy', 'peepoLove'],
  kick: ['EZ', 'OMEGALUL', 'Crazy', 'GIGACHAD', 'peepoLol', 'Clap'],
  discord: ['peepoHey', 'catJAM', 'KEKW', 'pogChamp', 'heart'],
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

// The seeds below must not be multiples of any pool size (users 7, 5, 5, 5,
// 5; emotes 8, 6, 5, 6, 5; messages 22, 16, 14, 13, 12), or a stride would
// cycle through only a fraction of a pool. 137 and 29 are coprime to all.
const MARQUEE_ROWS: ReadonlyArray<{ platform: LanePlatform; rows: MarqueeRow[] }> = LANES.map(
  ({ platform, rows }, laneIndex) => {
    const users = MARQUEE_USERS[platform]
    const emotes = MARQUEE_EMOTES[platform]
    const messageCount = MARQUEE_MESSAGE_COUNTS[platform]
    const group = FLOW_GROUPS[platform]
    return {
      platform,
      rows: Array.from({ length: rows }, (_, rowIndex) => {
        // Per-row seed: lane/row indices folded in so every row walks its
        // pools from a different offset (deterministic → hydration-safe).
        let itemSeed = (laneIndex + 1) * 53 + (rowIndex + 1) * 29
        const items = Array.from({ length: 14 }, (_, itemIndex) => {
          itemSeed += 137
          const user = users[itemSeed % users.length]
          const messageKey = `marketing.${group}.m${(itemSeed % messageCount) + 1}` as LaneMessageKey
          // Emote every third item — sparse enough to stay texture.
          const emote = itemSeed % 3 === 0 ? emotes[itemSeed % emotes.length] : undefined
          return { user, messageKey, emote }
        })
        return {
          items,
          duration: 90 + ((laneIndex * 7 + rowIndex * 13) % 40),
          reverse: (laneIndex + rowIndex) % 2 === 1,
        }
      }),
    }
  }
)

export interface LanesHeroProps {
  /** Ticking all-time count (shared with Numbers; formatted locally). */
  totalCount: number
  /** Display name of the logged-in user; undefined when logged out. */
  userName?: string
  /** Live-overlay count for the welcome-back note. */
  overlaysLive: number
  /** Weekly message counts per platform; lane heights follow these. */
  platformShares: Record<string, number> | null
  /**
   * Live per-lane rate modulation, LANES order: 1 = platform moving at its
   * weekly pace. Derived by HomeClient from consecutive stats samples.
   */
  laneModulations: number[]
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

export function LanesHero({
  totalCount,
  userName,
  overlaysLive,
  platformShares,
  laneModulations,
  onCta,
}: LanesHeroProps) {
  const t = useTranslations()
  const reducedMotion = useReducedMotion()
  const stageRef = useRef<HTMLDivElement>(null)
  const canvasRef = useRef<HTMLCanvasElement>(null)

  // Lane elements are addressed by index every frame; a ref array avoids
  // re-querying the DOM from the animation loop.
  const laneRefs = useRef<(HTMLDivElement | null)[]>([])
  const [scrollY, setScrollY] = useState(0)
  const isLoggedIn = userName !== undefined

  // The label splits into spans so no JSX carries a literal space between the
  // count and the words; .lanes-total spaces them with a flex gap.
  const totalWords = t('marketing.lanes.totalLabel').split(' ')

  // Real weekly shares, zero-guarded: a platform with no messages this
  // week falls back to its mockup weight rather than collapsing to zero
  // (the lane must stay visible; the wordmark carries the brand).
  const laneBases = LANES.map(({ platform }) => {
    const share = platformShares?.[platform]
    return share !== undefined && share > 0 ? share : FALLBACK_SHARES[platform]
  })
  const { weightsRef, activeRef } = useLaneWaves(canvasRef, {
    bases: laneBases,
    modulations: laneModulations,
    reducedMotion,
  })


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

  // DOM half of the lane waves: the canvas fills the bands (useLaneWaves)
  // and mutates weightsRef; this loop applies the same weights to the flex
  // lanes so the text rows track the WebGL boundaries. The counter is
  // deliberately not touched here: it renders the parent's ticking count
  // (React state), so a delivered-message total can only ever go up —
  // multiplying it by the wave scale made it visibly count down.
  useEffect(() => {
    if (reducedMotion) return
    let raf = 0
    const step = () => {
      const weights = weightsRef.current
      for (let i = 0; i < LANE_COUNT; i++) {
        const lane = laneRefs.current[i]
        if (lane) lane.style.flexGrow = String(weights[i])
      }
      // Once the canvas has actually drawn a frame, drop the CSS lane
      // fills — canvas tint would stack on top of them.
      if (activeRef.current && stageRef.current) stageRef.current.classList.add('gl')
      raf = window.requestAnimationFrame(step)
    }
    raf = window.requestAnimationFrame(step)
    return () => window.cancelAnimationFrame(raf)
  }, [reducedMotion, weightsRef, activeRef])

  // Reduced-motion + WebGL2: the canvas paints one static frame and this
  // loop is off, so the class swap above never runs — do it once here.
  useEffect(() => {
    if (!reducedMotion) return
    const timer = window.setTimeout(() => {
      if (activeRef.current && stageRef.current) stageRef.current.classList.add('gl')
    }, 250)
  }, [reducedMotion, activeRef])

  // Hero progress: 0 at rest, 1 by the time the stage has scrolled one
  // viewport height. Drives the sink/dim via CSS custom properties. The
  // prerender pass runs this with scrollY 0 on the server, where neither
  // stageRef nor window exists — both fall back to a safe 0 progress.
  const stageH = stageRef.current?.offsetHeight ?? 0
  const progress = scrollY > 0 ? Math.min(scrollY / Math.max(stageH, 1), 1) : 0

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
        {/* WebGL lane fills; the DOM lanes below carry only text. */}
        <canvas className="lanes-canvas" ref={canvasRef} aria-hidden="true" />
        {MARQUEE_ROWS.map(({ platform, rows }, laneIndex) => (
          <div
            key={platform}
            ref={(el) => {
              laneRefs.current[laneIndex] = el
            }}
            className="lane"
            data-p={platform}
            style={{ flexGrow: laneBases[laneIndex] }}
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
            <b>{formatNumber(totalCount)}</b>
            {totalWords.map((word, i) => (
              <span key={i}>{word}</span>
            ))}
          </div>
          {/* The multistream objection, answered on the spot: one merged
              chat, one mod queue. Copy lives in the catalog. */}
          <p className="lanes-manifesto">{t('marketing.lanes.manifesto')}</p>
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
