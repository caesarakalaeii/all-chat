/**
 * This file is part of All-Chat.
 * Copyright (C) 2026 caesarakalaeii
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published
 * by the Free Software Foundation, either version 3 of the License, or
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

'use client'

import { useCallback, useEffect, useRef, useState } from 'react'

interface Loaded<T> {
  url: string
  items: T[]
  total: number
  response: Record<string, unknown>
}

export interface PagedList<T> {
  /** Every row loaded so far. While a new url's first page loads, the previous url's rows stay so the list does not flash empty. */
  items: T[]
  /** How many rows match in total, across all pages. */
  total: number
  /** The latest page's whole response body, for extras such as per-tab counts. */
  response: Record<string, unknown> | undefined
  /** The current url's first page is in flight. */
  loading: boolean
  loadingMore: boolean
  error: boolean
  hasMore: boolean
  loadMore: () => void
  /** Refetch the first page, replacing what is loaded (after a mutation). */
  reload: () => void
}

async function fetchPage(
  url: string,
  offset: number,
  pageSize: number
): Promise<Record<string, unknown>> {
  const sep = url.includes('?') ? '&' : '?'
  // Same-origin cookie auth: the gateway turns the httpOnly session cookie
  // into a bearer token, so there is no JS-readable token to send.
  const res = await fetch(`${url}${sep}limit=${pageSize}&offset=${offset}`, {
    credentials: 'same-origin',
  })
  if (!res.ok) throw new Error(`HTTP ${res.status}`)
  return res.json()
}

/**
 * Pages through an admin list endpoint that takes limit/offset and answers
 * `{ [itemsKey]: T[], total }`. `url` carries the filters; a new url starts
 * over from the first page, and null loads nothing. Pair with <LoadMore> for
 * endless scrolling.
 */
export function usePagedList<T>(
  url: string | null,
  itemsKey: string,
  // The admin list endpoints cap a page at 100.
  pageSize = 50
): PagedList<T> {
  const [loaded, setLoaded] = useState<Loaded<T> | null>(null)
  const [failedUrl, setFailedUrl] = useState<string | null>(null)
  // The offset of the next page while it is in flight.
  const [moreOffset, setMoreOffset] = useState<number | null>(null)
  // Read only in callbacks: the url to keep responses for, and the page in
  // flight (a ref, so two loadMore calls in one tick cannot both fetch).
  const latestUrl = useRef(url)
  const inFlight = useRef<number | null>(null)

  // Loads the page at offset and folds it into the list. State is only set
  // in the promise callbacks, never synchronously.
  const load = useCallback(
    (offset: number) => {
      if (url === null) return
      fetchPage(url, offset, pageSize)
        .then((body) => {
          if (url !== latestUrl.current) return
          const page = (body[itemsKey] as T[] | undefined) ?? []
          const total = Number(body.total) || 0
          setLoaded((prev) => {
            if (offset === 0) return { url, items: page, total, response: body }
            // Append only onto the list this page continues; a reload in the
            // meantime has replaced it, and this page would leave a gap.
            if (prev?.url !== url || prev.items.length !== offset) return prev
            return { url, items: [...prev.items, ...page], total, response: body }
          })
          setFailedUrl(null)
        })
        .catch((err) => {
          console.error(`Failed to load ${url}:`, err)
          if (url === latestUrl.current) setFailedUrl(url)
        })
        .finally(() => {
          if (inFlight.current === offset) inFlight.current = null
          setMoreOffset((o) => (o === offset ? null : o))
        })
    },
    [url, itemsKey, pageSize]
  )

  const reload = useCallback(() => load(0), [load])

  useEffect(() => {
    latestUrl.current = url
    reload()
  }, [url, reload])

  const current = loaded?.url === url ? loaded : null
  const error = url !== null && failedUrl === url
  const hasMore = current !== null && current.items.length < current.total
  const loadingMore = current !== null && moreOffset === current.items.length

  const loadMore = useCallback(() => {
    if (current === null || !hasMore || inFlight.current === current.items.length) return
    inFlight.current = current.items.length
    setMoreOffset(current.items.length)
    load(current.items.length)
  }, [current, hasMore, load])

  return {
    items: loaded?.items ?? [],
    total: current?.total ?? 0,
    response: loaded?.response,
    loading: url !== null && current === null && !error,
    loadingMore,
    error,
    hasMore,
    loadMore,
    reload,
  }
}
