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

import { describe, expect, it } from 'vitest'

import {
  ALERT_QUEUE_CAPACITY,
  AlertQueue,
  alertDurationMs,
  alertText,
  type AlertFrame,
} from '@/components/overlays/alertQueue'

function alert(id: string, sortAfter = 0, extra: Partial<AlertFrame> = {}): AlertFrame {
  return {
    alert_id: id,
    overlay_id: 'o',
    platform: 'twitch',
    event_type: 'bits',
    event_data: {
      type: 'bits',
      value: { amount: 100, currency: 'bits', display_text: '100 bits' },
      duration: 8,
    },
    user: { id: 'u', name: 'Viewer', avatar_url: '' },
    profile: {
      event_type: 'bits',
      text_template: '{username} cheered {amount}!',
      sound_url: '',
      image_url: '',
      duration_ms: 5000,
      volume: 100,
      enabled: true,
      sort_after: sortAfter,
      text: `alert ${id}`,
    },
    ...extra,
  }
}

const drain = (q: AlertQueue) => {
  const out: string[] = []
  for (let a = q.shift(); a; a = q.shift()) out.push(a.alert_id)
  return out
}

describe('AlertQueue', () => {
  it('plays alerts in arrival order', () => {
    const q = new AlertQueue()
    ;['a', 'b', 'c'].forEach((id) => q.push(alert(id)))
    expect(drain(q)).toEqual(['a', 'b', 'c'])
  })

  it('drops a redelivered alert_id', () => {
    const q = new AlertQueue()
    expect(q.push(alert('a'))).toBe(true)
    expect(q.push(alert('a'))).toBe(false)
    q.shift()
    // Still a duplicate after it played: a reconnect can redeliver it later.
    expect(q.push(alert('a'))).toBe(false)
    expect(q.size).toBe(0)
  })

  it(`caps at ${ALERT_QUEUE_CAPACITY} waiting alerts, dropping the oldest`, () => {
    const q = new AlertQueue()
    for (let i = 0; i <= ALERT_QUEUE_CAPACITY; i++) q.push(alert(`a${i}`))
    expect(q.size).toBe(ALERT_QUEUE_CAPACITY)
    const ids = drain(q)
    expect(ids[0]).toBe('a1')
    expect(ids[ids.length - 1]).toBe(`a${ALERT_QUEUE_CAPACITY}`)
  })

  it('drops the oldest by arrival even when sort_after reordered it', () => {
    const q = new AlertQueue(2)
    q.push(alert('late', 5))
    q.push(alert('early', 0)) // sorts ahead of "late"
    q.push(alert('third', 0))
    expect(drain(q)).toEqual(['early', 'third'])
  })

  it('orders waiting alerts by sort_after, FIFO within equal values', () => {
    const q = new AlertQueue()
    q.push(alert('raid', 10))
    q.push(alert('sub1', 0))
    q.push(alert('bits', 5))
    q.push(alert('sub2', 0))
    expect(drain(q)).toEqual(['sub1', 'sub2', 'bits', 'raid'])
  })
})

describe('alert helpers', () => {
  it('uses the profile duration, else the event duration, else 5s', () => {
    expect(alertDurationMs(alert('a'))).toBe(5000)
    expect(alertDurationMs(alert('a', 0, { profile: undefined }))).toBe(8000)
    expect(
      alertDurationMs(alert('a', 0, { profile: undefined, event_data: { type: 'bits' } }))
    ).toBe(5000)
  })

  it('renders the server-substituted text, else name and value', () => {
    expect(alertText(alert('a'))).toBe('alert a')
    expect(alertText(alert('a', 0, { profile: undefined }))).toBe('Viewer · 100 bits')
  })
})
