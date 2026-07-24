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
 * Theme Card Component
 *
 * Displays a single theme with preview, metadata, and actions.
 */

'use client'

import clsx from 'clsx'
import type { Theme, ChatMessagePreview } from '@/lib/theme-marketplace/types'
import ThemePreview from './ThemePreview'
import CreditRollThemePreview from './CreditRollThemePreview'

interface ThemeCardProps {
  theme: Theme
  isFavorite: boolean
  messages: ChatMessagePreview[]
  onToggleFavorite: (themeId: string) => void
  onApply: (theme: Theme) => void
  themeType?: 'overlay' | 'creditroll'
}

/** Max tag pills shown on a card before collapsing the rest into a +N chip. */
const MAX_VISIBLE_TAGS = 3

/**
 * Get tag color based on tag name
 */
function getTagColor(tag: string): string {
  const colors: Record<string, string> = {
    minimal: 'bg-blue-500/20 text-blue-400 border-blue-500/30',
    clean: 'bg-cyan-500/20 text-cyan-400 border-cyan-500/30',
    retro: 'bg-orange-500/20 text-orange-400 border-orange-500/30',
    nostalgic: 'bg-pink-500/20 text-pink-400 border-pink-500/30',
    dark: 'bg-neutral-600/20 text-neutral-400 border-neutral-600/30',
    neon: 'bg-purple-500/20 text-purple-400 border-purple-500/30',
    '90s': 'bg-pink-500/20 text-pink-400 border-pink-500/30',
    inline: 'bg-green-500/20 text-green-400 border-green-500/30',
    simple: 'bg-teal-500/20 text-teal-400 border-teal-500/30',
  }

  return colors[tag.toLowerCase()] || 'bg-neutral-600/20 text-neutral-400 border-neutral-600/30'
}

export default function ThemeCard({
  theme,
  isFavorite,
  messages,
  onToggleFavorite,
  onApply,
  themeType = 'overlay',
}: ThemeCardProps) {
  return (
    <div className="theme-card flex flex-col overflow-hidden rounded-lg border border-border bg-surface transition-all duration-200 hover:-translate-y-1 hover:border-twitch/50 hover:shadow-lg">
      {/* Preview */}
      {themeType === 'creditroll' ? (
        <CreditRollThemePreview css={theme.css} themeId={theme.id} />
      ) : (
        <ThemePreview css={theme.css} messages={messages} themeId={theme.id} />
      )}

      {/* Metadata */}
      <div className="flex flex-1 flex-col p-4">
        {/* Name and Favorite */}
        <div className="mb-2 flex items-start justify-between gap-2">
          <h3 className="flex-1 text-base leading-tight font-semibold text-text">{theme.name}</h3>
          <button
            onClick={() => onToggleFavorite(theme.id)}
            className="flex-shrink-0 text-xl transition-transform hover:scale-110"
            aria-label={isFavorite ? 'Remove from favorites' : 'Add to favorites'}
            title={isFavorite ? 'Remove from favorites' : 'Add to favorites'}
          >
            {isFavorite ? '⭐' : '☆'}
          </button>
        </div>

        {/* Description */}
        <p className="mb-3 line-clamp-2 text-xs text-text-sub">{theme.description}</p>

        {/* Tags — capped to keep cards scannable; overflow collapses to a
            +N chip that lists the rest on hover (WYSIWYG search still matches
            every tag). */}
        <div className="mb-4 flex flex-wrap gap-1.5">
          {theme.tags.slice(0, MAX_VISIBLE_TAGS).map((tag) => (
            <span
              key={tag}
              className={clsx('rounded border px-1.5 py-0.5 text-xs', getTagColor(tag))}
            >
              {tag}
            </span>
          ))}
          {theme.tags.length > MAX_VISIBLE_TAGS && (
            <span
              className="rounded border border-border px-1.5 py-0.5 text-xs text-text-sub"
              title={theme.tags.slice(MAX_VISIBLE_TAGS).join(', ')}
            >
              +{theme.tags.length - MAX_VISIBLE_TAGS}
            </span>
          )}
        </div>

        {/* Apply Button */}
        <button
          onClick={() => onApply(theme)}
          className="mt-auto flex w-full items-center justify-center gap-2 rounded-lg bg-twitch px-4 py-2 text-sm font-semibold text-bg transition-colors hover:bg-twitch/90"
        >
          <svg className="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
          </svg>
          Apply Theme
        </button>
      </div>
    </div>
  )
}
