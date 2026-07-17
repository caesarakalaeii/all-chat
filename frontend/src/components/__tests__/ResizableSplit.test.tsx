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
import { fireEvent, render, screen, waitFor, cleanup } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { ResizableSplit } from '@/components/ResizableSplit'

function makeLocalStorageMock(): Storage {
  let store: Record<string, string> = {}
  return {
    getItem: (k: string) => (k in store ? store[k] : null),
    setItem: (k: string, v: string) => {
      store[k] = String(v)
    },
    removeItem: (k: string) => {
      delete store[k]
    },
    clear: () => {
      store = {}
    },
    key: (i: number) => Object.keys(store)[i] ?? null,
    get length() {
      return Object.keys(store).length
    },
  } as Storage
}

beforeEach(() => {
  vi.stubGlobal('localStorage', makeLocalStorageMock())
  vi.stubGlobal(
    'matchMedia',
    vi.fn().mockReturnValue({
      matches: true, // desktop
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    })
  )
})

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
})

describe('ResizableSplit', () => {
  it('moves the divider with the arrow keys and persists the ratio', () => {
    const { container } = render(
      <ResizableSplit storageKey="k" left={<div>L</div>} right={<div>R</div>} initial={45} />
    )
    const leftPanel = container.firstChild?.firstChild as HTMLElement
    expect(leftPanel.style.width).toBe('45%')

    fireEvent.keyDown(screen.getByRole('slider'), { key: 'ArrowRight' })
    expect(leftPanel.style.width).toBe('50%')
    expect(localStorage.getItem('k')).toBe('50')

    fireEvent.keyDown(screen.getByRole('slider'), { key: 'ArrowLeft' })
    expect(leftPanel.style.width).toBe('45%')
    expect(localStorage.getItem('k')).toBe('45')
  })

  it('steps the split by 10% per click of the step buttons and persists', () => {
    const { container } = render(
      <ResizableSplit storageKey="k" left={<div>L</div>} right={<div>R</div>} initial={45} />
    )
    const leftPanel = container.firstChild?.firstChild as HTMLElement

    fireEvent.click(screen.getByRole('button', { name: 'Grow left panel' }))
    expect(leftPanel.style.width).toBe('55%')
    expect(localStorage.getItem('k')).toBe('55')

    fireEvent.click(screen.getByRole('button', { name: 'Shrink left panel' }))
    expect(leftPanel.style.width).toBe('45%')
    expect(localStorage.getItem('k')).toBe('45')
  })

  it('exposes the split percentage as slider value semantics', () => {
    render(<ResizableSplit storageKey="k" left={<div>L</div>} right={<div>R</div>} initial={45} />)
    const slider = screen.getByRole('slider')
    expect(slider.getAttribute('aria-valuenow')).toBe('45')
    expect(slider.getAttribute('aria-valuemin')).toBe('25')
    expect(slider.getAttribute('aria-valuemax')).toBe('70')
  })

  it('clamps within min/max', () => {
    const { container } = render(
      <ResizableSplit
        storageKey="k"
        left={<div>L</div>}
        right={<div>R</div>}
        initial={28}
        min={25}
        max={30}
      />
    )
    const leftPanel = container.firstChild?.firstChild as HTMLElement
    const slider = screen.getByRole('slider')
    fireEvent.keyDown(slider, { key: 'ArrowLeft' }) // 28 - 5 -> clamp 25
    expect(leftPanel.style.width).toBe('25%')
    fireEvent.keyDown(slider, { key: 'ArrowRight' }) // 25 + 5 -> clamp 30
    expect(leftPanel.style.width).toBe('30%')
  })

  it('restores the persisted ratio after hydration', async () => {
    localStorage.setItem('k', '62')
    const { container } = render(
      <ResizableSplit storageKey="k" left={<div>L</div>} right={<div>R</div>} initial={45} />
    )
    const leftPanel = container.firstChild?.firstChild as HTMLElement
    await waitFor(() => expect(leftPanel.style.width).toBe('62%'))
  })

  it('sizes height (not width) and uses Up/Down keys when orientation is vertical', () => {
    const { container } = render(
      <ResizableSplit
        storageKey="k"
        left={<div>L</div>}
        right={<div>R</div>}
        initial={45}
        orientation="vertical"
      />
    )
    const firstPanel = container.firstChild?.firstChild as HTMLElement
    expect(firstPanel.style.height).toBe('45%')
    expect(firstPanel.style.width).toBe('')

    const slider = screen.getByRole('slider')
    expect(slider.getAttribute('aria-orientation')).toBe('vertical')

    fireEvent.keyDown(slider, { key: 'ArrowDown' })
    expect(firstPanel.style.height).toBe('50%')
    fireEvent.keyDown(slider, { key: 'ArrowUp' })
    expect(firstPanel.style.height).toBe('45%')
  })

  it('renders the right panel first when reversed', () => {
    const { container } = render(
      <ResizableSplit storageKey="k" left={<div>L</div>} right={<div>R</div>} reversed />
    )
    const firstPanel = container.firstChild?.firstChild as HTMLElement
    expect(firstPanel.textContent).toBe('R')
  })

  it('exposes a horizontal slider for the default horizontal layout', () => {
    render(<ResizableSplit storageKey="k" left={<div>L</div>} right={<div>R</div>} />)
    expect(screen.getByRole('slider').getAttribute('aria-orientation')).toBe('horizontal')
  })
})
