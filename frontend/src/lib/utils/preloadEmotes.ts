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
 * Browser-side emote pre-caching.
 *
 * Emote images are loaded lazily — the first chat message that uses an emote
 * triggers the CDN fetch, and until it resolves the browser shows the alt/text
 * fallback. This warms the browser image cache ahead of time: on overlay
 * connect we fetch each source channel's emote set (the same `/emotes/channel`
 * endpoint the message-processor enricher uses) and kick off hidden image loads
 * for every emote URL, so by the time a message references one it's already
 * decoded and cached. Complements the backend stale-while-revalidate cache —
 * that keeps server-side emote metadata warm, this keeps the client warm.
 *
 * Best-effort throughout: any network/parse failure is swallowed so it can
 * never affect rendering.
 */

/** Minimal source descriptor: enough to query the channel emote set. */
export interface EmotePreloadSource {
  platform: string
  channelId: string
}

interface ChannelEmoteResponse {
  emotes?: Array<{ url?: string }>
}

export interface PreloadEmotesOptions {
  /** Per-overlay 7TV emote-set override, merged into every channel query so the
   *  preloaded set matches what the enricher will attach to messages. */
  seventvSetId?: string
  /** Cap on the number of distinct image loads kicked off, to avoid hammering
   *  the CDN when an overlay aggregates several large emote sets. */
  maxUrls?: number
  /** Injectable image loader, primarily for tests. Defaults to `new Image()`. */
  loadImage?: (url: string) => void
  signal?: AbortSignal
}

const DEFAULT_MAX_URLS = 400

function defaultLoadImage(url: string): void {
  // A detached Image whose src is set still populates the HTTP cache without
  // ever entering the DOM, so it can't affect layout.
  const img = new Image()
  img.decoding = 'async'
  img.src = url
}

/**
 * Fetch the emote sets for the given source channels and warm the browser image
 * cache by requesting each emote URL. Returns the number of image loads started
 * (deduplicated across channels). Never throws.
 */
export async function preloadOverlayEmotes(
  sources: EmotePreloadSource[],
  options: PreloadEmotesOptions = {}
): Promise<number> {
  if (typeof window === 'undefined') return 0

  const maxUrls = options.maxUrls ?? DEFAULT_MAX_URLS
  const loadImage = options.loadImage ?? defaultLoadImage

  // Deduplicate the channel queries (an overlay can list the same channel twice
  // across platforms, and shared-overlay sources can repeat a channel id).
  const seenChannels = new Set<string>()
  const queries = sources.filter((s) => {
    if (!s.channelId) return false
    const key = `${s.platform}:${s.channelId}`
    if (seenChannels.has(key)) return false
    seenChannels.add(key)
    return true
  })

  const urls = new Set<string>()

  await Promise.all(
    queries.map(async (q) => {
      try {
        const params = new URLSearchParams()
        if (q.platform) params.set('platform', q.platform)
        if (options.seventvSetId) params.set('seventv_set_id', options.seventvSetId)
        const qs = params.toString()
        const res = await fetch(
          `/api/v1/emotes/channel/${encodeURIComponent(q.channelId)}${qs ? `?${qs}` : ''}`,
          { signal: options.signal }
        )
        if (!res.ok) return
        const data = (await res.json()) as ChannelEmoteResponse
        for (const emote of data.emotes ?? []) {
          if (emote.url) urls.add(emote.url)
        }
      } catch {
        // Best-effort: a failed channel fetch just means those emotes load on
        // demand as before.
      }
    })
  )

  let started = 0
  for (const url of urls) {
    if (started >= maxUrls) break
    loadImage(url)
    started++
  }
  return started
}
