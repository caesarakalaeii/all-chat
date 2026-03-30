import type { ChatMessage, Badge } from '@/lib/types/message'

// AllChat-native badge names that must not be resolved against Twitch's badge API.
// These badges are rendered by dedicated React components (AllChatBadge, PremiumBadge)
// and do not have Twitch CDN icon URLs.
const ALLCHAT_NATIVE_BADGE_NAMES = new Set(['allchat', 'allchat-premium'])

type TwitchBadgeVersion = {
  id: string
  image_url_1x?: string
  image_url_2x?: string
  image_url_4x?: string
}

type TwitchBadgeSet = {
  versions: Record<string, TwitchBadgeVersion>
}

type TwitchBadgeResponse = {
  badge_sets: Record<string, TwitchBadgeSet>
}

type BadgeCache = Record<string, Record<string, TwitchBadgeSet>>

const badgeCache: BadgeCache = {}
const inflightRequests: Record<string, Promise<Record<string, TwitchBadgeSet> | null>> = {}

const TWITCH_BADGE_BASE = '/api/twitch/badges'

async function fetchBadgeSets(
  cacheKey: string,
  url: string
): Promise<Record<string, TwitchBadgeSet> | null> {
  if (badgeCache[cacheKey]) {
    return badgeCache[cacheKey]
  }

  if (!inflightRequests[cacheKey]) {
    inflightRequests[cacheKey] = fetch(url)
      .then(async (res) => {
        if (!res.ok) {
          throw new Error(`Request failed with status ${res.status}`)
        }
        const data = (await res.json()) as TwitchBadgeResponse
        return data.badge_sets ?? {}
      })
      .catch((err) => {
        console.error('[Badges] Failed to fetch Twitch badges', { url, err })
        return null
      })
      .finally(() => {
        delete inflightRequests[cacheKey]
      })
  }

  const result = await inflightRequests[cacheKey]
  if (result) {
    badgeCache[cacheKey] = result
  }
  return result
}

async function getGlobalBadgeSets() {
  return fetchBadgeSets('global', `${TWITCH_BADGE_BASE}/global`)
}

async function getChannelBadgeSets(roomId: string | undefined) {
  if (!roomId) {
    return null
  }
  const trimmed = roomId.trim()
  if (!trimmed) {
    return null
  }
  return fetchBadgeSets(`channel:${trimmed}`, `${TWITCH_BADGE_BASE}/channels/${trimmed}`)
}

function resolveBadgeIcon(
  badge: Badge,
  channelBadges?: Record<string, TwitchBadgeSet> | null,
  globalBadges?: Record<string, TwitchBadgeSet> | null
): Badge {
  const resolved =
    channelBadges?.[badge.name]?.versions?.[badge.version] ??
    globalBadges?.[badge.name]?.versions?.[badge.version]

  if (!resolved) {
    return badge
  }

  const iconURL = resolved.image_url_1x || resolved.image_url_2x || resolved.image_url_4x
  if (!iconURL) {
    return badge
  }

  return {
    ...badge,
    icon_url: iconURL,
  }
}

export async function resolveTwitchBadgeIcons(message: ChatMessage): Promise<ChatMessage> {
  if (message.platform !== 'twitch' || !message.user?.badges?.length) {
    return message
  }

  const roomId = (message.metadata?.twitch_room_id as string | undefined) ?? undefined
  const [channelBadges, globalBadges] = await Promise.all([
    getChannelBadgeSets(roomId),
    getGlobalBadgeSets(),
  ])

  if (!channelBadges && !globalBadges) {
    return message
  }

  const updatedBadges = message.user.badges.map((badge) =>
    ALLCHAT_NATIVE_BADGE_NAMES.has(badge.name)
      ? badge
      : resolveBadgeIcon(badge, channelBadges, globalBadges)
  )

  return {
    ...message,
    user: {
      ...message.user,
      badges: updatedBadges,
    },
  }
}
