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
 * Social card (link embeds): a still frame of the lanes hero. Five stacked
 * platform lanes with marquee chatter and tone-on-tone wordmarks, the
 * headline plates across them. In June the lanes turn rainbow and the
 * chatter switches to the pride pool, like the hero.
 *
 * Pride month is read from the server clock in UTC, so the route files that
 * call this regenerate hourly (revalidate) rather than bake once at build.
 * The headline varies per route: the homepage card and the
 * /browser-extension card share everything else.
 */

import { ImageResponse } from 'next/og'
import { readFile } from 'node:fs/promises'
import { join } from 'node:path'
import { LANE_COLORS, PRIDE_LANE_COLORS } from '@/components/home/lanePalette'
import { getTranslations } from '@/lib/i18n'

// Module scope, so getTranslations rather than the hook.
const t = getTranslations()
type MessageKey = Parameters<typeof t>[0]

export const SOCIAL_CARD_SIZE = { width: 1200, height: 630 }

const INK = '#050508'
const PAPER = '#f4f3ef'

// The hero's fallback proportions (LanesHero FALLBACK_SHARES) through the
// same normalization as laneBaseWeights: a 10% floor per lane, the other
// half split by share. A card has no live stats to follow.
const LANES = [
  { platform: 'TWITCH', group: 'flowTwitch', share: 40 },
  { platform: 'YOUTUBE', group: 'flowYoutube', share: 26 },
  { platform: 'TIKTOK', group: 'flowTiktok', share: 14 },
  { platform: 'KICK', group: 'flowKick', share: 10 },
  { platform: 'DISCORD', group: 'flowDiscord', share: 7 },
] as const
const SHARE_TOTAL = LANES.reduce((sum, lane) => sum + lane.share, 0)

// Marquee strings per lane: the first few of each pool. The pride pool is
// shared by all lanes, so each lane starts at its own offset into it.
const CHATTER_PER_LANE = 7

export interface LanesCardText {
  /** The small plate above the headline. */
  kicker: string
  /** The two display lines. */
  titleTop: string
  titleBottom: string
  /** The plate under the headline. */
  chip: string
  /** Display size in px; long lines need less than the homepage's 112. */
  titleSize?: number
}

export async function renderLanesCard({
  kicker,
  titleTop,
  titleBottom,
  chip,
  titleSize = 112,
}: LanesCardText): Promise<ImageResponse> {
  const fontsDir = join(process.cwd(), 'public', 'fonts')
  const [archivoBlack, spaceMono, spaceMonoBold] = await Promise.all([
    readFile(join(fontsDir, 'ArchivoBlack-Regular.ttf')),
    readFile(join(fontsDir, 'SpaceMono-Regular.ttf')),
    readFile(join(fontsDir, 'SpaceMono-Bold.ttf')),
  ])
  const pride = new Date().getUTCMonth() === 5
  const colors = pride ? PRIDE_LANE_COLORS : LANE_COLORS

  return new ImageResponse(
    <div
      style={{
        width: '100%',
        height: '100%',
        display: 'flex',
        flexDirection: 'column',
        position: 'relative',
        background: INK,
        fontFamily: 'Space Mono',
      }}
    >
      {LANES.map(({ platform, group, share }, laneIndex) => {
        const weight = 0.1 + (0.5 * share) / SHARE_TOTAL
        const chatter = Array.from({ length: CHATTER_PER_LANE }, (_, i) =>
          t(
            (pride
              ? `marketing.flowPride.m${((laneIndex * 4 + i) % 22) + 1}`
              : `marketing.${group}.m${i + 1}`) as MessageKey
          )
        )
        return (
          <div
            key={platform}
            style={{
              flexGrow: weight,
              flexBasis: 0,
              display: 'flex',
              alignItems: 'center',
              position: 'relative',
              overflow: 'hidden',
              background: colors[laneIndex],
            }}
          >
            <div
              style={{
                position: 'absolute',
                right: -8,
                top: 0,
                bottom: 0,
                display: 'flex',
                alignItems: 'center',
                fontFamily: 'Archivo Black',
                fontSize: Math.min(150, Math.round(weight * 630 * 0.95)),
                letterSpacing: '-0.02em',
                color: 'rgba(5, 5, 8, 0.45)',
              }}
            >
              {platform}
            </div>
            <div
              style={{
                display: 'flex',
                whiteSpace: 'nowrap',
                fontSize: 17,
                fontWeight: 700,
                color: INK,
                marginLeft: -40 - laneIndex * 37,
              }}
            >
              {chatter.join('   ·   ')}
            </div>
          </div>
        )
      })}

      {/* Brand chip, top left, like the hero's fixed chrome. */}
      <div
        style={{
          position: 'absolute',
          top: 28,
          left: 32,
          display: 'flex',
          background: 'rgba(5, 5, 8, 0.62)',
          color: PAPER,
          fontSize: 22,
          fontWeight: 700,
          padding: '8px 16px',
        }}
      >
        {t('marketing.lanes.brand')}
      </div>

      {/* Headline plates across the lanes. */}
      <div
        style={{
          position: 'absolute',
          top: 0,
          left: 0,
          width: '100%',
          height: '100%',
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
          justifyContent: 'center',
        }}
      >
        <div
          style={{
            display: 'flex',
            background: colors[1],
            color: '#0a0a0e',
            fontSize: 18,
            fontWeight: 700,
            letterSpacing: '0.22em',
            padding: '8px 16px',
            marginBottom: 22,
          }}
        >
          {kicker}
        </div>
        <div
          style={{
            display: 'flex',
            flexDirection: 'column',
            alignItems: 'center',
            background: '#000',
            color: PAPER,
            fontFamily: 'Archivo Black',
            fontSize: titleSize,
            lineHeight: 1.06,
            padding: '6px 32px 14px',
          }}
        >
          <div style={{ display: 'flex' }}>{titleTop}</div>
          <div style={{ display: 'flex' }}>{titleBottom}</div>
        </div>
        <div
          style={{
            display: 'flex',
            marginTop: 22,
            background: '#000',
            color: colors[3],
            fontSize: 20,
            fontWeight: 700,
            letterSpacing: '0.12em',
            padding: '10px 18px',
          }}
        >
          {chip}
        </div>
      </div>
    </div>,
    {
      ...SOCIAL_CARD_SIZE,
      fonts: [
        { name: 'Archivo Black', data: archivoBlack, style: 'normal', weight: 400 },
        { name: 'Space Mono', data: spaceMono, style: 'normal', weight: 400 },
        { name: 'Space Mono', data: spaceMonoBold, style: 'normal', weight: 700 },
      ],
    }
  )
}
