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
 * Poll Overlay Widget (Unauthenticated OBS Browser Source) — issue #523.
 *
 * Polls the public engagement endpoint and renders the active poll as live bars.
 * Transparent background for OBS. Works for All-Chat-native and (once mirrored)
 * Twitch-native polls alike — it renders whatever aggregate the API returns, so
 * no viewer identity is involved. Hidden when no poll is active.
 */

'use client'

import { use, useCallback, useEffect, useState } from 'react'
import type { Poll } from '@/lib/types/engagement'

const POLL_INTERVAL_MS = 2000

export default function PollOverlayPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params)
  const [poll, setPoll] = useState<Poll | null>(null)
  const [now, setNow] = useState(() => Date.now())

  const fetchPoll = useCallback(async () => {
    try {
      const res = await fetch(`/api/v1/engagement/overlays/${id}/active-poll`, { cache: 'no-store' })
      if (res.status === 404) {
        setPoll(null)
        return
      }
      if (!res.ok) return
      setPoll((await res.json()) as Poll)
    } catch {
      // Transient network/OBS hiccup — keep the last render until the next tick.
    }
  }, [id])

  useEffect(() => {
    // Defer the first fetch out of the effect body (its setState is not synchronous).
    const first = setTimeout(() => void fetchPoll(), 0)
    const t = setInterval(() => void fetchPoll(), POLL_INTERVAL_MS)
    return () => {
      clearTimeout(first)
      clearInterval(t)
    }
  }, [fetchPoll])

  // 1s ticker so the "remaining" countdown updates smoothly between fetches.
  useEffect(() => {
    const t = setInterval(() => setNow(Date.now()), 1000)
    return () => clearInterval(t)
  }, [])

  if (!poll || poll.state !== 'ACTIVE') return null

  const total = poll.options.reduce((sum, o) => sum + o.votes, 0)
  const remaining = poll.ends_at ? Math.max(0, Math.floor((new Date(poll.ends_at).getTime() - now) / 1000)) : null

  return (
    <div className="min-h-screen bg-transparent p-4">
      <div className="mx-auto max-w-md rounded-xl bg-black/70 p-4 text-white shadow-lg backdrop-blur-sm">
        <div className="mb-3 flex items-center gap-2 text-lg font-bold">
          <span aria-hidden>📊</span>
          <span>{poll.question}</span>
        </div>
        <div className="space-y-2">
          {poll.options.map((o) => {
            const pct = total > 0 ? Math.round((o.votes / total) * 100) : 0
            return (
              <div key={o.id} className="relative h-8 overflow-hidden rounded-md bg-white/15">
                <div
                  className="absolute inset-y-0 left-0 bg-purple-500/70 transition-[width] duration-500"
                  style={{ width: `${pct}%` }}
                />
                <div className="relative flex h-full items-center justify-between px-3 text-sm font-semibold">
                  <span className="truncate">{o.label}</span>
                  <span className="tabular-nums">
                    {pct}% ({o.votes.toLocaleString()})
                  </span>
                </div>
              </div>
            )
          })}
        </div>
        {remaining !== null && (
          <div className="mt-3 text-right text-sm text-white/80 tabular-nums">
            ⏱ {Math.floor(remaining / 60)}:{String(remaining % 60).padStart(2, '0')} remaining
          </div>
        )}
      </div>
    </div>
  )
}
