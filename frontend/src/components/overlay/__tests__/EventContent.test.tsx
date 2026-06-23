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

import { EventContent } from '@/components/overlay/EventContent'
import type { ChatMessage, EventType } from '@/lib/types/message'

afterEach(cleanup)

/** Build a minimal event-bearing ChatMessage for the renderer. */
function eventMessage(type: EventType, metadata: Record<string, unknown> = {}): ChatMessage {
  return {
    id: 'evt-1',
    overlay_id: 'ov-1',
    platform: 'system',
    channel_id: 'system',
    channel_name: 'All-Chat System',
    user: {
      id: 'system',
      username: 'system',
      display_name: 'All-Chat System',
      badges: [],
      color: '#EF4444',
    },
    message: { text: '', emotes: [] },
    timestamp: '2026-06-23T00:00:00Z',
    metadata: {},
    event: { type, tier: 'high', duration: 60, is_update: false, metadata },
  }
}

describe('EventContent — system notices', () => {
  it('renders listener_deprecation_notice with its real title, body and CTA (not the generic fallback)', () => {
    render(
      <EventContent
        message={eventMessage('listener_deprecation_notice', {
          description: 'The legacy Twitch chat connection is being retired.',
          action_url: '/dashboard',
        })}
      />
    )
    expect(screen.getByText('Twitch Connection Update')).toBeInTheDocument()
    expect(
      screen.getByText('The legacy Twitch chat connection is being retired.')
    ).toBeInTheDocument()
    expect(
      screen.getByText('→ Re-add your Twitch source to switch to the new EventSub connection')
    ).toBeInTheDocument()
    // Regression guard: must NOT fall through to the generic default title.
    expect(screen.queryByText('Event!')).not.toBeInTheDocument()
  })

  it('renders source_permission_error and token_expiration_warning titles', () => {
    const { rerender } = render(
      <EventContent message={eventMessage('source_permission_error', { channel_id: '123' })} />
    )
    expect(screen.getByText('Bot Missing Channel Permission')).toBeInTheDocument()
    expect(screen.getByText('Channel 123 is not accessible')).toBeInTheDocument()

    rerender(
      <EventContent message={eventMessage('token_expiration_warning', { platform: 'twitch' })} />
    )
    expect(screen.getByText('Twitch Authentication Error')).toBeInTheDocument()
  })

  it('still renders audience event titles', () => {
    render(<EventContent message={eventMessage('subscription')} />)
    expect(screen.getByText('New Subscriber!')).toBeInTheDocument()
  })

  it('uses the generic title only for event types with no dedicated title', () => {
    // `ritual` is a known type with no custom title -> generic fallback path.
    render(<EventContent message={eventMessage('ritual')} />)
    expect(screen.getByText('Event!')).toBeInTheDocument()
  })

  it('joins numeric metadata with separators (e.g. resub months + streak)', () => {
    render(<EventContent message={eventMessage('resubscription', { months: 6, streak: 3 })} />)
    expect(screen.getByText('6 months • 3 month streak')).toBeInTheDocument()
  })
})
