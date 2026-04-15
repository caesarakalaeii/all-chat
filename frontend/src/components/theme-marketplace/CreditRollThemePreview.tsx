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
 * Credit Roll Theme Preview Component
 *
 * Shows a preview of what the credit roll will look like with the theme applied.
 * Displays sample leaderboard entries instead of chat messages.
 */

'use client'

import { useMemo } from 'react'
import clsx from 'clsx'

interface CreditRollThemePreviewProps {
  css: string
  themeId?: string
}

const SAMPLE_LEADERBOARD_DATA = [
  {
    rank: 1,
    display_name: 'TopSupporter',
    platform: 'twitch',
    avatar_url: 'https://static-cdn.jtvnw.net/jtv_user_pictures/aaa-profile_image-70x70.png',
    total_value: 500.0,
  },
  {
    rank: 2,
    display_name: 'GenerousViewer',
    platform: 'youtube',
    avatar_url: 'https://yt3.ggpht.com/a/default-user=s88-c-k-c0x00ffffff-no-rj',
    total_value: 250.0,
  },
  {
    rank: 3,
    display_name: 'AwesomeFan',
    platform: 'kick',
    avatar_url: 'https://static-cdn.jtvnw.net/jtv_user_pictures/bbb-profile_image-70x70.png',
    total_value: 100.0,
  },
]

export default function CreditRollThemePreview({
  css,
  themeId = 'preview',
}: CreditRollThemePreviewProps) {
  // Create unique scope ID for this preview
  const scopeId = `credit-roll-preview-${themeId.replace(/[^a-z0-9]/gi, '-')}`

  // Scope CSS using Shadow DOM approach - wrap all selectors with unique ID
  const scopedCss = useMemo(() => {
    return css
      .split('\n')
      .map((line) => {
        const trimmed = line.trim()

        // Keep @import and @font-face as-is
        if (trimmed.startsWith('@import') || trimmed.startsWith('@font-face')) {
          return line
        }

        // Handle @keyframes
        if (trimmed.startsWith('@keyframes')) {
          const name = trimmed.match(/@keyframes\s+([^\s{]+)/)?.[1]
          if (name) {
            return line.replace(name, `${scopeId}-${name}`)
          }
          return line
        }

        // Handle animation references
        if (trimmed.includes('animation:') && !trimmed.startsWith('@')) {
          // Replace animation names with scoped versions
          return line.replace(/animation:\s*([^\s]+)/g, (match, animName) => {
            return `animation: ${scopeId}-${animName}`
          })
        }

        // Scope regular CSS rules
        if (trimmed.includes('{') && !trimmed.startsWith('@') && !trimmed.startsWith('/*')) {
          const selectorEnd = line.indexOf('{')
          const selector = line.substring(0, selectorEnd).trim()
          const rest = line.substring(selectorEnd)

          if (selector) {
            // Replace body with scoped class
            const scopedSelector = selector
              .replace(/\bbody\b/g, `#${scopeId}`)
              .split(',')
              .map((s) => `#${scopeId} ${s.trim()}`)
              .join(', ')

            return scopedSelector + rest
          }
        }

        return line
      })
      .join('\n')
  }, [css, scopeId])

  return (
    <div id={scopeId} className="relative h-full w-full overflow-hidden">
      {/* Inject scoped theme CSS */}
      <style dangerouslySetInnerHTML={{ __html: scopedCss }} />

      {/* Credit roll preview container */}
      <div className="min-h-full overflow-y-auto bg-linear-to-b from-surface to-bg p-4">
        {/* Header */}
        <div className="mb-6 text-center">
          <h1 className="mb-2 text-3xl font-bold text-white">🎬 Stream Credits</h1>
          <p className="text-sm text-text-sub">Thank you for your support!</p>
        </div>

        {/* Sample Leaderboard */}
        <div className="mx-auto max-w-md">
          <h2 className="mb-4 flex items-center gap-2 text-2xl font-bold text-white">
            <span className="text-3xl">⭐</span>
            Top Subscribers
          </h2>
          <div className="space-y-4">
            {SAMPLE_LEADERBOARD_DATA.map((entry) => (
              <div
                key={entry.rank}
                className={clsx(
                  'flex items-center gap-4 rounded-lg p-4',
                  entry.rank === 1 && 'border-2 border-yellow-500 bg-yellow-500/20',
                  entry.rank === 2 && 'border-2 border-neutral-200 bg-neutral-200/20',
                  entry.rank === 3 && 'border-2 border-orange-600 bg-orange-600/20',
                  entry.rank > 3 && 'border border-border bg-surface/50'
                )}
              >
                <div
                  className={clsx(
                    'w-12 text-center text-3xl font-bold',
                    entry.rank === 1 && 'text-yellow-400',
                    entry.rank === 2 && 'text-neutral-200',
                    entry.rank === 3 && 'text-orange-500',
                    entry.rank > 3 && 'text-text-dim'
                  )}
                >
                  #{entry.rank}
                </div>
                <div className="relative h-12 w-12 overflow-hidden rounded-full bg-surface-2">
                  {/* eslint-disable-next-line @next/next/no-img-element */}
                  <img
                    src={entry.avatar_url}
                    alt={entry.display_name}
                    className="h-full w-full object-cover"
                    onError={(e) => {
                      ;(e.target as HTMLImageElement).src =
                        'data:image/svg+xml,%3Csvg xmlns="http://www.w3.org/2000/svg" width="48" height="48"%3E%3Crect fill="%23374151" width="48" height="48"/%3E%3C/svg%3E'
                    }}
                  />
                </div>
                <div className="flex-1">
                  <div className="text-xl font-semibold text-white">{entry.display_name}</div>
                  <div className="text-sm text-text-sub capitalize">{entry.platform}</div>
                </div>
                <div className="text-2xl font-bold text-white">${entry.total_value.toFixed(2)}</div>
              </div>
            ))}
          </div>
        </div>

        {/* Footer */}
        <div className="mt-8 text-center">
          <div className="mb-2 text-2xl font-bold text-white">Thank you! ❤️</div>
          <p className="text-sm text-text-sub">See you next stream!</p>
        </div>
      </div>
    </div>
  )
}
