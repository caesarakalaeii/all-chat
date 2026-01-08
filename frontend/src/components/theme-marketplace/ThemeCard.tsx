/**
 * Theme Card Component
 *
 * Displays a single theme with preview, metadata, and actions.
 */

'use client';

import type { Theme, ChatMessagePreview } from '@/lib/theme-marketplace/types';
import ThemePreview from './ThemePreview';

interface ThemeCardProps {
  theme: Theme;
  isFavorite: boolean;
  messages: ChatMessagePreview[];
  onToggleFavorite: (themeId: string) => void;
  onApply: (css: string) => void;
}

/**
 * Get tag color based on tag name
 */
function getTagColor(tag: string): string {
  const colors: Record<string, string> = {
    minimal: 'bg-blue-500/20 text-blue-400 border-blue-500/30',
    clean: 'bg-cyan-500/20 text-cyan-400 border-cyan-500/30',
    retro: 'bg-orange-500/20 text-orange-400 border-orange-500/30',
    nostalgic: 'bg-pink-500/20 text-pink-400 border-pink-500/30',
    dark: 'bg-gray-500/20 text-gray-400 border-gray-500/30',
    neon: 'bg-purple-500/20 text-purple-400 border-purple-500/30',
    classic: 'bg-yellow-500/20 text-yellow-400 border-yellow-500/30',
    '90s': 'bg-pink-500/20 text-pink-400 border-pink-500/30',
    inline: 'bg-green-500/20 text-green-400 border-green-500/30',
    simple: 'bg-teal-500/20 text-teal-400 border-teal-500/30',
  };

  return (
    colors[tag.toLowerCase()] ||
    'bg-gray-500/20 text-gray-400 border-gray-500/30'
  );
}

export default function ThemeCard({
  theme,
  isFavorite,
  messages,
  onToggleFavorite,
  onApply,
}: ThemeCardProps) {
  return (
    <div
      className="theme-card bg-gray-900 border border-gray-700 rounded-lg overflow-hidden
                 hover:border-purple-500/50 hover:shadow-lg
                 transition-all duration-200 hover:-translate-y-1 flex flex-col"
    >
      {/* Preview */}
      <ThemePreview css={theme.css} messages={messages} themeId={theme.id} />

      {/* Metadata */}
      <div className="p-4 flex-1 flex flex-col">
        {/* Name and Favorite */}
        <div className="flex items-start justify-between gap-2 mb-2">
          <h3 className="text-white font-semibold text-lg leading-tight flex-1">
            {theme.name}
          </h3>
          <button
            onClick={() => onToggleFavorite(theme.id)}
            className="flex-shrink-0 text-2xl hover:scale-110 transition-transform"
            aria-label={
              isFavorite ? 'Remove from favorites' : 'Add to favorites'
            }
            title={isFavorite ? 'Remove from favorites' : 'Add to favorites'}
          >
            {isFavorite ? '⭐' : '☆'}
          </button>
        </div>

        {/* Description */}
        <p className="text-gray-400 text-sm mb-3 line-clamp-2">
          {theme.description}
        </p>

        {/* Tags */}
        <div className="flex flex-wrap gap-2 mb-4">
          {theme.tags.map((tag) => (
            <span
              key={tag}
              className={`text-xs px-2 py-1 rounded border ${getTagColor(
                tag
              )}`}
            >
              {tag}
            </span>
          ))}
        </div>

        {/* Apply Button */}
        <button
          onClick={() => onApply(theme.css)}
          className="mt-auto w-full bg-purple-600 hover:bg-purple-700 text-white
                     font-semibold py-2 px-4 rounded-lg transition-colors
                     flex items-center justify-center gap-2"
        >
          <svg
            className="w-5 h-5"
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M5 13l4 4L19 7"
            />
          </svg>
          Apply Theme
        </button>
      </div>
    </div>
  );
}
