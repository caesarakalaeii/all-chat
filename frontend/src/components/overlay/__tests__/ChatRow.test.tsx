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
import { afterEach, describe, expect, it, vi } from 'vitest'

import { ChatRow } from '@/components/overlay/ChatRow'
import { DEFAULT_VIEW_PREFS, type MonitorViewPrefs } from '@/app/overlay/[id]/view/viewPrefs'
import type { ViewItem } from '@/lib/utils/overlayViewModel'

// jsdom doesn't implement SVG geometry; the AllChatBadge -> InfinityLogo
// animation calls getTotalLength() on mount. Force a stub so rendering succeeds.
;(SVGElement.prototype as unknown as { getTotalLength: () => number }).getTotalLength = () => 0

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

function makeItem(): ViewItem {
  return {
    id: 'm1',
    overlay_id: 'o1',
    platform: 'twitch',
    channel_id: 'c1',
    channel_name: 'chan',
    user: {
      id: 'u1',
      username: 'alice',
      display_name: 'Alice',
      badges: [{ name: 'allchat', version: '1', icon_url: '' }],
      pronouns: 'she/her',
    },
    message: { text: 'hello world', emotes: [] },
    timestamp: '2026-05-31T10:00:00.000Z',
    metadata: {},
  }
}

function prefs(overrides: Partial<MonitorViewPrefs>): MonitorViewPrefs {
  return { ...DEFAULT_VIEW_PREFS, ...overrides }
}

describe('ChatRow pref gating', () => {
  it('shows all signals with default (all-on) prefs', () => {
    const { container } = render(<ChatRow item={makeItem()} />)
    expect(screen.getByLabelText('Twitch')).toBeInTheDocument() // platform glyph
    expect(screen.getByLabelText('Alice')).toBeInTheDocument() // avatar (initials fallback)
    expect(screen.getByLabelText('All-Chat badge')).toBeInTheDocument() // badge
    expect(screen.getByText('she/her')).toBeInTheDocument() // pronoun pill
    expect(container.querySelector('.font-mono')).not.toBeNull() // timestamp
  })

  it('hides the platform glyph when showPlatformGlyph is off', () => {
    render(<ChatRow item={makeItem()} prefs={prefs({ showPlatformGlyph: false })} />)
    expect(screen.queryByLabelText('Twitch')).not.toBeInTheDocument()
  })

  it('hides the avatar when showAvatars is off', () => {
    render(<ChatRow item={makeItem()} prefs={prefs({ showAvatars: false })} />)
    expect(screen.queryByLabelText('Alice')).not.toBeInTheDocument()
  })

  it('hides badges when showBadges is off', () => {
    render(<ChatRow item={makeItem()} prefs={prefs({ showBadges: false })} />)
    expect(screen.queryByLabelText('All-Chat badge')).not.toBeInTheDocument()
  })

  it('hides the pronoun pill when showPronouns is off', () => {
    render(<ChatRow item={makeItem()} prefs={prefs({ showPronouns: false })} />)
    expect(screen.queryByText('she/her')).not.toBeInTheDocument()
  })

  it('hides the timestamp when showTimestamps is off', () => {
    const { container } = render(
      <ChatRow item={makeItem()} prefs={prefs({ showTimestamps: false })} />
    )
    expect(container.querySelector('.font-mono')).toBeNull()
  })

  it('renders no moderation controls when no moderation wiring is provided', () => {
    render(<ChatRow item={makeItem()} />)
    expect(screen.queryByLabelText('Delete message')).not.toBeInTheDocument()
  })

  it('hides moderation controls when showModeration is off, even with wiring', () => {
    const noop = () => {}
    render(
      <ChatRow
        item={makeItem()}
        prefs={prefs({ showModeration: false })}
        moderation={{
          onDelete: noop,
          onTimeout: noop,
          onBan: noop,
          onUnban: noop,
          capability: {
            platform: 'twitch',
            channel_id: 'c1',
            channel_name: 'chan',
            moderatable: true,
            actions: ['delete', 'timeout', 'ban', 'unban'],
          },
        }}
      />
    )
    expect(screen.queryByLabelText('Delete message')).not.toBeInTheDocument()
  })
})

describe('ChatRow username click', () => {
  it('renders the username as a button and reports the clicked item', () => {
    const onUserClick = vi.fn()
    const item = makeItem()
    render(<ChatRow item={item} onUserClick={onUserClick} />)
    fireEvent.click(screen.getByRole('button', { name: 'Alice' }))
    expect(onUserClick).toHaveBeenCalledWith(item)
  })

  it('renders the username as plain text without an onUserClick handler', () => {
    render(<ChatRow item={makeItem()} />)
    expect(screen.getByText('Alice')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Alice' })).not.toBeInTheDocument()
  })
})
