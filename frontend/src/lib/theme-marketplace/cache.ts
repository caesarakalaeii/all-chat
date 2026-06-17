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
 * Theme Marketplace Cache Manager
 *
 * Manages localStorage of theme favorites. (Theme CSS itself is bundled into
 * the build — see bundled-themes.ts — so there is no theme-list cache anymore.)
 */

const CACHE_KEY = 'theme_marketplace_cache'
const FAVORITES_KEY = 'theme_marketplace_favorites'

/**
 * Clear any legacy theme-list cache left over from the old GitHub-fetch path.
 * Kept so the marketplace "refresh" control can evict stale cached entries from
 * clients that ran the pre-bundle build.
 */
export function clearCache(): void {
  if (typeof window === 'undefined') return
  localStorage.removeItem(CACHE_KEY)
}

/**
 * Get favorite theme IDs
 */
export function getFavorites(): string[] {
  if (typeof window === 'undefined') return []

  try {
    const favorites = localStorage.getItem(FAVORITES_KEY)
    return favorites ? JSON.parse(favorites) : []
  } catch (error) {
    console.error('Failed to read favorites:', error)
    return []
  }
}

/**
 * Save favorite theme IDs
 */
export function saveFavorites(favorites: string[]): void {
  if (typeof window === 'undefined') return

  try {
    localStorage.setItem(FAVORITES_KEY, JSON.stringify(favorites))
  } catch (error) {
    console.error('Failed to save favorites:', error)
  }
}

/**
 * Toggle favorite status for a theme
 */
export function toggleFavorite(themeId: string): string[] {
  const favorites = getFavorites()
  const newFavorites = favorites.includes(themeId)
    ? favorites.filter((id) => id !== themeId)
    : [...favorites, themeId]

  saveFavorites(newFavorites)
  return newFavorites
}
