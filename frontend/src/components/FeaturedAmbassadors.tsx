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
 * Featured Ambassadors (ADR-0041)
 *
 * Social-proof band for the marketing homepage: the streamers an admin has made
 * ambassadors AND who have opted in to being shown. Fetches the public
 * `/api/v1/ambassadors` endpoint with a raw `fetch` + silent catch, mirroring the
 * decorative `/api/v1/stats` call in HomeClient — the landing page deliberately
 * avoids the apiClient refresh/redirect machinery. Renders nothing until data
 * arrives and nothing when the list is empty, so the section never shows an empty
 * shell.
 */

'use client'

import { useEffect, useState } from 'react'
import { UserAvatar } from './UserAvatar'
import { ChannelLink } from './ChannelLink'
import { PlatformBadge } from './ui/badge'

interface Ambassador {
  username: string
  display_name: string
  avatar_url: string
  platform: string
  tagline: string | null
}

export function FeaturedAmbassadors() {
  const [ambassadors, setAmbassadors] = useState<Ambassador[] | null>(null)

  useEffect(() => {
    fetch('/api/v1/ambassadors')
      .then((r) => (r.ok ? r.json() : null))
      .then((data) => {
        if (Array.isArray(data)) setAmbassadors(data as Ambassador[])
      })
      .catch(() => {}) // fail silently — the showcase is decorative
  }, [])

  // Nothing to show (still loading, request failed, or no opted-in ambassadors):
  // render no section at all rather than an empty band.
  if (!ambassadors || ambassadors.length === 0) return null

  return (
    <section className="mx-auto max-w-5xl border-t border-border px-4 py-16">
      <p className="mb-3 text-center text-xs font-bold tracking-widest text-text-sub uppercase">
        Ambassadors
      </p>
      <h2 className="mb-10 text-center text-2xl font-bold text-text">
        Streamers who run on All-Chat
      </h2>
      <ul className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {ambassadors.map((a) => (
          <li
            key={a.username}
            className="flex items-start gap-4 rounded-xl border border-border bg-surface p-5"
          >
            <UserAvatar avatarUrl={a.avatar_url} displayName={a.display_name} size={48} />
            <div className="min-w-0">
              <div className="flex items-center gap-2">
                <span className="truncate font-semibold text-text">{a.display_name}</span>
                <PlatformBadge platform={a.platform} size="sm" />
              </div>
              {a.tagline && <p className="mt-1 text-sm text-text-sub">{a.tagline}</p>}
              <ChannelLink
                platform={a.platform}
                channelId={a.username}
                channelHandle={a.username}
                className="mt-2 text-sm text-text-sub"
              />
            </div>
          </li>
        ))}
      </ul>
    </section>
  )
}
