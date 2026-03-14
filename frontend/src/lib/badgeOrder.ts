import type { Badge, ChatMessage } from '@/lib/types/message'

type BadgeGroup = 'role' | 'subscriber' | 'other'

const BADGE_GROUP_ORDER: Record<BadgeGroup, number> = {
  role: 0,
  subscriber: 1,
  other: 2,
}

const ROLE_PRIORITIES: Record<string, number> = {
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
