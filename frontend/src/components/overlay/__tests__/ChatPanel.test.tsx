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

import { ChatPanel } from '@/components/overlay/ChatPanel'
import type { ViewItem } from '@/lib/utils/overlayViewModel'

afterEach(() => cleanup())

function item(id: string, text: string, mod?: ViewItem['_moderated']): ViewItem {
  return {
    id,
    overlay_id: 'o1',
    platform: 'twitch',
    channel_id: 'c1',
    channel_name: 'chan',
    user: { id: `u-${id}`, username: text + 'er', display_name: text + 'er', badges: [] },
    message: { text, emotes: [] },
    timestamp: '2026-05-31T10:00:00.000Z',
    metadata: {},
    _moderated: mod,
  }
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
