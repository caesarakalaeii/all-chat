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
 * Local PlatformBadge re-export.
 *
 * Adapts the shares-domain `source` object shape to the shared
 * PlatformBadge component from @/components/ui/badge.
 */
import { PlatformBadge as SharedPlatformBadge } from '@/components/ui/badge'
import type { Platform } from '@/lib/platform-colors'

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

  return <SharedPlatformBadge platform={platform} />
}
