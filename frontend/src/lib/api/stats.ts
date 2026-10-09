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

import { getApiUrl } from './client'

/** The public GET /api/v1/stats payload the landing counters read. */
export interface LandingStats {
  platforms: Record<string, number>
  all_time: number
  users: number
  overlays_live: number
}

function isLandingStats(value: unknown): value is LandingStats {
  if (!value || typeof value !== 'object') return false
  const stats = value as Record<string, unknown>
  return (
    typeof stats.all_time === 'number' &&
    typeof stats.users === 'number' &&
    typeof stats.overlays_live === 'number' &&
    !!stats.platforms &&
    typeof stats.platforms === 'object'
  )
}

/**
 * Server-side read for the homepage, so the first HTML (what crawlers and LLM
 * fetchers see) already carries the numbers instead of empty counters.
 *
 * The stats are decorative, so a slow or unreachable gateway must not hold up
 * or break the page: anything but a timely 2xx yields null and the client's
 * own fetch on mount fills the counters in as before.
 */
export async function fetchLandingStats(): Promise<LandingStats | null> {
  try {
    const res = await fetch(`${getApiUrl()}/api/v1/stats`, {
      next: { revalidate: 300 },
      signal: AbortSignal.timeout(2000),
    })
    if (!res.ok) return null
    const data: unknown = await res.json()
    return isLandingStats(data) ? data : null
  } catch {
    return null
  }
}
