'use client'

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


import React from 'react'
import ThemeCard from './ThemeCard'
import ThemeFilters from './ThemeFilters'
import { useThemeMarketplace } from '@/hooks/useThemeMarketplace'
import { SAMPLE_PREVIEW_MESSAGES } from '@/lib/theme-marketplace/constants'
import { clearCache } from '@/lib/theme-marketplace/cache'

interface ThemeContentProps {
  onApply: (css: string) => void
  isAdmin?: boolean
}

export function ThemeContent({ onApply, isAdmin = false }: ThemeContentProps): React.ReactElement {
  const {
    themes,
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
    hasActiveFilters,
    totalCount,
    filteredCount,
    refreshThemes,
  } = useThemeMarketplace()

  return (
    <div className="@container space-y-3">
      {/* Filters */}
      <ThemeFilters
        searchQuery={searchQuery}
        onSearchChange={setSearchQuery}
        availableTags={availableTags}
        selectedTags={selectedTags}
        onToggleTag={toggleTag}
        onClearFilters={clearFilters}
        hasActiveFilters={hasActiveFilters}
      />

      {/* Loading State */}
      {loading && (
        <div className="flex items-center justify-center py-8">
          <div className="text-center">
            <div className="mx-auto mb-3 h-8 w-8 animate-spin rounded-full border-b-2 border-purple-500" />
            <p className="text-sm text-text-sub">Loading themes...</p>
          </div>
        </div>
      )}

      {/* Error State */}
      {error && !loading && (
        <div className="rounded-lg border border-yellow-500/30 bg-yellow-500/10 p-3">
          <p className="text-sm font-semibold text-yellow-400">Error Loading Themes</p>
          <p className="text-xs text-yellow-300/80">{error}</p>
        </div>
      )}

      {/* Empty State */}
      {!loading && themes.length === 0 && !error && (
        <div className="py-8 text-center">
          <p className="text-sm text-text-sub">No themes found</p>
          <p className="mt-1 text-xs text-text-dim">Try adjusting your filters</p>
        </div>
      )}

      {/* Theme Grid */}
      {!loading && themes.length > 0 && (
        <div className="space-y-2">
          <div className="flex items-center justify-between">
            <p className="text-xs text-text-sub">
              Showing {filteredCount} of {totalCount} themes
            </p>
            {isAdmin && (
              <button
                onClick={() => { clearCache(); refreshThemes() }}
                className="flex items-center gap-1 rounded px-2 py-1 text-xs text-text-dim transition-colors hover:bg-subtle hover:text-text"
                title="Force refresh themes from GitHub (Admin)"
              >
                <svg className="h-3.5 w-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                </svg>
                Sync
              </button>
            )}
          </div>
          <div className="grid grid-cols-1 gap-6 @[480px]:grid-cols-2 @[768px]:grid-cols-3">
            {themes.map((theme) => (
              <ThemeCard
                key={theme.id}
                theme={theme}
                isFavorite={favorites.includes(theme.id)}
                messages={SAMPLE_PREVIEW_MESSAGES}
                onToggleFavorite={toggleFavorite}
                onApply={onApply}
                themeType="overlay"
              />
            ))}
          </div>
        </div>
      )}
    </div>
  )
}
