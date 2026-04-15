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

import type { Badge, ChatMessage } from '@/lib/types/message'

type BadgeGroup = 'role' | 'subscriber' | 'other'

const BADGE_GROUP_ORDER: Record<BadgeGroup, number> = {
  role: 0,
  subscriber: 1,
  other: 2,
}

const ROLE_PRIORITIES: Record<string, number> = {
  allchat: -2,  // Phase 31: sorts before all platform role badges
  'allchat-premium': -1,  // Phase 31: sorts before moderator/vip/broadcaster
  moderator: 0,
  mod: 0,
  vip: 1,
  broadcaster: 2,
  streamer: 2,
  owner: 2,
}

const SUB_PRIORITIES: Record<string, number> = {
  founder: 0,
  subscriber: 1,
  premium: 2,
  turbo: 3,
  prime: 4,
  primegaming: 4,
  'prime-gaming': 4,
  subgifter: 5,
  'sub-gifter': 5,
  sub_gifter: 5,
  bits: 6,
}

function normalizeName(name: string | undefined): string {
  return (name ?? '').toLowerCase()
}

function getBadgeGroup(badge: Badge): BadgeGroup {
  const name = normalizeName(badge.name)
  if (name in ROLE_PRIORITIES) {
    return 'role'
  }
  if (name in SUB_PRIORITIES) {
    return 'subscriber'
  }
  return 'other'
}

function getInGroupRank(badge: Badge): number {
  const name = normalizeName(badge.name)

  switch (getBadgeGroup(badge)) {
    case 'role':
      return ROLE_PRIORITIES[name] ?? Number.MAX_SAFE_INTEGER
    case 'subscriber':
      return SUB_PRIORITIES[name] ?? Number.MAX_SAFE_INTEGER
    default:
      return Number.MAX_SAFE_INTEGER
  }
}

export function sortBadges<T extends Badge>(badges: T[]): T[] {
  if (!badges.length) {
    return badges
  }

  return [...badges]
    .map((badge, index) => ({
      badge,
      index,
      group: getBadgeGroup(badge),
      rank: getInGroupRank(badge),
    }))
    .sort((a, b) => {
      if (a.group !== b.group) {
        return BADGE_GROUP_ORDER[a.group] - BADGE_GROUP_ORDER[b.group]
      }

      if (a.rank !== b.rank) {
        return a.rank - b.rank
      }

      return a.index - b.index
    })
    .map((entry) => entry.badge)
}

export function sortMessageBadges(message: ChatMessage): ChatMessage {
  if (!message.user?.badges?.length) {
    return message
  }

  return {
    ...message,
    user: {
      ...message.user,
      badges: sortBadges(message.user.badges),
    },
  }
}
