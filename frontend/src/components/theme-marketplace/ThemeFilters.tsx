/**
 * Theme Filters Component
 *
 * Search bar and tag filter pills.
 */

'use client';

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
            className="w-full bg-gray-700 border border-gray-600 rounded-lg
                       px-4 py-2 pl-10 text-white placeholder-gray-400
                       focus:outline-none focus:border-purple-500 focus:ring-1
                       focus:ring-purple-500 transition-colors"
            aria-label="Search themes"
          />
          <svg
            className="absolute left-3 top-1/2 -translate-y-1/2 w-5 h-5 text-gray-400"
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
            className="px-4 py-2 bg-gray-700 hover:bg-gray-600 text-white
                       rounded-lg transition-colors font-medium whitespace-nowrap"
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
                className={`px-3 py-1 rounded-full text-sm font-medium
                           transition-colors border ${
                             isSelected
                               ? 'bg-purple-600 text-white border-purple-500'
                               : 'bg-gray-700 text-gray-300 border-gray-600 hover:bg-gray-600'
                           }`}
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
