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
 * KLIPY GIF search (https://docs.klipy.com; "Klippy" in issue #954): the free media source
 * of the alerts editor. Called from the browser on purpose — KLIPY's usage
 * guidelines require requests and media loads to come from the user's browser,
 * results shown in the order returned, media URLs used exactly as returned (no
 * re-hosting), and a "Powered by KLIPY" attribution next to the results.
 *
 * The app key is visible to whoever may search (it travels in every request URL). The
 * server reads KLIPY_API_KEY at runtime and hands it out at /klipy-config
 * (ADR-0072), to signed-in premium or beta users only (ADR-0074). Without it
 * search is unavailable and the editor says so; pasting a URL or uploading
 * still works.
 */

const KLIPY_API = 'https://api.klipy.com/api/v1'

export interface KlipyGif {
  id: string
  title: string
  /** Small rendition for the result grid. */
  previewUrl: string
  /** What the profile stores and the overlay plays. */
  url: string
}

interface Rendition {
  gif?: { url?: string }
}
interface KlipyItem {
  id?: number | string
  slug?: string
  title?: string
  file?: Partial<Record<'hd' | 'md' | 'sm' | 'xs', Rendition>>
}
interface KlipySearchResponse {
  result?: boolean
  data?: { data?: KlipyItem[] }
}

let keyPromise: Promise<string> | undefined

/** The runtime app key, fetched once per page load; a failed fetch is retried next time. */
export function klipyApiKey(): Promise<string> {
  keyPromise ??= fetch('/klipy-config', { credentials: 'same-origin' })
    .then(async (res) => {
      if (!res.ok) throw new Error(`klipy: config HTTP ${res.status}`)
      return String(((await res.json()) as { apiKey?: unknown }).apiKey ?? '')
    })
    .catch((err: unknown) => {
      keyPromise = undefined
      throw err
    })
  return keyPromise
}

/**
 * Search GIFs. Throws when search is unavailable (no key, network, non-2xx,
 * or a `result: false` body); callers degrade to "search unavailable".
 */
export async function searchKlipyGifs(
  query: string,
  { apiKey, perPage = 24, signal }: { apiKey?: string; perPage?: number; signal?: AbortSignal } = {}
): Promise<KlipyGif[]> {
  apiKey ??= await klipyApiKey()
  if (!apiKey) throw new Error('klipy: no api key')
  const params = new URLSearchParams({
    q: query,
    page: '1',
    per_page: String(perPage),
    content_filter: 'medium',
    format_filter: 'gif',
  })
  const res = await fetch(`${KLIPY_API}/${encodeURIComponent(apiKey)}/gifs/search?${params}`, {
    signal,
  })
  if (!res.ok) throw new Error(`klipy: HTTP ${res.status}`)
  const body = (await res.json()) as KlipySearchResponse
  if (body.result === false) throw new Error('klipy: request rejected')
  // Order kept as returned (KLIPY guidelines). An item with no GIF rendition
  // (an ad slot, when ads are enabled in the KLIPY partner panel) has nothing
  // this picker can store, so it is not offered.
  // ponytail: ad slots are not rendered; render them if ads get enabled.
  return (body.data?.data ?? []).flatMap((item) => {
    const url = item.file?.hd?.gif?.url ?? item.file?.md?.gif?.url
    const previewUrl = item.file?.sm?.gif?.url ?? item.file?.xs?.gif?.url ?? url
    if (!url || !previewUrl) return []
    return [{ id: String(item.id ?? item.slug ?? url), title: item.title ?? '', previewUrl, url }]
  })
}
