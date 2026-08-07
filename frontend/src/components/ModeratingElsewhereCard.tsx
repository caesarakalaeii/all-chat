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

'use client'

/**
 * Dashboard entry point to the channels this user moderates for other people (ADR-0048).
 *
 * The dashboard lists overlays a user OWNS, and there is no other listing that could
 * surface someone else's — so without a pointer here, a volunteer who accepted an invite
 * last week has no way back to it except the original invite link, which is single-use.
 *
 * It renders nothing at all when there is nothing to point at, including on error: a
 * streamer who moderates for nobody must not grow an empty section, and a failed request
 * must not turn into a broken one.
 */

import { useEffect, useState } from 'react'
import Link from 'next/link'
import { ShieldCheck } from 'lucide-react'

import { moderationApi } from '@/lib/api/moderation'

export function ModeratingElsewhereCard() {
  const [count, setCount] = useState(0)

  // Set from the promise callback, not the effect body: a synchronous setState in an
  // effect cascades renders. Failures stay silent — this is a shortcut, not a feature.
  useEffect(() => {
    let cancelled = false
    moderationApi
      .listDelegations()
      .then((list) => {
        if (!cancelled) setCount(list.delegations.length)
      })
      .catch(() => {
        /* no shortcut this load; the /moderate page still works if they know it */
      })
    return () => {
      cancelled = true
    }
  }, [])

  if (count === 0) return null

  return (
    <Link
      href="/moderate"
      className="flex items-center gap-3 rounded-xl border border-border bg-surface px-4 py-3 text-sm transition-colors hover:border-border-md focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
    >
      <ShieldCheck className="size-5 shrink-0 text-twitch" aria-hidden="true" />
      <span className="text-text-sub">
        You moderate{' '}
        <span className="font-semibold text-text">
          {count} {count === 1 ? 'channel' : 'channels'}
        </span>{' '}
        for other streamers.
      </span>
      <span className="ml-auto font-medium text-twitch">Open</span>
    </Link>
  )
}
