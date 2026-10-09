/**
 * This file is part of All-Chat.
 * Copyright (C) 2026 caesarakalaeii
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published
 * by the Free Software Foundation, either version 3 of the License, or
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
 *
 * Lanes restyle: a .lanes-section with black panels, hard white borders and
 * offset shadows instead of the rounded shadcn cards.
 */

'use client'

import { useEffect, useState } from 'react'
import { UserAvatar } from './UserAvatar'
import { ChannelLink } from './ChannelLink'
import { PlatformBadge } from './ui/badge'
import { useTranslations } from '@/lib/i18n'

interface Ambassador {
  username: string
  display_name: string
  avatar_url: string
  platform: string
  tagline: string | null
}

export function FeaturedAmbassadors() {
  const t = useTranslations()
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
    <section className="lanes-section ambassadors" data-reveal>
      <span className="mono-label">{t('marketing.ambassadors.eyebrow')}</span>
      <h2>{t('marketing.ambassadors.title')}</h2>
      <ul className="ambassador-grid">
        {ambassadors.map((a) => (
          <li key={a.username} className="ambassador-card panel-fill">
            <UserAvatar avatarUrl={a.avatar_url} displayName={a.display_name} size={48} />
            <div className="min-w-0">
              <div className="flex items-center gap-2">
                <span className="truncate font-bold">{a.display_name}</span>
                <PlatformBadge platform={a.platform} size="sm" />
              </div>
              {a.tagline && <p className="mt-1 text-sm dim">{a.tagline}</p>}
              <ChannelLink
                platform={a.platform}
                channelId={a.username}
                channelHandle={a.username}
                className="mt-2 text-sm dim"
              />
            </div>
          </li>
        ))}
      </ul>
    </section>
  )
}
