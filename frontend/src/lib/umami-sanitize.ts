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
 * Umami URL sanitiser
 *
 * Umami's automatic page-view tracking sends the full URL — path, search, and
 * hash. Left untouched that produces two problems:
 *
 *   1. High cardinality — every overlay is a unique path (`/overlays/<uuid>`),
 *      so the stats fragment into thousands of one-hit URLs that aren't useful.
 *   2. Secrets and identifiers in analytics — URLs carry bearer tokens
 *      (`/overlay/<id>?tts_token=<jwt>`, `/auth/callback#access_token=...`),
 *      one-time codes (`/chat/auth-success?code=...`), usernames
 *      (`?existing_username=`, `?streamer=`) and admin lookups (`?user=`,
 *      `?overlay=`), and new params like these appear with new features.
 *
 * This module is wired into the tracker via `data-before-send` (see
 * `components/Analytics.tsx`). Umami resolves the callback as `window[name]` at
 * send time and transmits whatever payload we return (a falsy return cancels the
 * event). We rewrite the `url` and `referrer` fields:
 *
 *   - keep only `utm_*` query params (campaign attribution) and drop every
 *     other one. This is an allowlist on purpose: a denylist misses each new
 *     param that carries a token or an identifier, and the miss is silent.
 *   - collapse every UUID anywhere in the result (path or kept query value)
 *     to `:id`
 *   - strip the hash defensively (the tracker also has `data-exclude-hash` set,
 *     which removes it race-free before this even runs)
 */

/** RFC-4122 UUID (overlay/source IDs use `gen_random_uuid()`), case-insensitive, global. */
const UUID_PATTERN = /[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}/gi

/**
 * The only query params a page-view URL keeps: Umami reads `utm_*` for its
 * campaign report. Everything else is dropped; the typed custom events in
 * `lib/analytics.ts` carry the useful, aggregate signal instead.
 */
const KEPT_QUERY_PREFIX = 'utm_'

/**
 * Normalise a tracked URL (or referrer) so it carries no UUIDs, no hash and
 * no query params other than `utm_*`.
 * Accepts relative (`/p?q#h`) and absolute (`https://host/p?q#h`) forms.
 */
export function sanitizeUrl(raw: string): string {
  const noHash = raw.split('#', 1)[0]
  const queryStart = noHash.indexOf('?')
  const path = queryStart === -1 ? noHash : noHash.slice(0, queryStart)
  const search = queryStart === -1 ? '' : noHash.slice(queryStart + 1)

  const kept = search.split('&').filter((pair) => pair.startsWith(KEPT_QUERY_PREFIX))
  const result = kept.length > 0 ? `${path}?${kept.join('&')}` : path
  return result.replace(UUID_PATTERN, ':id')
}

/** Subset of the Umami event payload we touch; other fields pass through unchanged. */
export interface UmamiPayload {
  url?: string
  referrer?: string
  [key: string]: unknown
}

/**
 * `data-before-send` handler. Mutates and returns the payload so Umami transmits
 * the sanitised version. Best-effort and defensive: if normalisation ever throws
 * on unexpected input we fall back to a bare path, so a token can never escape.
 */
export function umamiBeforeSend(_type: string, payload: UmamiPayload): UmamiPayload {
  try {
    if (typeof payload.url === 'string') {
      payload.url = sanitizeUrl(payload.url)
    }
    if (typeof payload.referrer === 'string') {
      payload.referrer = sanitizeUrl(payload.referrer)
    }
  } catch {
    // Last-resort: drop query + hash entirely. A less precise path is an
    // acceptable price for guaranteeing no token is ever transmitted.
    if (typeof payload.url === 'string') payload.url = payload.url.split(/[?#]/)[0]
    if (typeof payload.referrer === 'string') payload.referrer = payload.referrer.split(/[?#]/)[0]
  }
  return payload
}

declare global {
  interface Window {
    __umamiBeforeSend?: (type: string, payload: UmamiPayload) => UmamiPayload
  }
}
