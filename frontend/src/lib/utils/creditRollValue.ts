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

import { type MessageKey, type TFunction, formatCurrency, formatNumber } from '@/lib/i18n'
import type { LeaderboardEntry } from '@/lib/types/overlay'

type UnitKeys = { one: MessageKey; many: MessageKey }

// Keyed by the EventValue currency the message-processor stores with each entry
// (metadata.currency). A leaderboard can mix units: Top Gifters holds Twitch gift
// subs and TikTok diamonds side by side, so the unit is chosen per entry.
const UNIT_KEYS: Record<string, UnitKeys> = {
  bits: { one: 'viewerOverlay.credits.valueBitsOne', many: 'viewerOverlay.credits.valueBitsMany' },
  months: {
    one: 'viewerOverlay.credits.valueMonthsOne',
    many: 'viewerOverlay.credits.valueMonthsMany',
  },
  viewers: {
    one: 'viewerOverlay.credits.valueViewersOne',
    many: 'viewerOverlay.credits.valueViewersMany',
  },
  points: {
    one: 'viewerOverlay.credits.valuePointsOne',
    many: 'viewerOverlay.credits.valuePointsMany',
  },
  gift: {
    one: 'viewerOverlay.credits.valueGiftsOne',
    many: 'viewerOverlay.credits.valueGiftsMany',
  },
  gifts: {
    one: 'viewerOverlay.credits.valueGiftsOne',
    many: 'viewerOverlay.credits.valueGiftsMany',
  },
  diamonds: {
    one: 'viewerOverlay.credits.valueDiamondsOne',
    many: 'viewerOverlay.credits.valueDiamondsMany',
  },
}

const ISO_CURRENCY = /^[A-Z]{3}$/

/**
 * A leaderboard entry's total in its own unit: "500 bits", "3 months", "€12.34",
 * or channel points under the streamer's own name ("5,000 Kappa Coins").
 * Only an ISO currency code renders as money. Empty when there is nothing
 * meaningful to show (no total yet, or a count-only event such as a follow).
 */
export function creditRollValue(t: TFunction, entry: LeaderboardEntry): string {
  const value = entry.total_value ?? 0
  if (value <= 0) return ''
  const currency = typeof entry.metadata?.currency === 'string' ? entry.metadata.currency : ''
  const pointsName = entry.metadata?.points_name
  if (currency === 'points' && typeof pointsName === 'string' && pointsName) {
    return t('viewerOverlay.credits.valuePointsNamed', {
      count: formatNumber(value),
      name: pointsName,
    })
  }
  const unit = UNIT_KEYS[currency]
  if (unit) return t(value === 1 ? unit.one : unit.many, { count: formatNumber(value) })
  if (ISO_CURRENCY.test(currency)) return formatCurrency(value, currency)
  return ''
}
