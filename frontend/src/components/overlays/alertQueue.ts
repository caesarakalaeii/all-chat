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
 * The alert-box queue (ADR-0068): alerts arrive as `type: "alert"` WebSocket
 * frames and play one at a time. The waiting list is capped (default 30) and
 * drops the OLDEST arrival on overflow, so a raid-sized burst never delays the
 * newest alert indefinitely. Delivery is at-least-once (a reconnect can
 * redeliver), so alerts are deduped by alert_id.
 */

import { makeSeenIdCache } from '@/core/overlayStreamCore'
import type { AlertFrame } from '@/lib/types/alert'

export type { AlertFrame, AlertProfile } from '@/lib/types/alert'

export const ALERT_QUEUE_CAPACITY = 30
const DEFAULT_DURATION_MS = 5000
// Remembers far more ids than the queue holds: a redelivery can arrive long
// after the original played.
const SEEN_ALERT_IDS = 1024

const sortAfter = (a: AlertFrame) => a.profile?.sort_after ?? 0

export class AlertQueue {
  private waiting: Array<{ alert: AlertFrame; seq: number }> = []
  private seq = 0
  private seen = makeSeenIdCache(SEEN_ALERT_IDS)

  constructor(private readonly capacity = ALERT_QUEUE_CAPACITY) {}

  /** Queue an alert; false when its alert_id was already seen. */
  push(alert: AlertFrame): boolean {
    if (this.seen.markSeen(alert.alert_id)) return false
    const entry = { alert, seq: this.seq++ }
    const at = this.waiting.findIndex((e) => sortAfter(e.alert) > sortAfter(alert))
    if (at < 0) this.waiting.push(entry)
    else this.waiting.splice(at, 0, entry)
    if (this.waiting.length > this.capacity) {
      // drop-oldest: the earliest arrival, which sort_after may have moved.
      let oldest = 0
      this.waiting.forEach((e, i) => {
        if (e.seq < this.waiting[oldest].seq) oldest = i
      })
      this.waiting.splice(oldest, 1)
    }
    return true
  }

  shift(): AlertFrame | undefined {
    return this.waiting.shift()?.alert
  }

  get size(): number {
    return this.waiting.length
  }
}

/** How long an alert stays up (a sound that runs longer extends it). */
export function alertDurationMs(alert: AlertFrame): number {
  if (alert.profile?.duration_ms && alert.profile.duration_ms > 0) return alert.profile.duration_ms
  if (alert.event_data.duration && alert.event_data.duration > 0) {
    return alert.event_data.duration * 1000
  }
  return DEFAULT_DURATION_MS
}

/** The on-stream text: the profile's rendered text, else the sender and value. */
export function alertText(alert: AlertFrame): string {
  if (alert.profile?.text) return alert.profile.text
  return [alert.user.name, alert.event_data.value?.display_text].filter(Boolean).join(' · ')
}
