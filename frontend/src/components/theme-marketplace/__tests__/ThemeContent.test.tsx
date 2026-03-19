// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import React from 'react'
import { render, screen, fireEvent, cleanup } from '@testing-library/react'

afterEach(() => { cleanup() })

// Mock useThemeMarketplace hook
vi.mock('@/hooks/useThemeMarketplace', () => ({
  useThemeMarketplace: vi.fn(),
}))

// Mock ThemeFilters to keep tests lightweight
vi.mock('../ThemeFilters', () => ({
  default: ({ searchQuery, onSearchChange }: { searchQuery: string; onSearchChange: (q: string) => void }) => (
    <div data-testid="theme-filters">
      <input
        role="searchbox"
        aria-label="Search themes"
        value={searchQuery}
        onChange={(e) => onSearchChange(e.target.value)}
        placeholder="Search themes"
      />
    </div>
  ),
}))

// Mock ThemeCard to keep tests lightweight
vi.mock('../ThemeCard', () => ({
  default: ({
    theme,
    onApply,
  }: {
    theme: { id: string; name: string; css: string }
    onApply: (css: string) => void
  }) => (
    <div data-testid={`theme-card-${theme.id}`}>
      <span>{theme.name}</span>
      <button onClick={() => onApply(theme.css)}>Apply {theme.name}</button>
    </div>
  ),
}))

// Mock constants
vi.mock('@/lib/theme-marketplace/constants', () => ({
  SAMPLE_PREVIEW_MESSAGES: [],
}))

import { useThemeMarketplace } from '@/hooks/useThemeMarketplace'
import { ThemeContent } from '../ThemeContent'

const mockUseThemeMarketplace = vi.mocked(useThemeMarketplace)

const defaultHookReturn = {
  themes: [],
  loading: false,
  error: null,
  searchQuery: '',
  setSearchQuery: vi.fn(),
  selectedTags: [],
  toggleTag: vi.fn(),
  favorites: [],
  toggleFavorite: vi.fn(),
  availableTags: [],
  clearFilters: vi.fn(),
  hasActiveFilters: false,
  totalCount: 0,
  filteredCount: 0,
  refreshThemes: vi.fn(),
}

describe('ThemeContent', () => {
  beforeEach(() => {
    mockUseThemeMarketplace.mockReturnValue(defaultHookReturn)
  })

  it('renders without a fixed/absolute positioned overlay element (no modal wrapper)', () => {
    const { container } = render(<ThemeContent onApply={vi.fn()} />)
    // The root element should not have fixed or absolute positioning
    const allElements = container.querySelectorAll('*')
    for (const el of allElements) {
      const style = window.getComputedStyle(el)
      expect(style.position).not.toBe('fixed')
      expect(style.position).not.toBe('absolute')
    }
  })

  it('does not render any element with role="dialog"', () => {
    render(<ThemeContent onApply={vi.fn()} />)
    const dialogs = screen.queryAllByRole('dialog')
    expect(dialogs).toHaveLength(0)
  })

  it('renders ThemeFilters (search input) when loaded', () => {
    mockUseThemeMarketplace.mockReturnValue({
      ...defaultHookReturn,
      themes: [],
      loading: false,
    })
    render(<ThemeContent onApply={vi.fn()} />)
    expect(screen.getByTestId('theme-filters')).toBeDefined()
  })

  it('calls onApply with CSS string when a theme card apply button is clicked', () => {
    const testTheme = {
      id: 'theme-1',
      name: 'Dark Theme',
      css: '.chat { color: white; }',
      description: 'A dark theme',
      tags: [],
      author: 'test',
      version: '1.0',
      preview: '',
      createdAt: '',
      updatedAt: '',
    }
    mockUseThemeMarketplace.mockReturnValue({
      ...defaultHookReturn,
      themes: [testTheme],
      totalCount: 1,
      filteredCount: 1,
    })

    const onApply = vi.fn()
    render(<ThemeContent onApply={onApply} />)

    const applyButton = screen.getByRole('button', { name: /Apply Dark Theme/i })
    fireEvent.click(applyButton)

    expect(onApply).toHaveBeenCalledWith('.chat { color: white; }')
    expect(onApply).toHaveBeenCalledTimes(1)
  })
})
