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
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import React from 'react'
import { render, screen, fireEvent, cleanup } from '@testing-library/react'

afterEach(() => {
  cleanup()
})

// useReducedMotion reads window.matchMedia; jsdom has no implementation.
beforeEach(() => {
  Object.defineProperty(window, 'matchMedia', {
    writable: true,
    value: vi.fn().mockImplementation((query: string) => ({
      matches: false, // motion-on
      media: query,
      onchange: null,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      addListener: vi.fn(),
      removeListener: vi.fn(),
      dispatchEvent: vi.fn(),
    })),
  })
})

import { MessageAttachments } from '../MessageAttachments'
import type { Attachment, ChatMessage } from '@/lib/types/message'

function makeMessage(attachments: Attachment[]): ChatMessage {
  return {
    id: 'm1',
    overlay_id: 'o1',
    platform: 'discord',
    channel_id: 'c1',
    channel_name: 'general',
    user: { id: 'u1', username: 'bob', display_name: 'Bob', badges: [] },
    message: { text: 'hello', emotes: [], attachments },
    timestamp: new Date('2024-01-01T00:00:00Z').toISOString(),
    metadata: {},
  } as ChatMessage
}

describe('MessageAttachments', () => {
  it('renders nothing when there are no attachments', () => {
    const { container } = render(<MessageAttachments message={makeMessage([])} />)
    expect(container.firstChild).toBeNull()
  })

  it('renders an image with its filename as alt text', () => {
    render(
      <MessageAttachments
        message={makeMessage([{ type: 'image', url: 'https://x/cat.png', filename: 'cat.png' }])}
      />
    )
    const img = screen.getByAltText('cat.png')
    expect(img.tagName).toBe('IMG')
    expect(img.getAttribute('src')).toBe('https://x/cat.png')
    expect(img.getAttribute('loading')).toBe('lazy')
  })

  it('falls back to a descriptive alt for GIFs without a filename', () => {
    render(
      <MessageAttachments
        message={makeMessage([{ type: 'image', url: 'https://x/a.gif', content_type: 'image/gif' }])}
      />
    )
    expect(screen.getByAltText('Shared GIF')).toBeTruthy()
  })

  it('renders a video with an accessible label', () => {
    render(
      <MessageAttachments
        message={makeMessage([{ type: 'video', url: 'https://x/clip.mp4' }])}
      />
    )
    const video = screen.getByLabelText('Shared video clip')
    expect(video.tagName).toBe('VIDEO')
    // Videos are always muted so they never play audio on the broadcast.
    expect((video as HTMLVideoElement).muted).toBe(true)
  })

  it('autoplays videos without controls on the overlay surface', () => {
    render(
      <MessageAttachments message={makeMessage([{ type: 'video', url: 'https://x/clip.mp4' }])} />
    )
    const video = screen.getByLabelText('Shared video clip') as HTMLVideoElement
    expect(video.autoplay).toBe(true)
    expect(video.controls).toBe(false)
  })

  it('exposes controls instead of autoplay in the compact surface (WCAG 2.2.2)', () => {
    render(
      <MessageAttachments
        message={makeMessage([{ type: 'video', url: 'https://x/clip.mp4' }])}
        variant="compact"
      />
    )
    const video = screen.getByLabelText('Shared video clip') as HTMLVideoElement
    expect(video.autoplay).toBe(false)
    expect(video.controls).toBe(true)
  })

  it('blurs spoilers behind a reveal control that toggles', () => {
    render(
      <MessageAttachments
        message={makeMessage([
          { type: 'image', url: 'https://x/s.png', filename: 's.png', spoiler: true },
        ])}
      />
    )
    const img = screen.getByAltText('s.png')
    expect(img.className).toContain('blur-xl')

    const revealBtn = screen.getByRole('button', { name: /reveal/i })
    fireEvent.click(revealBtn)

    // After revealing, the blur is gone and the control flips to "Hide".
    expect(screen.getByAltText('s.png').className).not.toContain('blur-xl')
    expect(screen.getByRole('button', { name: /hide/i })).toBeTruthy()
  })

  it('renders multiple attachments', () => {
    render(
      <MessageAttachments
        message={makeMessage([
          { type: 'image', url: 'https://x/1.png', filename: '1.png' },
          { type: 'image', url: 'https://x/2.png', filename: '2.png' },
        ])}
      />
    )
    expect(screen.getByAltText('1.png')).toBeTruthy()
    expect(screen.getByAltText('2.png')).toBeTruthy()
  })

  it('gives animated GIFs a hide/show control on the compact monitor (WCAG 2.2.2)', () => {
    render(
      <MessageAttachments
        message={makeMessage([
          { type: 'image', url: 'https://x/a.gif', content_type: 'image/gif', filename: 'a.gif' },
        ])}
        variant="compact"
      />
    )
    // Shown by default (motion on) with a Hide control.
    expect(screen.getByAltText('a.gif').tagName).toBe('IMG')
    fireEvent.click(screen.getByRole('button', { name: /hide gif/i }))
    // Hidden: the animating img is removed and the control flips to Show.
    expect(screen.queryByAltText('a.gif')).toBeNull()
    expect(screen.getByRole('button', { name: /show gif/i })).toBeTruthy()
  })

  it('leaves GIFs animating with no control on the broadcast overlay (out of scope)', () => {
    render(
      <MessageAttachments
        message={makeMessage([
          { type: 'image', url: 'https://x/a.gif', content_type: 'image/gif', filename: 'a.gif' },
        ])}
      />
    )
    expect(screen.getByAltText('a.gif').tagName).toBe('IMG')
    expect(screen.queryByRole('button', { name: /gif/i })).toBeNull()
  })

  it('defaults animated GIFs to hidden under reduced motion on the monitor', () => {
    Object.defineProperty(window, 'matchMedia', {
      writable: true,
      value: vi.fn().mockImplementation((query: string) => ({
        matches: true, // reduced motion
        media: query,
        onchange: null,
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
        addListener: vi.fn(),
        removeListener: vi.fn(),
        dispatchEvent: vi.fn(),
      })),
    })
    render(
      <MessageAttachments
        message={makeMessage([
          { type: 'image', url: 'https://x/a.gif', content_type: 'image/gif', filename: 'a.gif' },
        ])}
        variant="compact"
      />
    )
    expect(screen.queryByAltText('a.gif')).toBeNull()
    expect(screen.getByRole('button', { name: /show gif/i })).toBeTruthy()
  })
})
