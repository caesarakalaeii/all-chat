/**
 * This file is part of All-Chat.
 * Copyright (C) 2026 caesarakalaeii
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published
 * by the Free Software Foundation, either version 3 of the License, or
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
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { act, cleanup, fireEvent, render, screen } from '@testing-library/react'
import { LoadMore } from '../LoadMore'
import type { PagedList } from '@/hooks/usePagedList'

// Captures the observer LoadMore registers so a test can scroll it into view.
let observed: { callback: IntersectionObserverCallback; disconnected: boolean }[] = []

beforeEach(() => {
  observed = []
  vi.stubGlobal(
    'IntersectionObserver',
    class {
      entry: (typeof observed)[number]
      constructor(callback: IntersectionObserverCallback) {
        this.entry = { callback, disconnected: false }
        observed.push(this.entry)
      }
      observe() {}
      disconnect() {
        this.entry.disconnected = true
      }
    }
  )
})

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
})

function list(overrides: Partial<PagedList<unknown>> = {}): PagedList<unknown> {
  return {
    items: [],
    total: 10,
    response: undefined,
    loading: false,
    loadingMore: false,
    error: false,
    hasMore: true,
    loadMore: vi.fn(),
    reload: vi.fn(),
    ...overrides,
  }
}

const scrollIntoView = (isIntersecting: boolean) =>
  act(() => {
    const live = observed.filter((o) => !o.disconnected)
    live.forEach((o) =>
      o.callback([{ isIntersecting } as IntersectionObserverEntry], {} as IntersectionObserver)
    )
  })

describe('LoadMore', () => {
  it('loads the next page when it scrolls into view', () => {
    const l = list()
    render(<LoadMore list={l} />)
    scrollIntoView(false)
    expect(l.loadMore).not.toHaveBeenCalled()
    scrollIntoView(true)
    expect(l.loadMore).toHaveBeenCalledTimes(1)
  })

  it('loads the next page from the button, for keyboard users', () => {
    const l = list()
    render(<LoadMore list={l} />)
    fireEvent.click(screen.getByRole('button', { name: 'Load more' }))
    expect(l.loadMore).toHaveBeenCalledTimes(1)
  })

  it('does not auto-retry a failed page, but offers a retry', () => {
    const l = list({ error: true })
    render(<LoadMore list={l} />)
    scrollIntoView(true)
    expect(l.loadMore).not.toHaveBeenCalled()
    fireEvent.click(screen.getByRole('button', { name: "Couldn't load more. Try again" }))
    expect(l.loadMore).toHaveBeenCalledTimes(1)
  })

  it('shows progress while a page loads', () => {
    render(<LoadMore list={list({ loadingMore: true })} />)
    expect(screen.getByRole('button', { name: 'Loading...' })).toBeDisabled()
  })

  it('renders nothing once every row is loaded', () => {
    const { container } = render(<LoadMore list={list({ hasMore: false })} />)
    expect(container).toBeEmptyDOMElement()
  })

  it('renders as a list item inside a list', () => {
    render(
      <ul>
        <LoadMore list={list()} as="li" />
      </ul>
    )
    expect(screen.getByRole('listitem')).toContainElement(screen.getByRole('button'))
  })
})
