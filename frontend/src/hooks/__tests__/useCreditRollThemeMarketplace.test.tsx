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
import { act, renderHook, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import type { Theme, ThemeCacheData } from '@/lib/theme-marketplace/types'

vi.mock('@/lib/theme-marketplace/credit-roll-github-api', () => ({
  fetchAllCreditRollThemes: vi.fn(),
}))
import { fetchAllCreditRollThemes } from '@/lib/theme-marketplace/credit-roll-github-api'
import { useCreditRollThemeMarketplace } from '../useCreditRollThemeMarketplace'

const CACHE_KEY = 'credit-roll-themes-marketplace'
const FAVORITES_KEY = 'credit-roll-themes-favorites'

const mockFetchThemes = vi.mocked(fetchAllCreditRollThemes)

function theme(id: string, overrides: Partial<Theme> = {}): Theme {
  return {
    id,
    filename: `${id}.css`,
    name: id,
    description: `${id} description`,
    tags: [],
    css: `/* ${id} */`,
    ...overrides,
  }
}

beforeEach(() => {
  localStorage.clear()
  mockFetchThemes.mockReset()
})

afterEach(() => {
  vi.useRealTimers()
})

describe('useCreditRollThemeMarketplace loading', () => {
  it('exposes the fetched themes and clears loading once the fetch settles', async () => {
    mockFetchThemes.mockResolvedValue([theme('zebra'), theme('apple')])

    const { result } = renderHook(() => useCreditRollThemeMarketplace())
    expect(result.current.loading).toBe(true)

    await waitFor(() => expect(result.current.loading).toBe(false))
    expect(result.current.themes.map((t) => t.id)).toEqual(['apple', 'zebra'])
    expect(result.current.totalCount).toBe(2)
    expect(result.current.error).toBeNull()
  })

  it('fetches exactly once for the initial load', async () => {
    mockFetchThemes.mockResolvedValue([theme('apple')])

    const { result, rerender } = renderHook(() => useCreditRollThemeMarketplace())
    await waitFor(() => expect(result.current.loading).toBe(false))
    rerender()
    rerender()

    expect(mockFetchThemes).toHaveBeenCalledTimes(1)
  })

  it('writes the fetched themes to the cache and serves a second mount from it', async () => {
    mockFetchThemes.mockResolvedValue([theme('apple')])

    const first = renderHook(() => useCreditRollThemeMarketplace())
    await waitFor(() => expect(first.result.current.loading).toBe(false))
    first.unmount()

    const second = renderHook(() => useCreditRollThemeMarketplace())
    await waitFor(() => expect(second.result.current.loading).toBe(false))

    expect(second.result.current.themes.map((t) => t.id)).toEqual(['apple'])
    expect(mockFetchThemes).toHaveBeenCalledTimes(1)
  })

  it('refetches once past the cache TTL', async () => {
    const stale: ThemeCacheData = {
      timestamp: Date.now() - 48 * 60 * 60 * 1000,
      ttl: 24 * 60 * 60 * 1000,
      themes: [theme('stale')],
    }
    localStorage.setItem(CACHE_KEY, JSON.stringify(stale))
    mockFetchThemes.mockResolvedValue([theme('fresh')])

    const { result } = renderHook(() => useCreditRollThemeMarketplace())
    await waitFor(() => expect(result.current.themes.map((t) => t.id)).toEqual(['fresh']))
    expect(mockFetchThemes).toHaveBeenCalledTimes(1)
  })

  it('refreshThemes bypasses the cache and fetches exactly one more time', async () => {
    mockFetchThemes.mockResolvedValue([theme('apple')])

    const { result } = renderHook(() => useCreditRollThemeMarketplace())
    await waitFor(() => expect(result.current.loading).toBe(false))

    mockFetchThemes.mockResolvedValue([theme('banana')])
    await act(async () => {
      result.current.refreshThemes()
    })

    await waitFor(() => expect(result.current.themes.map((t) => t.id)).toEqual(['banana']))
    expect(mockFetchThemes).toHaveBeenCalledTimes(2)
  })

  it('reports the failure and leaves no themes when the fetch rejects', async () => {
    mockFetchThemes.mockRejectedValue(new Error('GitHub API error: 503'))
    const consoleError = vi.spyOn(console, 'error').mockImplementation(() => {})

    const { result } = renderHook(() => useCreditRollThemeMarketplace())
    await waitFor(() => expect(result.current.loading).toBe(false))

    expect(result.current.error).toBe('GitHub API error: 503')
    expect(result.current.themes).toEqual([])
    consoleError.mockRestore()
  })
})

describe('useCreditRollThemeMarketplace favorites', () => {
  it('restores favorites from localStorage and sorts them first', async () => {
    localStorage.setItem(FAVORITES_KEY, JSON.stringify(['zebra']))
    mockFetchThemes.mockResolvedValue([theme('apple'), theme('zebra')])

    const { result } = renderHook(() => useCreditRollThemeMarketplace())
    await waitFor(() => expect(result.current.loading).toBe(false))

    expect(result.current.favorites).toEqual(['zebra'])
    await waitFor(() => expect(result.current.themes.map((t) => t.id)).toEqual(['zebra', 'apple']))
  })

  it('falls back to no favorites when the stored value is not JSON', async () => {
    localStorage.setItem(FAVORITES_KEY, 'not json')
    mockFetchThemes.mockResolvedValue([theme('apple')])

    const { result } = renderHook(() => useCreditRollThemeMarketplace())
    await waitFor(() => expect(result.current.loading).toBe(false))

    expect(result.current.favorites).toEqual([])
  })

  it('persists a toggled favorite so the next mount restores it', async () => {
    mockFetchThemes.mockResolvedValue([theme('apple')])

    const first = renderHook(() => useCreditRollThemeMarketplace())
    await waitFor(() => expect(first.result.current.loading).toBe(false))
    act(() => first.result.current.toggleFavorite('apple'))
    await waitFor(() =>
      expect(JSON.parse(localStorage.getItem(FAVORITES_KEY) ?? '[]')).toEqual(['apple'])
    )
    first.unmount()

    const second = renderHook(() => useCreditRollThemeMarketplace())
    await waitFor(() => expect(second.result.current.favorites).toEqual(['apple']))
  })

  it('leaves stored favorites untouched when the reader never toggles one', async () => {
    localStorage.setItem(FAVORITES_KEY, JSON.stringify(['zebra']))
    mockFetchThemes.mockResolvedValue([theme('apple')])

    const { result, unmount } = renderHook(() => useCreditRollThemeMarketplace())
    await waitFor(() => expect(result.current.loading).toBe(false))
    unmount()

    expect(JSON.parse(localStorage.getItem(FAVORITES_KEY) ?? 'null')).toEqual(['zebra'])
  })

  it('toggling a favorite off clears it from storage', async () => {
    localStorage.setItem(FAVORITES_KEY, JSON.stringify(['apple']))
    mockFetchThemes.mockResolvedValue([theme('apple')])

    const { result } = renderHook(() => useCreditRollThemeMarketplace())
    await waitFor(() => expect(result.current.favorites).toEqual(['apple']))
    act(() => result.current.toggleFavorite('apple'))

    await waitFor(() => expect(result.current.favorites).toEqual([]))
    expect(JSON.parse(localStorage.getItem(FAVORITES_KEY) ?? 'null')).toEqual([])
  })
})

describe('useCreditRollThemeMarketplace filtering', () => {
  it('filters by search query across name and description', async () => {
    mockFetchThemes.mockResolvedValue([
      theme('apple', { name: 'Apple', description: 'crisp' }),
      theme('zebra', { name: 'Zebra', description: 'striped' }),
    ])

    const { result } = renderHook(() => useCreditRollThemeMarketplace())
    await waitFor(() => expect(result.current.loading).toBe(false))

    act(() => result.current.setSearchQuery('striped'))
    await waitFor(() => expect(result.current.themes.map((t) => t.id)).toEqual(['zebra']))
    expect(result.current.filteredCount).toBe(1)
    expect(result.current.totalCount).toBe(2)
    expect(result.current.hasActiveFilters).toBe(true)
  })

  it('filters by every selected tag and offers the union of tags', async () => {
    mockFetchThemes.mockResolvedValue([
      theme('apple', { tags: ['retro', 'dark'] }),
      theme('zebra', { tags: ['retro'] }),
    ])

    const { result } = renderHook(() => useCreditRollThemeMarketplace())
    await waitFor(() => expect(result.current.loading).toBe(false))
    expect(result.current.availableTags).toEqual(['dark', 'retro'])

    act(() => result.current.toggleTag('dark'))
    await waitFor(() => expect(result.current.themes.map((t) => t.id)).toEqual(['apple']))

    act(() => result.current.clearFilters())
    await waitFor(() => expect(result.current.themes).toHaveLength(2))
    expect(result.current.hasActiveFilters).toBe(false)
  })
})
