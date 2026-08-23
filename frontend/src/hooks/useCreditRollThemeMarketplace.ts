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

/**
 * Credit Roll Theme Marketplace Hook
 *
 * Manages credit roll theme loading, caching, filtering, and favorites
 */

import { useState, useEffect, useCallback, useMemo } from 'react'
import type { Theme, ThemeCacheData } from '@/lib/theme-marketplace/types'
import { fetchAllCreditRollThemes } from '@/lib/theme-marketplace/credit-roll-github-api'
import { useHydrated } from '@/hooks/useHydrated'

const CACHE_KEY = 'credit-roll-themes-marketplace'
const FAVORITES_KEY = 'credit-roll-themes-favorites'
const CACHE_TTL = 24 * 60 * 60 * 1000 // 24 hours

function storedFavorites(): string[] {
  try {
    const stored = localStorage.getItem(FAVORITES_KEY)
    return stored ? JSON.parse(stored) : []
  } catch {
    // Absent, blocked, or not JSON — start from no favorites.
    return []
  }
}

/** The cached theme list, or null if there is none or it has aged out. */
function cachedThemes(): Theme[] | null {
  try {
    const cached = localStorage.getItem(CACHE_KEY)
    if (!cached) return null
    const data: ThemeCacheData = JSON.parse(cached)
    return Date.now() - data.timestamp <= data.ttl ? data.themes : null
  } catch {
    return null
  }
}

export function useCreditRollThemeMarketplace() {
  const [themes, setThemes] = useState<Theme[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [searchQuery, setSearchQuery] = useState('')
  const [selectedTags, setSelectedTags] = useState<string[]>([])
  // null until the reader stars or unstars something in this session, at which point
  // their list wins over what is stored. Derived rather than copied out of localStorage
  // by an effect (react-hooks/set-state-in-effect), and only consulted after hydration
  // because the server has no localStorage to read.
  const hydrated = useHydrated()
  const [pickedFavorites, setPickedFavorites] = useState<string[] | null>(null)
  // Memoised so it is a stable array identity: `toggleFavorite` closes over it.
  const favorites = useMemo(
    () => (hydrated ? (pickedFavorites ?? storedFavorites()) : []),
    [hydrated, pickedFavorites]
  )

  // A promise chain, not async/await, and the synchronous cache branch is deferred into a
  // `.then` as well: `react-hooks/set-state-in-effect` follows this call from the effect
  // below, so no setState may be reachable without first yielding. Every setState here is
  // in a callback for that reason.
  const loadThemes = useCallback(
    (forceRefresh = false) =>
      Promise.resolve()
        .then(() => {
          const cached = forceRefresh || typeof window === 'undefined' ? null : cachedThemes()
          if (cached) {
            setThemes(cached)
            setError(null)
            setLoading(false)
            return
          }
          setLoading(true)
          setError(null)
          return fetchAllCreditRollThemes().then((fetchedThemes) => {
            setThemes(fetchedThemes)
            setLoading(false)
            const cacheData: ThemeCacheData = {
              timestamp: Date.now(),
              ttl: CACHE_TTL,
              themes: fetchedThemes,
            }
            try {
              localStorage.setItem(CACHE_KEY, JSON.stringify(cacheData))
            } catch {
              // Storage full or blocked — every mount just refetches.
            }
          })
        })
        .catch((err) => {
          console.error('Failed to load credit roll themes:', err)
          setError(err instanceof Error ? err.message : 'Failed to load themes')
          setThemes([])
          setLoading(false)
        }),
    []
  )

  useEffect(() => {
    void loadThemes()
  }, [loadThemes])

  // Persist the reader's own list. Skipped until they touch it, so an unopened
  // marketplace never writes an empty list over what is already stored.
  useEffect(() => {
    if (pickedFavorites === null) return
    try {
      localStorage.setItem(FAVORITES_KEY, JSON.stringify(pickedFavorites))
    } catch {
      // Storage blocked — favorites hold for this session only.
    }
  }, [pickedFavorites])

  const filteredThemes = themes.filter((theme) => {
    const matchesSearch =
      searchQuery === '' ||
      theme.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
      theme.description.toLowerCase().includes(searchQuery.toLowerCase())

    const matchesTags =
      selectedTags.length === 0 || selectedTags.every((tag) => theme.tags.includes(tag))

    return matchesSearch && matchesTags
  })

  const sortedThemes = [...filteredThemes].sort((a, b) => {
    const aFav = favorites.includes(a.id)
    const bFav = favorites.includes(b.id)
    if (aFav && !bFav) return -1
    if (!aFav && bFav) return 1
    return a.name.localeCompare(b.name)
  })

  const availableTags = Array.from(new Set(themes.flatMap((theme) => theme.tags))).sort()

  const toggleTag = useCallback((tag: string) => {
    setSelectedTags((prev) => (prev.includes(tag) ? prev.filter((t) => t !== tag) : [...prev, tag]))
  }, [])

  const toggleFavorite = useCallback(
    (themeId: string) => {
      // Starts from the currently shown list, which on the first toggle is the stored one.
      const next = favorites.includes(themeId)
        ? favorites.filter((id) => id !== themeId)
        : [...favorites, themeId]
      setPickedFavorites(next)
    },
    [favorites]
  )

  const clearFilters = useCallback(() => {
    setSearchQuery('')
    setSelectedTags([])
  }, [])

  return {
    themes: sortedThemes,
    loading,
    error,
    searchQuery,
    setSearchQuery,
    selectedTags,
    toggleTag,
    favorites,
    toggleFavorite,
    availableTags,
    clearFilters,
    hasActiveFilters: searchQuery !== '' || selectedTags.length > 0,
    totalCount: themes.length,
    filteredCount: filteredThemes.length,
    refreshThemes: () => loadThemes(true),
  }
}
