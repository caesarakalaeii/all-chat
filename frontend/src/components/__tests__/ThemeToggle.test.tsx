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
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { ThemeToggle } from '../ThemeToggle'
import { chooseTheme } from '@/lib/theme'

beforeEach(() => {
  const data = new Map<string, string>()
  vi.stubGlobal('localStorage', {
    getItem: (k: string) => data.get(k) ?? null,
    setItem: (k: string, v: string) => void data.set(k, v),
  })
})

afterEach(() => {
  cleanup()
  chooseTheme('dark')
  vi.unstubAllGlobals()
})

describe('ThemeToggle', () => {
  it('switches the whole document to light and back', () => {
    render(<ThemeToggle />)
    const toggle = screen.getByRole('button', { name: 'Light mode' })
    expect(toggle).toHaveAttribute('aria-pressed', 'false')

    fireEvent.click(toggle)
    expect(document.documentElement.dataset.theme).toBe('light')
    expect(toggle).toHaveAttribute('aria-pressed', 'true')

    fireEvent.click(toggle)
    expect(document.documentElement.dataset.theme).toBeUndefined()
    expect(toggle).toHaveAttribute('aria-pressed', 'false')
  })

  it('keeps every toggle on the page in step', () => {
    render(
      <>
        <ThemeToggle />
        <ThemeToggle />
      </>
    )
    const [first, second] = screen.getAllByRole('button', { name: 'Light mode' })
    fireEvent.click(first)
    expect(second).toHaveAttribute('aria-pressed', 'true')
  })
})
