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
import { render, screen, fireEvent, cleanup } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { LegalThemeToggle } from '@/components/legal/LegalThemeToggle'

const STORAGE_KEY = 'legal-light-mode'

/**
 * The toggle drives the `legal-light` class on `#legal-wrapper`, which the real
 * legal layout renders around the page. Tests stand that element up themselves.
 */
function renderInWrapper() {
  const wrapper = document.createElement('div')
  wrapper.id = 'legal-wrapper'
  document.body.appendChild(wrapper)
  render(<LegalThemeToggle />, { container: wrapper })
  return wrapper
}

beforeEach(() => localStorage.clear())
afterEach(() => {
  cleanup()
  document.body.innerHTML = ''
  vi.restoreAllMocks()
})

describe('LegalThemeToggle', () => {
  it('renders in light mode and lights the wrapper when the stored preference is true', () => {
    localStorage.setItem(STORAGE_KEY, 'true')
    const wrapper = renderInWrapper()
    expect(screen.getByRole('button', { name: 'Switch to dark mode' })).toBeInTheDocument()
    expect(wrapper).toHaveClass('legal-light')
  })

  it('renders in dark mode and leaves the wrapper unlit when nothing is stored', () => {
    const wrapper = renderInWrapper()
    expect(screen.getByRole('button', { name: 'Switch to light mode' })).toBeInTheDocument()
    expect(wrapper).not.toHaveClass('legal-light')
  })

  it('renders in dark mode when the stored preference is false', () => {
    localStorage.setItem(STORAGE_KEY, 'false')
    const wrapper = renderInWrapper()
    expect(screen.getByRole('button', { name: 'Switch to light mode' })).toBeInTheDocument()
    expect(wrapper).not.toHaveClass('legal-light')
  })

  it('clicking lights the wrapper and writes the new preference back', () => {
    const wrapper = renderInWrapper()
    fireEvent.click(screen.getByRole('button', { name: 'Switch to light mode' }))
    expect(wrapper).toHaveClass('legal-light')
    expect(localStorage.getItem(STORAGE_KEY)).toBe('true')
    expect(screen.getByRole('button', { name: 'Switch to dark mode' })).toBeInTheDocument()
  })

  it('renders dark and does not throw when localStorage is unavailable', () => {
    const blocked = () => {
      throw new Error('SecurityError: storage is disabled')
    }
    vi.spyOn(Storage.prototype, 'getItem').mockImplementation(blocked)
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(blocked)

    const wrapper = renderInWrapper()
    expect(screen.getByRole('button', { name: 'Switch to light mode' })).toBeInTheDocument()
    expect(wrapper).not.toHaveClass('legal-light')
  })

  it('clicking again unlights the wrapper and writes the preference back', () => {
    localStorage.setItem(STORAGE_KEY, 'true')
    const wrapper = renderInWrapper()
    fireEvent.click(screen.getByRole('button', { name: 'Switch to dark mode' }))
    expect(wrapper).not.toHaveClass('legal-light')
    expect(localStorage.getItem(STORAGE_KEY)).toBe('false')
  })
})
