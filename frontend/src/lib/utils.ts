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

import { clsx, type ClassValue } from 'clsx'
import { twMerge } from 'tailwind-merge'

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

/**
 * Formats an elapsed duration (in milliseconds) as a compact "how long"
 * string suitable for dense admin tables: "3d 4h", "5h 12m", "8m", "just now".
 * Shows the two most significant non-zero units and never renders seconds
 * beyond the sub-minute "just now" case. Negative inputs are treated as zero.
 */
export function formatCompactDuration(ms: number): string {
  if (!Number.isFinite(ms) || ms < 60_000) return 'just now'

  const totalMinutes = Math.floor(ms / 60_000)
  const days = Math.floor(totalMinutes / 1440)
  const hours = Math.floor((totalMinutes % 1440) / 60)
  const minutes = totalMinutes % 60

  if (days > 0) return hours > 0 ? `${days}d ${hours}h` : `${days}d`
  if (hours > 0) return minutes > 0 ? `${hours}h ${minutes}m` : `${hours}h`
  return `${minutes}m`
}

/**
 * Formats "connected for X" from an ISO/RFC3339 start timestamp relative to
 * `now` (defaults to Date.now()). Returns null for missing or unparseable
 * input so callers can omit the label entirely.
 */
export function formatConnectedFor(
  startedAt: string | null | undefined,
  now: number = Date.now()
): string | null {
  if (!startedAt) return null
  const started = Date.parse(startedAt)
  if (Number.isNaN(started)) return null
  return formatCompactDuration(now - started)
}
