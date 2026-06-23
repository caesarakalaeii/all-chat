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

// @vitest-environment jsdom
import '@testing-library/jest-dom/vitest'
import { render, screen, cleanup } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'

import { ActivityPanel } from '@/components/overlay/ActivityPanel'
import type { EventType } from '@/lib/types/message'
import type { ModEntry, ViewItem } from '@/lib/utils/overlayViewModel'

afterEach(() => cleanup())

function eventItem(id: string, type: EventType, ts: string): ViewItem {
  return {
    id,
    overlay_id: 'o1',
    platform: 'twitch',
    channel_id: 'c1',
    channel_name: 'chan',
    user: { id: 'u1', username: 'sub', display_name: 'Subby', badges: [] },
    message: { text: '', emotes: [] },
    timestamp: ts,
    metadata: {},
    event: { type, tier: 'medium', duration: 0, is_update: false, metadata: {} },
  }
}

describe('ActivityPanel', () => {
  it('shows an empty state when nothing has happened', () => {
    render(<ActivityPanel events={[]} system={[]} moderationLog={[]} />)
    expect(screen.getByText('No events yet.')).toBeInTheDocument()
  })

  it('renders audience events and moderation actions', () => {
    const events = [eventItem('e1', 'subscription', '2026-05-31T10:00:00.000Z')]
    const mods: ModEntry[] = [
      {
        id: 1,
        kind: 'timeout',
        username: 'spammer',
        banDuration: 600,
        source: 'live',
        at: Date.now(),
      },
    ]
    render(<ActivityPanel events={events} system={[]} moderationLog={mods} />)
    expect(screen.getByText('Subscription')).toBeInTheDocument()
    expect(screen.getByText('Subby')).toBeInTheDocument()
    expect(screen.getByText(/Timed out spammer for 600s/)).toBeInTheDocument()
  })

  it('orders the feed newest-first', () => {
    const events = [eventItem('e1', 'subscription', '2026-05-31T10:00:00.000Z')] // older
    const mods: ModEntry[] = [
      { id: 1, kind: 'clear', source: 'live', at: Date.parse('2026-05-31T10:05:00.000Z') }, // newer
    ]
    const { container } = render(<ActivityPanel events={events} system={[]} moderationLog={mods} />)
    const text = container.textContent ?? ''
    expect(text.indexOf('Chat cleared')).toBeGreaterThanOrEqual(0)
    expect(text.indexOf('Chat cleared')).toBeLessThan(text.indexOf('Subscription'))
  })

  it('renders system notices from the system bucket', () => {
    const system = [eventItem('s1', 'source_permission_error', '2026-05-31T10:01:00.000Z')]
    render(<ActivityPanel events={[]} system={system} moderationLog={[]} />)
    expect(screen.getByText('Permission Error')).toBeInTheDocument()
  })

  it('renders the IRC listener deprecation migration notice', () => {
    const item = eventItem('s2', 'listener_deprecation_notice', '2026-05-31T10:02:00.000Z')
    item.event!.metadata = {
      platform: 'twitch',
      channel_id: 'xqc',
      description:
        'The legacy Twitch chat connection is being retired. Re-add your Twitch source to keep chat working.',
      action_url: '/dashboard',
    }
    render(<ActivityPanel events={[]} system={[item]} moderationLog={[]} />)
    expect(screen.getByText('Action Needed')).toBeInTheDocument()
    // The body must render — system notices have empty message.text, so the
    // description from event.metadata is the only body. Without it the notice
    // fires with no body and viewers can't tell what's happening.
    expect(
      screen.getByText(
        'The legacy Twitch chat connection is being retired. Re-add your Twitch source to keep chat working.'
      )
    ).toBeInTheDocument()
    expect(screen.getByText('→ /dashboard')).toBeInTheDocument()
  })
})
