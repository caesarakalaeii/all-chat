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

import { useState } from 'react'

import { useTranslations } from '@/lib/i18n'

/**
 * Marks a Twitch shared-chat message with the channel it came from.
 *
 * The origin channel's avatar says WHICH channel relayed the message, which a "shared"
 * pill does not, in roughly the width of one emote. The pill is still the fallback: the
 * enricher leaves `source_avatar_url` unset when the Helix lookup fails or the channel
 * has no picture, and older messages predate it entirely.
 */
export function SharedChatOrigin({
  avatarUrl,
  displayName,
  size = '14px',
}: {
  avatarUrl?: string
  displayName?: string
  /**
   * Any CSS length. The default puts the avatar on the chat panel's badge-row
   * scale, about one emote of width; the OBS overlay passes `1em` because the
   * streamer configures that row's font size and a fixed pixel box looks wrong
   * at both ends of the range.
   */
  size?: string
}) {
  const t = useTranslations()

  // Tracking the failed URL rather than a boolean resets the fallback when this row is
  // reused for a different origin channel — the pattern UserAvatar uses for dead TikTok
  // CDN URLs (PR #555).
  const [failedUrl, setFailedUrl] = useState<string | null>(null)

  if (!avatarUrl || failedUrl === avatarUrl) {
    return (
      <span className="ml-1 rounded bg-twitch/20 px-1 text-[10px] font-medium text-twitch uppercase">
        {t('viewerOverlay.chatPanel.sharedBadge')}
      </span>
    )
  }

  const label = displayName || t('viewerOverlay.chatPanel.sharedBadge')
  return (
    // A plain img, not next/image: the URL comes from Twitch's API, and next/image
    // throws on any host missing from next.config.js images.domains — an unexpected CDN
    // would take the row down instead of falling back to the pill below.
    // eslint-disable-next-line @next/next/no-img-element
    <img
      src={avatarUrl}
      alt={label}
      title={label}
      // Inline, not Tailwind: an arbitrary value has to be literal in the class name,
      // so h-[${size}] would compile to no CSS at all.
      style={{ width: size, height: size }}
      referrerPolicy="no-referrer"
      onError={() => setFailedUrl(avatarUrl)}
      className="ml-1 inline-block shrink-0 rounded-full object-cover align-text-bottom"
    />
  )
}
