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
 * Local PlatformBadge.
 *
 * Shares-domain restyle: renders a .lanes-chip in the platform color instead
 * of the shared rounded badge, whose old-token palette fights the lanes
 * direction. Text-only chip; platform identity is read from the label.
 */
import { cn } from '@/lib/utils'
import { PLATFORM_COLORS, type Platform } from '@/lib/platform-colors'

interface PlatformBadgeProps {
  source: {
    platform: string
    channel_name: string
  }
}

export function PlatformBadge({ source }: PlatformBadgeProps) {
  const knownPlatforms: Platform[] = ['twitch', 'youtube', 'kick', 'tiktok', 'system']
  const platform: Platform = knownPlatforms.includes(source.platform as Platform)
    ? (source.platform as Platform)
    : 'system'

  const colorClass = platform === 'system' ? 'text-sub' : PLATFORM_COLORS[platform].text

  return (
    <span
      data-slot="platform-badge"
      data-platform={source.platform}
      className={cn('lanes-chip', colorClass)}
    >
      {source.platform.replace(/_/g, ' ').toUpperCase()}
    </span>
  )
}
