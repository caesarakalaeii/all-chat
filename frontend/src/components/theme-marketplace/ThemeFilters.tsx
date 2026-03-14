/**
 * Theme Filters Component
 *
 * Search bar and tag filter pills.
 */

'use client';

import clsx from 'clsx';

interface ThemeFiltersProps {
  searchQuery: string;
  onSearchChange: (query: string) => void;
  availableTags: string[];
  selectedTags: string[];
  onToggleTag: (tag: string) => void;
  onClearFilters: () => void;
  hasActiveFilters: boolean;
}

export default function ThemeFilters({
  searchQuery,
  onSearchChange,
  availableTags,
  selectedTags,
  onToggleTag,
  onClearFilters,
  hasActiveFilters,
}: ThemeFiltersProps) {
  return (
    <div className="space-y-3">
      {/* Search Bar */}
      <div className="flex gap-2">
        <div className="relative flex-1">
          <input
            type="search"
            value={searchQuery}
            onChange={(e) => onSearchChange(e.target.value)}
            placeholder="Search themes..."
            className="w-full rounded-lg border border-slate-600 bg-slate-700
                       px-4 py-2 pl-10 text-white placeholder-slate-400
                       transition-colors focus-visible:border-purple-500 focus-visible:outline-none
                       focus-visible:ring-1 focus-visible:ring-purple-500"
            aria-label="Search themes"
          />
          <svg
            className="absolute left-3 top-1/2 h-5 w-5 -translate-y-1/2 text-slate-400"
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"
            />
          </svg>
        </div>

        {/* Clear Filters Button */}
        {hasActiveFilters && (
          <button
            onClick={onClearFilters}
            className="whitespace-nowrap rounded-lg bg-slate-700 px-4 py-2 font-medium text-white transition-colors hover:bg-slate-600"
          >
            Clear Filters
          </button>
        )}
      </div>

      {/* Tag Pills */}
      {availableTags.length > 0 && (
        <div className="flex flex-wrap gap-2">
          {availableTags.map((tag) => {
            const isSelected = selectedTags.includes(tag);
            return (
              <button
                key={tag}
                onClick={() => onToggleTag(tag)}
                className={clsx(
                  'rounded-full border px-3 py-1 text-sm font-medium transition-colors',
                  isSelected
                    ? 'border-purple-500 bg-purple-600 text-white'
                    : 'border-slate-600 bg-slate-700 text-slate-300 hover:bg-slate-600'
                )}
              >
                {tag}
              </button>
            );
          })}
        </div>
      )}
    </div>
  );
}
