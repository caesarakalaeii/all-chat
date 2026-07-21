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
import { render, screen, cleanup, fireEvent } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'

import { ChatPanel } from '@/components/overlay/ChatPanel'
import type { ViewItem } from '@/lib/utils/overlayViewModel'

// jsdom may lack window.matchMedia; useReducedMotion (mounted via
// MessageAttachments in every row) calls it. Static non-matching stub.
if (typeof window.matchMedia !== 'function') {
  window.matchMedia = (query: string) =>
    ({
      matches: false,
      media: query,
      addEventListener: () => {},
      removeEventListener: () => {},
    }) as unknown as MediaQueryList
}

afterEach(() => cleanup())

function item(
  id: string,
  text: string,
  mod?: ViewItem['_moderated'],
  user?: { id: string; username: string }
): ViewItem {
  return {
    id,
    overlay_id: 'o1',
    platform: 'twitch',
    channel_id: 'c1',
    channel_name: 'chan',
    user: user
      ? { ...user, display_name: user.username, badges: [] }
      : { id: `u-${id}`, username: text + 'er', display_name: text + 'er', badges: [] },
    message: { text, emotes: [] },
    timestamp: '2026-05-31T10:00:00.000Z',
    metadata: {},
    _moderated: mod,
  }
}

/** Make the panel's scroll container report a scrolled-up viewport, then scroll. */
function scrollUp(container: HTMLElement) {
  const el = container.querySelector('.overflow-y-auto') as HTMLElement
  Object.defineProperty(el, 'scrollHeight', { value: 1000, configurable: true })
  Object.defineProperty(el, 'clientHeight', { value: 100, configurable: true })
  el.scrollTop = 0
  fireEvent.scroll(el)
}

describe('ChatPanel', () => {
  it('shows an empty state when there are no messages', () => {
    render(<ChatPanel items={[]} />)
    expect(screen.getByText('No chat messages yet.')).toBeInTheDocument()
  })

  it('renders each message and a count', () => {
    render(<ChatPanel items={[item('1', 'hello'), item('2', 'world')]} />)
    expect(screen.getByText('hello')).toBeInTheDocument()
    expect(screen.getByText('world')).toBeInTheDocument()
    expect(screen.getByText('2')).toBeInTheDocument()
  })

  it('marks a moderated message with a tag (kept visible)', () => {
    render(<ChatPanel items={[item('1', 'spam', { kind: 'timeout', banDuration: 600 })]} />)
    expect(screen.getByText('spam')).toBeInTheDocument() // still visible
    expect(screen.getByText(/timed out/i)).toBeInTheDocument()
  })
})

describe('ChatPanel 1:1 user filter', () => {
  const alice = { id: 'ua', username: 'alice' }
  const bob = { id: 'ub', username: 'bob' }

  it('clicking a username shows only that chatter, with a filter bar and count', () => {
    render(
      <ChatPanel
        items={[
          item('1', 'hi there', undefined, alice),
          item('2', 'spam spam', undefined, bob),
          item('3', 'question?', undefined, alice),
        ]}
      />
    )
    fireEvent.click(screen.getAllByRole('button', { name: 'alice' })[0])

    expect(screen.getByText('hi there')).toBeInTheDocument()
    expect(screen.getByText('question?')).toBeInTheDocument()
    expect(screen.queryByText('spam spam')).not.toBeInTheDocument()
    expect(screen.getByText(/Showing only messages from/)).toBeInTheDocument()
    expect(screen.getByText('2 of 3')).toBeInTheDocument()
  })

  it('keeps following new messages from the filtered chatter', () => {
    const { rerender } = render(<ChatPanel items={[item('1', 'hi there', undefined, alice)]} />)
    fireEvent.click(screen.getByRole('button', { name: 'alice' }))
    rerender(
      <ChatPanel
        items={[
          item('1', 'hi there', undefined, alice),
          item('2', 'noise', undefined, bob),
          item('3', 'follow-up', undefined, alice),
        ]}
      />
    )
    expect(screen.getByText('follow-up')).toBeInTheDocument()
    expect(screen.queryByText('noise')).not.toBeInTheDocument()
  })

  it('"Show all chat" clears the filter; clicking the same name again also clears it', () => {
    render(
      <ChatPanel
        items={[item('1', 'hi there', undefined, alice), item('2', 'spam spam', undefined, bob)]}
      />
    )
    fireEvent.click(screen.getByRole('button', { name: 'alice' }))
    expect(screen.queryByText('spam spam')).not.toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: /Show all chat/ }))
    expect(screen.getByText('spam spam')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'alice' }))
    expect(screen.queryByText('spam spam')).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'alice' }))
    expect(screen.getByText('spam spam')).toBeInTheDocument()
  })

  it('shows a filter-specific empty state when the chatter has no messages left', () => {
    const { rerender } = render(<ChatPanel items={[item('1', 'hi there', undefined, alice)]} />)
    fireEvent.click(screen.getByRole('button', { name: 'alice' }))
    rerender(<ChatPanel items={[item('2', 'other', undefined, bob)]} />)
    expect(screen.getByText('No messages from alice yet.')).toBeInTheDocument()
  })
})

describe('ChatPanel pause-on-scroll', () => {
  it('freezes the feed while scrolled up and resumes via the paused pill', () => {
    const first = [item('1', 'hello'), item('2', 'world')]
    const { container, rerender } = render(<ChatPanel items={first} />)

    scrollUp(container)
    expect(screen.getByRole('button', { name: /Chat paused/ })).toBeInTheDocument()

    rerender(<ChatPanel items={[...first, item('3', 'newest')]} />)
    // Frozen: the new message is buffered, not rendered, and the pill counts it.
    expect(screen.queryByText('newest')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Chat paused · 1 new/ })).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: /Chat paused/ }))
    expect(screen.getByText('newest')).toBeInTheDocument()
    expect(screen.queryByText(/Chat paused/)).not.toBeInTheDocument()
  })
})
