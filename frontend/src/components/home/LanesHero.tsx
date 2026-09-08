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

import { type MessageKey, useTranslations } from '@/lib/i18n'
import { DISCORD_INVITE_URL } from '@/lib/constants'

// Marquee chatter: how many `marketing.flow<Platform>.mN` keys each lane
// draws from. Kept beside the usernames so both stay in sync with the
// catalog.
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

/** One item of a marquee row: `<b>user</b> message`. */
type MarqueeItem = { user: string; messageKey: LaneMessageKey }

/** One marquee row: 14 items plus its drift duration/direction. */
type MarqueeRow = { items: MarqueeItem[]; duration: number; reverse: boolean }

// Lane order and row count: twitch/youtube get two marquee rows, the three
// shorter lanes one. Same order as the mockup (twitch, youtube, tiktok, kick,
// discord).
const LANES: ReadonlyArray<{ platform: LanePlatform; rows: number }> = [
  { platform: 'twitch', rows: 2 },
  { platform: 'youtube', rows: 2 },
  { platform: 'tiktok', rows: 1 },
  { platform: 'kick', rows: 1 },
  { platform: 'discord', rows: 1 },
]

// The seeds below must not be multiples of any pool size (5, 4, 3, 15, 8, 7,
// 6), or a stride would cycle through only a fraction of a pool. 137 and 7
// are coprime to all of them.
const MARQUEE_ROWS: ReadonlyArray<{ platform: LanePlatform; rows: MarqueeRow[] }> = LANES.map(
  ({ platform, rows }, laneIndex) => {
    const users = MARQUEE_USERS[platform]
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
  onCta: () => void
}

export function LanesHero({ totalDisplay, userName, overlaysLive, onCta }: LanesHeroProps) {
  const t = useTranslations()
  const isLoggedIn = userName !== undefined

  // The label splits into spans so no JSX carries a literal space between the
  // count and the words; .lanes-total spaces them with a flex gap.
  const totalWords = t('marketing.lanes.totalLabel').split(' ')

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

      <header className="lanes-stage">
        {MARQUEE_ROWS.map(({ platform, rows }) => (
          <div key={platform} className="lane" data-p={platform}>
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
                  </span>
                ))}
                <span className="sep">·</span>
                {items.map((item, j) => (
                  <span key={`d${j}`}>
                    {j > 0 && <span className="sep">·</span>}
                    <b>{item.user}</b> {t(item.messageKey)}
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
