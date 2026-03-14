/**
 * Theme Marketplace Modal Component
 *
 * Main modal container for browsing and applying themes.
 */

'use client'

import { useEffect } from 'react'
import ThemeCard from './ThemeCard'
import ThemeFilters from './ThemeFilters'
import { useThemeMarketplace } from '@/hooks/useThemeMarketplace'
import { useCreditRollThemeMarketplace } from '@/hooks/useCreditRollThemeMarketplace'
import { SAMPLE_PREVIEW_MESSAGES } from '@/lib/theme-marketplace/constants'
import { useAuthStore } from '@/lib/stores/auth-store'
import { clearCache } from '@/lib/theme-marketplace/cache'

interface ThemeMarketplaceModalProps {
  isOpen: boolean
  onClose: () => void
  onApplyTheme: (css: string) => void
  themeType?: 'overlay' | 'creditroll'
}

export default function ThemeMarketplaceModal({
  isOpen,
  onClose,
  onApplyTheme,
  themeType = 'overlay',
}: ThemeMarketplaceModalProps) {
  const { user } = useAuthStore()
  const isAdmin = user?.is_admin || false

  // Add custom scrollbar styles
  useEffect(() => {
    if (!isOpen) return

    const style = document.createElement('style')
    style.id = 'theme-marketplace-scrollbar'
    style.textContent = `
      .custom-scrollbar::-webkit-scrollbar {
        width: 12px;
      }
      .custom-scrollbar::-webkit-scrollbar-track {
        background: rgba(31, 41, 55, 0.5);
        border-radius: 6px;
      }
      .custom-scrollbar::-webkit-scrollbar-thumb {
        background: rgba(139, 92, 246, 0.6);
        border-radius: 6px;
        border: 2px solid rgba(31, 41, 55, 0.5);
      }
      .custom-scrollbar::-webkit-scrollbar-thumb:hover {
        background: rgba(139, 92, 246, 0.8);
      }
      .theme-preview-body::-webkit-scrollbar {
        width: 6px;
      }
      .theme-preview-body::-webkit-scrollbar-track {
        background: rgba(0, 0, 0, 0.3);
      }
      .theme-preview-body::-webkit-scrollbar-thumb {
        background: rgba(107, 114, 128, 0.5);
        border-radius: 3px;
      }
      .theme-preview-body::-webkit-scrollbar-thumb:hover {
        background: rgba(107, 114, 128, 0.7);
      }
    `
    document.head.appendChild(style)

    return () => {
      const existingStyle = document.getElementById('theme-marketplace-scrollbar')
      if (existingStyle) {
        existingStyle.remove()
      }
    }
  }, [isOpen])

  // Use appropriate hook based on theme type
  const overlayThemes = useThemeMarketplace()
  const creditRollThemes = useCreditRollThemeMarketplace()

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
  } = themeType === 'creditroll' ? creditRollThemes : overlayThemes

  // Handle ESC key to close modal
  useEffect(() => {
    if (!isOpen) return

    const handleEscape = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        onClose()
      }
    }

    document.addEventListener('keydown', handleEscape)
    return () => document.removeEventListener('keydown', handleEscape)
  }, [isOpen, onClose])

  // Prevent body scroll when modal is open
  useEffect(() => {
    if (isOpen) {
      document.body.style.overflow = 'hidden'
    } else {
      document.body.style.overflow = ''
    }
    return () => {
      document.body.style.overflow = ''
    }
  }, [isOpen])

  if (!isOpen) return null

  const handleApply = (css: string) => {
    onApplyTheme(css)
    onClose()
  }

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/80 p-4"
      onClick={(e) => {
        if (e.target === e.currentTarget) onClose()
      }}
    >
      <div className="flex max-h-[90vh] w-full max-w-6xl flex-col overflow-hidden rounded-lg border border-slate-700 bg-slate-800 shadow-2xl">
        {/* Header */}
        <div className="flex items-center justify-between border-b border-slate-700 p-6">
          <div>
            <h2 id="theme-marketplace-title" className="text-2xl font-bold text-white">
              {themeType === 'creditroll' ? 'Credit Roll' : ''} Theme Marketplace
            </h2>
            <p className="mt-1 text-sm text-slate-400">
              {themeType === 'creditroll'
                ? 'Browse and apply custom CSS themes for your credit roll'
                : 'Browse and apply custom CSS themes for your overlay'}
            </p>
          </div>
          <div className="flex items-center gap-2">
            {/* Admin Force Refresh Button */}
            {isAdmin && (
              <button
                onClick={() => {
                  clearCache()
                  refreshThemes()
                }}
                className="rounded-lg p-2 text-slate-400 transition-colors hover:bg-slate-700 hover:text-purple-400"
                aria-label="Force refresh themes from GitHub"
                title="Force refresh themes (Admin)"
              >
                <svg className="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
                  />
                </svg>
              </button>
            )}

            {/* Close Button */}
            <button
              onClick={onClose}
              className="rounded-lg p-2 text-slate-400 transition-colors hover:bg-slate-700 hover:text-white"
              aria-label="Close theme marketplace"
            >
              <svg className="h-6 w-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M6 18L18 6M6 6l12 12"
                />
              </svg>
            </button>
          </div>
        </div>

        {/* Filters */}
        <div className="border-b border-slate-700 p-6">
          <ThemeFilters
            searchQuery={searchQuery}
            onSearchChange={setSearchQuery}
            availableTags={availableTags}
            selectedTags={selectedTags}
            onToggleTag={toggleTag}
            onClearFilters={clearFilters}
            hasActiveFilters={hasActiveFilters}
          />
        </div>

        {/* Content */}
        <div className="custom-scrollbar flex-1 overflow-y-auto p-6">
          {/* Loading State */}
          {loading && (
            <div className="flex items-center justify-center py-12">
              <div className="text-center">
                <div className="mx-auto mb-4 h-12 w-12 animate-spin rounded-full border-b-2 border-purple-500" />
                <p className="text-slate-400">Loading themes...</p>
              </div>
            </div>
          )}

          {/* Error State */}
          {error && !loading && (
            <div className="mb-4 rounded-lg border border-yellow-500/30 bg-yellow-500/10 p-4">
              <div className="flex items-start gap-3">
                <svg
                  className="h-6 w-6 flex-shrink-0 text-yellow-500"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                  />
                </svg>
                <div>
                  <p className="font-semibold text-yellow-400">Error Loading Themes</p>
                  <p className="text-sm text-yellow-300/80">{error}</p>
                </div>
              </div>
            </div>
          )}

          {/* Empty State */}
          {!loading && themes.length === 0 && !error && (
            <div className="py-12 text-center">
              <svg
                className="mx-auto mb-4 h-16 w-16 text-slate-600"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M9.172 16.172a4 4 0 015.656 0M9 10h.01M15 10h.01M12 12h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
                />
              </svg>
              <p className="text-lg text-slate-400">No themes found</p>
              <p className="mt-1 text-sm text-slate-500">Try adjusting your filters</p>
            </div>
          )}

          {/* Theme Grid */}
          {!loading && themes.length > 0 && (
            <>
              {/* Count */}
              <div className="mb-4 text-sm text-slate-400">
                Showing {filteredCount} of {totalCount} themes
              </div>

              {/* Grid */}
              <div className="grid grid-cols-1 gap-6 md:grid-cols-2 lg:grid-cols-3">
                {themes.map((theme) => (
                  <ThemeCard
                    key={theme.id}
                    theme={theme}
                    isFavorite={favorites.includes(theme.id)}
                    messages={SAMPLE_PREVIEW_MESSAGES}
                    onToggleFavorite={toggleFavorite}
                    onApply={handleApply}
                    themeType={themeType}
                  />
                ))}
              </div>
            </>
          )}
        </div>
      </div>
    </div>
  )
}
