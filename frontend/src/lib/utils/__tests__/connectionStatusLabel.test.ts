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

import { describe, it, expect } from 'vitest'

import { connectionStatusDisplay, OFFLINE_THRESHOLD } from '../connectionStatusLabel'

describe('connectionStatusDisplay', () => {
  it('shows a healthy socket as Live', () => {
    expect(connectionStatusDisplay('open')).toEqual({
      label: 'Live',
      dot: 'bg-kick',
      title: 'Live',
    })
  })

  it('distinguishes a first connect from a reconnect', () => {
    expect(connectionStatusDisplay('connecting').label).toBe('Connecting')
    expect(connectionStatusDisplay('reconnecting', 1).label).toBe('Reconnecting')
  })

  it('escalates to red once the failures are sustained', () => {
    expect(connectionStatusDisplay('reconnecting', OFFLINE_THRESHOLD - 1).dot).toBe('bg-amber-400')
    expect(connectionStatusDisplay('reconnecting', OFFLINE_THRESHOLD).dot).toBe('bg-red-500')
    expect(connectionStatusDisplay('reconnecting', OFFLINE_THRESHOLD + 20).dot).toBe('bg-red-500')
  })

  it('keeps the ~13s threshold that separates a blip from a real outage', () => {
    expect(OFFLINE_THRESHOLD).toBe(4)
  })

  // The socket retries forever, and a redeploy routinely outlasts the
  // threshold. Claiming "Offline" told the streamer the link was dead when it
  // was mid-recovery, and the predictable response — reopen the overlay —
  // resets the watermark and causes exactly the loss the badge warned about.
  it('never claims the connection is offline, however long it has been failing', () => {
    for (const attempts of [OFFLINE_THRESHOLD, 10, 100]) {
      const { label, title } = connectionStatusDisplay('reconnecting', attempts)
      expect(label).not.toMatch(/offline/i)
      expect(title).not.toMatch(/offline/i)
    }
  })

  it('says recovery is automatic in the red state', () => {
    expect(connectionStatusDisplay('reconnecting', OFFLINE_THRESHOLD).label).toMatch(
      /reconnecting/i
    )
  })

  it('carries the retry count in the title, not the label', () => {
    const one = connectionStatusDisplay('reconnecting', 1)
    expect(one.title).toBe('Reconnecting — 1 failed attempt')
    expect(one.label).toBe('Reconnecting')

    const many = connectionStatusDisplay('reconnecting', 7)
    expect(many.title).toBe('Reconnecting… — 7 failed attempts')
    expect(many.label).toBe('Reconnecting…')
  })

  it('omits the count from the title when there is nothing to count', () => {
    expect(connectionStatusDisplay('reconnecting', 0).title).toBe('Reconnecting')
    expect(connectionStatusDisplay('connecting', 0).title).toBe('Connecting')
  })

  it('ignores a stale attempt count once the socket is back', () => {
    // attempts is mirrored render state and can lag a frame behind the status.
    expect(connectionStatusDisplay('open', 99)).toEqual({
      label: 'Live',
      dot: 'bg-kick',
      title: 'Live',
    })
  })

  it('defaults attempts to zero', () => {
    expect(connectionStatusDisplay('reconnecting')).toEqual(
      connectionStatusDisplay('reconnecting', 0)
    )
  })
})
