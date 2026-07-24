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
import { describe, it, expect, vi, afterEach } from 'vitest'
import React from 'react'
import { render, screen, cleanup } from '@testing-library/react'
import type { Theme } from '@/lib/theme-marketplace/types'
import ThemeCard from '../ThemeCard'

afterEach(() => {
  cleanup()
})

// The preview components render CSS into an iframe/canvas that jsdom can't
// exercise; stub them so the card's own markup is what we assert on.
vi.mock('../ThemePreview', () => ({ default: () => <div data-testid="theme-preview" /> }))
vi.mock('../CreditRollThemePreview', () => ({
  default: () => <div data-testid="creditroll-preview" />,
}))

function makeTheme(tags: string[]): Theme {
  return {
    id: 'test-theme',
    name: 'Test Theme',
    filename: 'test-theme.css',
    description: 'A theme for testing',
    tags,
    css: '/* css */',
  }
}

const noop = () => {}

function renderCard(tags: string[]) {
  return render(
    <ThemeCard
      theme={makeTheme(tags)}
      isFavorite={false}
      messages={[]}
      onToggleFavorite={noop}
      onApply={noop}
    />
  )
}

describe('ThemeCard tag display', () => {
  it('shows every tag when there are 3 or fewer', () => {
    renderCard(['minimal', 'clean', 'modern'])
    expect(screen.getByText('minimal')).toBeDefined()
    expect(screen.getByText('clean')).toBeDefined()
    expect(screen.getByText('modern')).toBeDefined()
    expect(screen.queryByText(/^\+\d/)).toBeNull()
  })

  it('caps at 3 pills and collapses the rest into a +N chip', () => {
    renderCard(['minimal', 'clean', 'modern', 'icons', 'dark'])
    expect(screen.getByText('minimal')).toBeDefined()
    expect(screen.getByText('clean')).toBeDefined()
    expect(screen.getByText('modern')).toBeDefined()
    // Overflow tags are hidden from the pill row...
    expect(screen.queryByText('icons')).toBeNull()
    expect(screen.queryByText('dark')).toBeNull()
    // ...and summarized by a +2 chip that lists them on hover
    const overflow = screen.getByText('+2')
    expect(overflow).toBeDefined()
    expect(overflow.getAttribute('title')).toBe('icons, dark')
  })
})
