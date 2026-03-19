'use client'

import React from 'react'
import ThemeCard from './ThemeCard'
import ThemeFilters from './ThemeFilters'
import { useThemeMarketplace } from '@/hooks/useThemeMarketplace'
import { SAMPLE_PREVIEW_MESSAGES } from '@/lib/theme-marketplace/constants'

interface ThemeContentProps {
  onApply: (css: string) => void
}

export function ThemeContent({ onApply }: ThemeContentProps): React.ReactElement {
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
  } = useThemeMarketplace()

  return (
    <div className="space-y-3">
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
          <p className="text-xs text-text-sub">
            Showing {filteredCount} of {totalCount} themes
          </p>
          <div className="grid grid-cols-1 gap-6 md:grid-cols-2 lg:grid-cols-3">
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
