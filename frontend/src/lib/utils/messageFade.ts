/**
 * This file is part of All-Chat.
 * Copyright (C) 2026 caesarakalaeii
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
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
 * Message fade-out expiry, as pure arithmetic on arrival times.
 *
 * The OBS overlay fades each message a fixed number of seconds after ITS OWN
 * arrival — not after the last state change. The first version of that effect
 * depended on the whole `messages` array and armed one `setTimeout` for the
 * head row, so every append re-armed the timer from the full duration and a
 * busy chat never let anything disappear (#overlays fade bug). Keeping the
 * arithmetic here makes "appends must not extend an older row's lifetime" a
 * directly testable property: expiry is derived from `arrivedAt`, which the
 * overlay stamps once per message id and never overwrites.
 *
 * Arrival times are the CLIENT clock, not the message's server `timestamp`:
 * a reconnect replays the tail, and rows must not expire instantly on arrival.
 */

import type { ChatMessage, EventTier } from '@/lib/types/message'

/** Fallback display duration (seconds) for an event whose `duration` is 0. */
export function getTierDuration(tier: EventTier): number {
  switch (tier) {
    case 'high':
      return 30
    case 'medium':
      return 15
    case 'low':
      return 8
    default:
      return 15
  }
}

/**
 * Absolute expiry (ms epoch) of one row. Events keep their own duration —
 * explicit, or tier default when the event carries 0. A missing arrival time
 * means the row predates the clock (or the stamp raced the commit), so it
 * expires immediately rather than lingering forever.
 */
export function messageExpiry(
  message: ChatMessage,
  arrivedAt: number | undefined,
  chatDurationSeconds: number
): number {
  const duration = message.event
    ? message.event.duration || getTierDuration(message.event.tier)
    : chatDurationSeconds
  return (arrivedAt ?? 0) + duration * 1000
}

/**
 * How long the fade timer should wait before sweeping, or `null` when there is
 * nothing left to wait for. The result is clamped at 0, so overdue rows (a
 * throttled background tab, a just-shrunk duration) sweep on the next tick
 * instead of scheduling a negative delay.
 */
export function nextFadeDelayMs(
  messages: readonly ChatMessage[],
  arrivalTimes: ReadonlyMap<string, number>,
  chatDurationSeconds: number,
  now: number
): number | null {
  let earliest: number | null = null
  for (const message of messages) {
    const expiry = messageExpiry(message, arrivalTimes.get(message.id), chatDurationSeconds)
    if (earliest === null || expiry < earliest) earliest = expiry
  }
  return earliest === null ? null : Math.max(earliest - now, 0)
}
