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
 * The streamer's own credit roll copy and text sizes (migration 112).
 *
 * A custom text is keyed by the message's leaf name under viewerOverlay.credits
 * and replaces that message in every UI language, which is how a roll gets
 * French (or any other language) copy without a French UI catalog. A text size
 * is a percentage of an element's default size.
 */

import type { TFunction } from '@/lib/i18n'
import { viewerOverlay } from '@/lib/i18n/messages/en/viewerOverlay'
import type { CreditRollConfig, CreditRollResponse, Leaderboards } from '@/lib/types/overlay'

type CreditMessage = keyof typeof viewerOverlay.credits

const PREFIX = 'viewerOverlay.credits.'
const PLACEHOLDER = /\{(\w+)\}/g

/** The editable messages, in the order the editor lists them. */
export const CREDIT_TEXT_GROUPS = [
  { id: 'header', keys: ['heading', 'subheading', 'session', 'duration'] },
  {
    id: 'categories',
    keys: [
      'topSubscribers',
      'topGifters',
      'topCheerers',
      'topChannelPoints',
      'topRaiders',
      'topSuperChats',
      'newFollowers',
      'topMembers',
      'topCoinChests',
      'topSharers',
      'topLikers',
    ],
  },
  {
    id: 'values',
    keys: [
      'valueBitsOne',
      'valueBitsMany',
      'valueMonthsOne',
      'valueMonthsMany',
      'valueViewersOne',
      'valueViewersMany',
      'valuePointsOne',
      'valuePointsMany',
      'valuePointsNamed',
      'valueGiftsOne',
      'valueGiftsMany',
      'valueDiamondsOne',
      'valueDiamondsMany',
      'valueCoinsOne',
      'valueCoinsMany',
      'valueLikesOne',
      'valueLikesMany',
      'valueSharesOne',
      'valueSharesMany',
    ],
  },
  {
    id: 'durationWords',
    keys: [
      'durationHourOne',
      'durationHourMany',
      'durationMinuteOne',
      'durationMinuteMany',
      'durationHoursAndMinutes',
      'durationJustStarted',
    ],
  },
  { id: 'clips', keys: ['nowPlaying', 'clipViews', 'clipCounter'] },
  { id: 'footer', keys: ['thanks', 'seeYou', 'empty'] },
] as const satisfies ReadonlyArray<{ id: string; keys: ReadonlyArray<CreditMessage> }>

export type CreditTextKey = (typeof CREDIT_TEXT_GROUPS)[number]['keys'][number]
export type CreditTextGroupId = (typeof CREDIT_TEXT_GROUPS)[number]['id']

/**
 * Values the roll passes beyond the ones in the default copy, so custom copy can
 * format them its own way: a French date order, or a duration in "2 h 35".
 */
const EXTRA_PLACEHOLDERS: Partial<Record<CreditTextKey, readonly string[]>> = {
  session: ['day', 'month', 'year'],
  duration: ['hours', 'minutes'],
}

/** The placeholders a message's custom text can use, e.g. ["date", "day", ...]. */
export function creditTextPlaceholders(key: CreditTextKey): string[] {
  const own = [...viewerOverlay.credits[key].matchAll(PLACEHOLDER)].map((m) => m[1])
  return [...own, ...(EXTRA_PLACEHOLDERS[key] ?? [])]
}

function fill(template: string, params?: Record<string, string | number>): string {
  return template.replace(PLACEHOLDER, (placeholder, name: string) => {
    const value = params?.[name]
    return value === undefined ? placeholder : String(value)
  })
}

/**
 * A translator that answers credit roll messages with the streamer's text where
 * there is one, and with the catalog everywhere else. Returning a TFunction keeps
 * every call site (creditRollValue included) unaware of customization.
 */
export function withCreditTexts(t: TFunction, texts?: Record<string, string> | null): TFunction {
  if (!texts || Object.keys(texts).length === 0) return t
  return (key, params) => {
    const custom = key.startsWith(PREFIX) ? texts[key.slice(PREFIX.length)] : undefined
    return custom?.trim() ? fill(custom, params) : t(key, params)
  }
}

/**
 * Roll elements with their own text size, and each one's default size in rem.
 * The defaults are the roll's Tailwind classes (text-6xl is 3.75rem, ...), which
 * stay on the elements because published credit roll themes select on them.
 */
export const CREDIT_FONT_ELEMENTS = [
  { id: 'heading', baseRem: 3.75 },
  { id: 'subheading', baseRem: 1.5 },
  { id: 'session', baseRem: 1 },
  { id: 'category', baseRem: 2.25 },
  { id: 'entry', baseRem: 1.25 },
  { id: 'footer', baseRem: 2.25 },
  { id: 'nowPlaying', baseRem: 1 },
] as const

export type CreditFontElement = (typeof CREDIT_FONT_ELEMENTS)[number]['id']

export const FONT_SCALE_MIN = 50
export const FONT_SCALE_MAX = 300
export const FONT_SCALE_DEFAULT = 100

const BASE_REM = Object.fromEntries(CREDIT_FONT_ELEMENTS.map((e) => [e.id, e.baseRem])) as Record<
  CreditFontElement,
  number
>

/**
 * A CSS font size for `element`, `ratio` times its default size, or undefined
 * while the element is at its default size: then the element's class (and any
 * custom theme CSS) decides, exactly as before sizes were configurable.
 * Sub-parts of an element (the rank beside a name, the line under a heading)
 * pass their own ratio, so the whole element grows together.
 */
export function creditFontSize(
  scales: Record<string, number> | null | undefined,
  element: CreditFontElement,
  ratio = 1
): string | undefined {
  const pct = scales?.[element]
  if (pct === undefined || pct === FONT_SCALE_DEFAULT) return undefined
  return `${(BASE_REM[element] * ratio * pct) / 100}rem`
}

/** The editor's preview → credit roll message carrying the unsaved config. */
export const CREDIT_ROLL_PREVIEW_MESSAGE = 'CREDIT_ROLL_PREVIEW_UPDATE'

const sampleEntry = (
  rank: number,
  display_name: string,
  platform: string,
  total_value: number,
  currency: string
) => ({
  rank,
  user_id: `sample-${display_name}`,
  display_name,
  avatar_url: '',
  platform,
  total_value,
  metadata: { currency },
})

/**
 * Which event toggles show each leaderboard. Mirrors aggregateLeaderboards in
 * overlay-manager/creditroll/handler.go, so the preview drops exactly the
 * boards the live roll would.
 */
const LEADERBOARD_TOGGLES: Record<keyof Leaderboards, ReadonlyArray<keyof CreditRollConfig>> = {
  subs: ['include_subs', 'include_resubs'],
  gifts: ['include_gift_subs'],
  bits: ['include_bits'],
  raids: ['include_raids'],
  super_chats: ['include_super_chats'],
  memberships: ['include_memberships'],
  follows: ['include_follows'],
  points: ['include_channel_points'],
  treasure_chests: ['include_treasure_chests'],
  shares: ['include_shares'],
  likes: ['include_likes'],
}

// One sample board per event toggle, so switching any of them visibly adds or
// removes a board in the preview.
const SAMPLE_LEADERBOARDS: Required<Leaderboards> = {
  subs: [
    sampleEntry(1, 'NightOwl', 'twitch', 12, 'months'),
    sampleEntry(2, 'PixelPilot', 'twitch', 1, 'months'),
  ],
  gifts: [
    sampleEntry(1, 'GiftGoblin', 'twitch', 25, 'gifts'),
    sampleEntry(2, 'RoseRider', 'tiktok', 1500, 'diamonds'),
    sampleEntry(3, 'LuckyLlama', 'tiktok', 99, 'diamonds'),
  ],
  bits: [sampleEntry(1, 'CheerChamp', 'twitch', 5000, 'bits')],
  points: [sampleEntry(1, 'PointHoarder', 'twitch', 12000, 'points')],
  raids: [sampleEntry(1, 'RaidLeader', 'twitch', 85, 'viewers')],
  super_chats: [sampleEntry(1, 'TubeFan', 'youtube', 20, 'EUR')],
  follows: [
    sampleEntry(1, 'NewFriend', 'twitch', 0, ''),
    sampleEntry(2, 'HelloThere', 'kick', 0, ''),
  ],
  memberships: [sampleEntry(1, 'LoyalMember', 'youtube', 0, '')],
  treasure_chests: [sampleEntry(1, 'ChestOpener', 'tiktok', 500, 'coins')],
  shares: [sampleEntry(1, 'ShareBear', 'tiktok', 7, 'share')],
  likes: [sampleEntry(1, 'TapTapTap', 'tiktok', 2400, 'likes')],
}

/**
 * What the editor preview rolls: sample supporters on the boards the config's
 * event toggles enable. The preview must show a roll whether or not the
 * streamer has a session, and must not fetch the real roll, which counts as a
 * display.
 */
export function sampleCreditRoll(config: Partial<CreditRollConfig>): CreditRollResponse {
  const leaderboards: Leaderboards = {}
  for (const board of Object.keys(SAMPLE_LEADERBOARDS) as Array<keyof Leaderboards>) {
    if (LEADERBOARD_TOGGLES[board].some((toggle) => config[toggle] === true)) {
      leaderboards[board] = SAMPLE_LEADERBOARDS[board]
    }
  }
  return {
    overlay_id: 'preview',
    session_id: 'preview',
    session_started_at: '2026-10-17T18:00:00Z',
    session_duration_seconds: 2 * 3600 + 35 * 60,
    leaderboards,
    clips: [],
    clips_is_fallback: false,
  }
}
