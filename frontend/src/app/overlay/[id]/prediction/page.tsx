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
 * Prediction Overlay Widget (Unauthenticated OBS Browser Source) — issue #523.
 *
 * Polls the public engagement endpoint and renders the active prediction: the
 * wagered pool per outcome, a combined bar, and the live state (open / locked /
 * resolved). Transparent background for OBS; no viewer identity involved.
 */

'use client'

import { use, useCallback, useEffect, useState } from 'react'
import clsx from 'clsx'
import type { Prediction } from '@/lib/types/engagement'
import { useEngagementLive } from '@/lib/hooks/useEngagementLive'

const POLL_INTERVAL_MS = 2000

const OUTCOME_COLORS = ['bg-sky-500/70', 'bg-pink-500/70', 'bg-amber-500/70', 'bg-emerald-500/70']

export default function PredictionOverlayPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params)
  const [prediction, setPrediction] = useState<Prediction | null>(null)

  const fetchPrediction = useCallback(async () => {
    try {
      const res = await fetch(`/api/v1/engagement/overlays/${id}/active-prediction`, { cache: 'no-store' })
      if (res.status === 404) {
        setPrediction(null)
        return
      }
      if (!res.ok) return
      setPrediction((await res.json()) as Prediction)
    } catch {
      // Transient hiccup — keep the last render.
    }
  }, [id])

  useEffect(() => {
    // Defer the first fetch out of the effect body (its setState is not synchronous).
    const first = setTimeout(() => void fetchPrediction(), 0)
    const t = setInterval(() => void fetchPrediction(), POLL_INTERVAL_MS)
    return () => {
      clearTimeout(first)
      clearInterval(t)
    }
  }, [fetchPrediction])

  // Near-real-time refresh on a prediction_update WS frame (L-D1); the 2s interval
  // above remains the fallback / source of truth.
  useEngagementLive(id, (kind) => {
    if (kind === 'prediction') void fetchPrediction()
  })

  // RESOLVED/CANCELED predictions are briefly still returned by /active; hide once gone.
  if (!prediction || (prediction.state !== 'ACTIVE' && prediction.state !== 'LOCKED' && prediction.state !== 'RESOLVED')) {
    return null
  }

  const totalPool = prediction.outcomes.reduce((sum, o) => sum + o.total_points, 0)
  const stateBadge =
    prediction.state === 'LOCKED' ? '🔒 LOCKED' : prediction.state === 'RESOLVED' ? '🏆 RESOLVED' : 'OPEN'

  return (
    <div className="min-h-screen bg-transparent p-4">
      <div className="mx-auto max-w-md rounded-xl bg-black/70 p-4 text-white shadow-lg backdrop-blur-sm">
        <div className="mb-3 flex items-center gap-2 text-lg font-bold">
          <span aria-hidden>🔮</span>
          <span>{prediction.title}</span>
        </div>
        <div className="space-y-2">
          {prediction.outcomes.map((o, i) => {
            const pct = totalPool > 0 ? Math.round((o.total_points / totalPool) * 100) : 0
            const isWinner = prediction.state === 'RESOLVED' && prediction.winning_outcome_id === o.id
            return (
              <div
                key={o.id}
                className={clsx('relative h-9 overflow-hidden rounded-md bg-white/15', isWinner && 'ring-2 ring-yellow-400')}
              >
                <div
                  className={clsx('absolute inset-y-0 left-0 transition-[width] duration-500', OUTCOME_COLORS[i % OUTCOME_COLORS.length])}
                  style={{ width: `${pct}%` }}
                />
                <div className="relative flex h-full items-center justify-between px-3 text-sm font-semibold">
                  <span className="flex min-w-0 items-center gap-1.5">
                    <span className="truncate">{o.idx}. {o.label}</span>
                    {/* Winner conveyed by a labelled pill (not colour/emoji alone) so it
                        has an accessible name and reads for colourblind viewers (L-A1). */}
                    {isWinner && (
                      <span className="shrink-0 rounded bg-yellow-400 px-1.5 py-0.5 text-[10px] font-bold uppercase text-black">
                        Winner
                      </span>
                    )}
                  </span>
                  <span className="tabular-nums">
                    {o.total_points.toLocaleString()} pts · {pct}%
                  </span>
                </div>
              </div>
            )
          })}
        </div>
        <div className="mt-3 flex items-center justify-between text-sm text-white/80">
          <span className="tabular-nums">{totalPool.toLocaleString()} pts wagered · {prediction.outcomes.reduce((s, o) => s + o.entrants, 0)} players</span>
          <span className="font-semibold">{stateBadge}</span>
        </div>
      </div>
    </div>
  )
}
