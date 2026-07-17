'use client'

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

import { ExternalLink } from 'lucide-react'
import clsx from 'clsx'
import { channelUrl } from '@/lib/platform-channel-url'

/**
 * Renders a chat source's channel name as a link to its public page on the
 * origin platform (twitch.tv/…, kick.com/…, tiktok.com/@…, youtube.com/@…).
 * When no trustworthy URL can be built (discord, shared_overlay, ambiguous
 * YouTube ids) it falls back to plain text, so it is always safe to use.
 */
export function ChannelLink({
  platform,
  channelId,
  channelName,
  channelHandle,
  className,
}: {
  platform: string
  channelId: string
  channelName?: string
  channelHandle?: string | null
  className?: string
}) {
  const url = channelUrl(platform, channelId, channelHandle)
  const label = channelName || channelId || channelHandle || 'unknown'

  if (!url) {
    return <span className={className}>{label}</span>
  }

  return (
    <a
      href={url}
      target="_blank"
      rel="noopener noreferrer"
      aria-label={`Open ${label} on ${platform} (opens in a new tab)`}
      className={clsx('inline-flex items-center gap-1 hover:underline', className)}
    >
      {label}
      <ExternalLink aria-hidden="true" className="h-3 w-3 shrink-0 opacity-70" />
    </a>
  )
}
