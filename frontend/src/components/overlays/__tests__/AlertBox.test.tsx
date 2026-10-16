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
import { act, cleanup, render, screen } from '@testing-library/react'
import { useEffect } from 'react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { AlertPlayer, useAlertQueue } from '@/components/overlays/AlertBox'
import type { AlertFrame } from '@/components/overlays/alertQueue'

class FakeAudio {
  static instances: FakeAudio[] = []
  volume = 1
  src: string
  private listeners: Record<string, Array<() => void>> = {}
  play = vi.fn(() => Promise.resolve())
  pause = vi.fn()
  constructor(src: string) {
    this.src = src
    FakeAudio.instances.push(this)
  }
  addEventListener(type: string, fn: () => void) {
    ;(this.listeners[type] ??= []).push(fn)
  }
  removeEventListener() {}
  fire(type: string) {
    this.listeners[type]?.forEach((fn) => fn())
  }
}

function alert(id: string, profile: Partial<NonNullable<AlertFrame['profile']>> = {}): AlertFrame {
  return {
    alert_id: id,
    overlay_id: 'o',
    platform: 'twitch',
    event_type: 'bits',
    event_data: { type: 'bits' },
    user: { id: 'u', name: 'Viewer', avatar_url: '' },
    profile: {
      event_type: 'bits',
      text_template: '',
      sound_url: '',
      image_url: '',
      duration_ms: 1000,
      volume: 100,
      enabled: true,
      sort_after: 0,
      text: `text ${id}`,
      ...profile,
    },
  }
}

// The page owns the queue (fed by the stream hook's onAlert) and renders the
// player for whatever is current; the harness wires them the same way.
const sink: { push: (a: AlertFrame) => void } = { push: () => {} }
function Harness() {
  const queue = useAlertQueue()
  useEffect(() => {
    sink.push = queue.enqueue
  })
  return <AlertPlayer alert={queue.current} onDone={queue.finish} />
}

function feed(...alerts: AlertFrame[]) {
  const view = render(<Harness />)
  act(() => alerts.forEach((a) => sink.push(a)))
  return view
}

describe('AlertBox', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    FakeAudio.instances = []
    vi.stubGlobal('Audio', FakeAudio)
  })
  afterEach(() => {
    cleanup()
    vi.useRealTimers()
    vi.unstubAllGlobals()
  })

  it('renders nothing while the queue is empty', () => {
    const { container } = render(<Harness />)
    expect(container).toBeEmptyDOMElement()
  })

  it('plays one alert at a time and advances after duration_ms', () => {
    feed(alert('a'), alert('b'))
    // The text is in the accessibility tree, in a live region.
    expect(screen.getByRole('status')).toHaveTextContent('text a')
    expect(screen.queryByText('text b')).not.toBeInTheDocument()

    act(() => vi.advanceTimersByTime(999))
    expect(screen.getByText('text a')).toBeInTheDocument()
    act(() => vi.advanceTimersByTime(1))
    expect(screen.getByText('text b')).toBeInTheDocument()

    act(() => vi.advanceTimersByTime(1000))
    expect(screen.queryByRole('status')).not.toBeInTheDocument()
  })

  it('drops a duplicate alert_id fed twice', () => {
    feed(alert('a'), alert('a'))
    act(() => vi.advanceTimersByTime(1000))
    expect(screen.queryByRole('status')).not.toBeInTheDocument()
  })

  it('plays the sound once at the profile volume and waits for it to end', () => {
    feed(alert('a', { sound_url: 'https://x/s.mp3', volume: 40 }), alert('b'))
    expect(FakeAudio.instances).toHaveLength(1)
    expect(FakeAudio.instances[0].volume).toBeCloseTo(0.4)
    expect(FakeAudio.instances[0].play).toHaveBeenCalledTimes(1)

    act(() => vi.advanceTimersByTime(1000))
    // Duration elapsed but the sound is still playing: the media end is later.
    expect(screen.getByText('text a')).toBeInTheDocument()
    act(() => FakeAudio.instances[0].fire('ended'))
    expect(screen.getByText('text b')).toBeInTheDocument()
    expect(FakeAudio.instances).toHaveLength(1)
  })

  it('shows the profile image', () => {
    feed(alert('a', { image_url: 'https://x/a.gif' }))
    expect(document.querySelector('img')).toHaveAttribute('src', 'https://x/a.gif')
  })
})
