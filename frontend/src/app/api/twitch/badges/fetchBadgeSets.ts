import { unstable_noStore as noStore } from 'next/cache';

type TwitchBadgeVersion = {
  id: string;
  image_url_1x?: string;
  image_url_2x?: string;
  image_url_4x?: string;
};

type TwitchBadgeSet = {
  versions: Record<string, TwitchBadgeVersion>;
};

export type TwitchBadgeResponse = {
  badge_sets: Record<string, TwitchBadgeSet>;
};

type CacheEntry = {
  expires: number;
  data: TwitchBadgeResponse;
};

const TWITCH_BADGE_BASE = 'https://badges.twitch.tv/v1/badges';
export const BADGE_CACHE_TTL_MS = 1000 * 60 * 15; // 15 minutes

const cache = new Map<string, CacheEntry>();

export async function fetchBadgeSets(path: string): Promise<TwitchBadgeResponse> {
  noStore();

  const cacheKey = path;
  const now = Date.now();
  const cached = cache.get(cacheKey);
  if (cached && cached.expires > now) {
    return cached.data;
  }

  const upstreamUrl = `${TWITCH_BADGE_BASE}${path}`;

  const response = await fetch(upstreamUrl, {
    headers: {
      Accept: 'application/json',
    },
    // Twitch badge endpoints already include caching headers but we keep our own TTL as well.
    cache: 'no-store',
  });

  if (!response.ok) {
    throw new Error(`Twitch badge request failed with status ${response.status}`);
  }

  const data = (await response.json()) as TwitchBadgeResponse;

  cache.set(cacheKey, {
    data,
    expires: now + BADGE_CACHE_TTL_MS,
  });

  return data;
}
