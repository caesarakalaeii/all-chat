/**
 * Theme Marketplace Modal Component
 *
 * Main modal container for browsing and applying themes.
 */

'use client';

import { useEffect } from 'react';
import ThemeCard from './ThemeCard';
import ThemeFilters from './ThemeFilters';
import { useThemeMarketplace } from '@/hooks/useThemeMarketplace';
import { SAMPLE_PREVIEW_MESSAGES } from '@/lib/theme-marketplace/constants';

interface ThemeMarketplaceModalProps {
  isOpen: boolean;
  onClose: () => void;
  onApplyTheme: (css: string) => void;
}

export default function ThemeMarketplaceModal({
  isOpen,
  onClose,
  onApplyTheme,
}: ThemeMarketplaceModalProps) {
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
  } = useThemeMarketplace();

  // Handle ESC key to close modal
  useEffect(() => {
    if (!isOpen) return;

    const handleEscape = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        onClose();
      }
    };

    document.addEventListener('keydown', handleEscape);
    return () => document.removeEventListener('keydown', handleEscape);
  }, [isOpen, onClose]);

  // Prevent body scroll when modal is open
  useEffect(() => {
    if (isOpen) {
      document.body.style.overflow = 'hidden';
    } else {
      document.body.style.overflow = '';
    }
    return () => {
      document.body.style.overflow = '';
    };
  }, [isOpen]);

  if (!isOpen) return null;

  const handleApply = (css: string) => {
    onApplyTheme(css);
    onClose();
  };

  return (
    <div
      className="fixed inset-0 bg-black/50 backdrop-blur-sm flex items-center justify-center z-50 p-4"
      onClick={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div
        className="bg-gray-800 border border-gray-700 rounded-lg max-w-6xl w-full
                   max-h-[90vh] flex flex-col shadow-2xl"
      >
        {/* Header */}
        <div className="flex items-center justify-between p-6 border-b border-gray-700">
          <div>
            <h2
              id="theme-marketplace-title"
              className="text-2xl font-bold text-white"
            >
              Theme Marketplace
            </h2>
            <p className="text-gray-400 text-sm mt-1">
              Browse and apply custom CSS themes for your overlay
            </p>
          </div>
          <button
            onClick={onClose}
            className="text-gray-400 hover:text-white transition-colors p-2
                       hover:bg-gray-700 rounded-lg"
            aria-label="Close theme marketplace"
          >
            <svg
              className="w-6 h-6"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M6 18L18 6M6 6l12 12"
              />
            </svg>
          </button>
        </div>

        {/* Filters */}
        <div className="p-6 border-b border-gray-700">
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
        <div className="flex-1 overflow-y-auto p-6">
          {/* Loading State */}
          {loading && (
            <div className="flex items-center justify-center py-12">
              <div className="text-center">
                <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-purple-500 mx-auto mb-4" />
                <p className="text-gray-400">Loading themes...</p>
              </div>
            </div>
          )}

          {/* Error State */}
          {error && !loading && (
            <div className="bg-yellow-500/10 border border-yellow-500/30 rounded-lg p-4 mb-4">
              <div className="flex items-start gap-3">
                <svg
                  className="w-6 h-6 text-yellow-500 flex-shrink-0"
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
                  <p className="text-yellow-400 font-semibold">
                    Error Loading Themes
                  </p>
                  <p className="text-yellow-300/80 text-sm">{error}</p>
                </div>
              </div>
            </div>
          )}

          {/* Empty State */}
          {!loading && themes.length === 0 && !error && (
            <div className="text-center py-12">
              <svg
                className="w-16 h-16 text-gray-600 mx-auto mb-4"
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
              <p className="text-gray-400 text-lg">No themes found</p>
              <p className="text-gray-500 text-sm mt-1">
                Try adjusting your filters
              </p>
            </div>
          )}

          {/* Theme Grid */}
          {!loading && themes.length > 0 && (
            <>
              {/* Count */}
              <div className="mb-4 text-gray-400 text-sm">
                Showing {filteredCount} of {totalCount} themes
              </div>

              {/* Grid */}
              <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
                {themes.map((theme) => (
                  <ThemeCard
                    key={theme.id}
                    theme={theme}
                    isFavorite={favorites.includes(theme.id)}
                    messages={SAMPLE_PREVIEW_MESSAGES}
                    onToggleFavorite={toggleFavorite}
                    onApply={handleApply}
                  />
                ))}
              </div>
            </>
          )}
        </div>
      </div>
    </div>
  );
}
