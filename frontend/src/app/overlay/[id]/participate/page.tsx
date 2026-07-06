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
 * Viewer participation page (issue #523) — the no-install, mobile-friendly way to
 * take part in polls/predictions without the extension. The viewer logs in with a
 * platform account (reusing the viewer OAuth→JWT flow), then votes, wagers points,
 * and sees their balance. Tier 2 of the participation model: chat commands are the
 * universal baseline, this page + the extension are the richer paths.
 */

'use client'

import { use, useCallback, useEffect, useState } from 'react'
import clsx from 'clsx'
import { viewerApi } from '@/lib/api/viewer'
import { inMemoryTokens } from '@/lib/auth/in-memory-store'
import { safeExternalRedirect } from '@/lib/auth/redirect-allowlist'
import type { Poll, Prediction, ViewerEngagement } from '@/lib/types/engagement'

const REFRESH_MS = 3000
const HEARTBEAT_MS = 60000
const PLATFORMS: ReadonlyArray<{ id: 'twitch' | 'youtube' | 'kick'; label: string }> = [
  { id: 'twitch', label: 'Twitch' },
  { id: 'youtube', label: 'YouTube' },
  { id: 'kick', label: 'Kick' },
]

function hasViewerToken(): boolean {
  if (typeof window === 'undefined') return false
  return Boolean(inMemoryTokens.getViewerAccessToken() ?? localStorage.getItem('viewer_jwt_token'))
}

export default function ParticipatePage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params)
  const [authed, setAuthed] = useState<boolean | null>(null)
  const [engagement, setEngagement] = useState<ViewerEngagement | null>(null)
  const [poll, setPoll] = useState<Poll | null>(null)
  const [prediction, setPrediction] = useState<Prediction | null>(null)
  const [wagerAmount, setWagerAmount] = useState('')
  const [notice, setNotice] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  const loadPublic = useCallback(async () => {
    try {
      const [pRes, prRes] = await Promise.all([
        fetch(`/api/v1/engagement/overlays/${id}/active-poll`, { cache: 'no-store' }),
        fetch(`/api/v1/engagement/overlays/${id}/active-prediction`, { cache: 'no-store' }),
      ])
      setPoll(pRes.ok ? ((await pRes.json()) as Poll) : null)
      setPrediction(prRes.ok ? ((await prRes.json()) as Prediction) : null)
    } catch {
      /* keep last render on transient errors */
    }
  }, [id])

  const loadPrivate = useCallback(async () => {
    try {
      setEngagement(await viewerApi.getEngagement(id))
      setAuthed(true)
    } catch (err) {
      if (err instanceof Error && err.message === 'Unauthorized') setAuthed(false)
    }
  }, [id])

  useEffect(() => {
    const kick = setTimeout(() => {
      setAuthed(hasViewerToken())
      void loadPublic()
      if (hasViewerToken()) void loadPrivate()
    }, 0)
    const refresh = setInterval(() => {
      void loadPublic()
      if (hasViewerToken()) void loadPrivate()
    }, REFRESH_MS)
    return () => {
      clearTimeout(kick)
      clearInterval(refresh)
    }
  }, [loadPublic, loadPrivate])

  // Watch-time heartbeat while the tab is open and the viewer is logged in.
  useEffect(() => {
    if (authed !== true) return
    const t = setInterval(() => {
      void viewerApi.engagementHeartbeat(id).catch(() => undefined)
    }, HEARTBEAT_MS)
    return () => clearInterval(t)
  }, [authed, id])

  const login = useCallback(
    async (platform: 'twitch' | 'youtube' | 'kick') => {
      try {
        const res = await fetch(
          `/api/v1/auth/viewer/${platform}/login?redirect_to=${encodeURIComponent(`/overlay/${id}/participate`)}`
        )
        const data = (await res.json()) as { auth_url?: string }
        if (data.auth_url) safeExternalRedirect(data.auth_url)
      } catch {
        setNotice('Could not start login. Please try again.')
      }
    },
    [id]
  )

  const vote = useCallback(
    async (optionIdx: number) => {
      // Native Twitch rounds are mirrored read-only — voting happens on Twitch.
      if (!poll || busy || poll.source === 'twitch_native') return
      setBusy(true)
      setNotice(null)
      try {
        await viewerApi.votePoll(id, poll.id, optionIdx)
        await Promise.all([loadPublic(), loadPrivate()])
      } catch (err) {
        setNotice(err instanceof Error ? err.message : 'Vote failed')
      } finally {
        setBusy(false)
      }
    },
    [poll, busy, id, loadPublic, loadPrivate]
  )

  const wager = useCallback(
    async (outcomeIdx: number) => {
      // Native Twitch rounds use Twitch channel points — no All-Chat wagers.
      if (!prediction || busy || prediction.source === 'twitch_native') return
      const amount = Number.parseInt(wagerAmount, 10)
      if (!Number.isFinite(amount) || amount <= 0) {
        setNotice('Enter a positive amount to wager.')
        return
      }
      setBusy(true)
      setNotice(null)
      try {
        await viewerApi.wagerPrediction(id, prediction.id, outcomeIdx, amount)
        setWagerAmount('')
        await Promise.all([loadPublic(), loadPrivate()])
      } catch (err) {
        setNotice(err instanceof Error ? err.message : 'Wager failed')
      } finally {
        setBusy(false)
      }
    },
    [prediction, busy, wagerAmount, id, loadPublic, loadPrivate]
  )

  if (authed === null) {
    return <main className="mx-auto max-w-md p-6 text-center text-slate-400">Loading…</main>
  }

  if (!authed) {
    return (
      <main className="mx-auto max-w-md space-y-4 p-6 text-center">
        <h1 className="text-xl font-bold">Join the fun</h1>
        <p className="text-slate-400">Log in with your platform account to vote and wager.</p>
        <div className="flex flex-col gap-2">
          {PLATFORMS.map((p) => (
            <button
              key={p.id}
              onClick={() => void login(p.id)}
              className="rounded-lg bg-purple-600 px-4 py-2 font-semibold text-white hover:bg-purple-700"
            >
              Continue with {p.label}
            </button>
          ))}
        </div>
      </main>
    )
  }

  const pollTotal = poll?.options.reduce((s, o) => s + o.votes, 0) ?? 0
  const predTotal = prediction?.outcomes.reduce((s, o) => s + o.total_points, 0) ?? 0
  const alreadyWagered = Boolean(engagement?.wager_outcome_id)
  const predOpen = prediction?.state === 'ACTIVE'
  // Mirrored native Twitch rounds show live tallies but are read-only here.
  const pollNative = poll?.source === 'twitch_native'
  const predNative = prediction?.source === 'twitch_native'

  return (
    <main className="mx-auto max-w-md space-y-6 p-4">
      <header className="flex items-center justify-between">
        <h1 className="text-lg font-bold">Participate</h1>
        <span className="rounded-full bg-black/10 px-3 py-1 text-sm font-semibold dark:bg-white/10">
          🔥 {(engagement?.balance ?? 0).toLocaleString()} {engagement?.points_name ?? 'Points'}
        </span>
      </header>

      {notice && <p className="rounded-md bg-red-500/15 px-3 py-2 text-sm text-red-400">{notice}</p>}

      {poll && poll.state === 'ACTIVE' && (
        <section className="space-y-2">
          <h2 className="flex items-center gap-2 font-semibold">📊 {poll.question}</h2>
          {pollNative && (
            <p className="text-sm text-slate-500">
              This poll runs on Twitch — vote in Twitch chat or the Twitch app.
            </p>
          )}
          {poll.options.map((o) => {
            const pct = pollTotal > 0 ? Math.round((o.votes / pollTotal) * 100) : 0
            const mine = engagement?.voted_option_id === o.id
            return (
              <button
                key={o.id}
                onClick={() => void vote(o.idx)}
                disabled={busy || pollNative}
                className={clsx(
                  'relative flex w-full items-center justify-between overflow-hidden rounded-lg border px-3 py-2 text-left',
                  mine ? 'border-purple-500' : 'border-slate-300 dark:border-slate-700',
                  (busy || pollNative) && 'opacity-60'
                )}
              >
                <span className="absolute inset-y-0 left-0 bg-purple-500/15" style={{ width: `${pct}%` }} />
                <span className="relative font-medium">
                  {o.label} {mine && '✓'}
                </span>
                <span className="relative text-sm tabular-nums text-slate-500">
                  {pct}% ({o.votes.toLocaleString()})
                </span>
              </button>
            )
          })}
        </section>
      )}

      {prediction && (prediction.state === 'ACTIVE' || prediction.state === 'LOCKED') && (
        <section className="space-y-2">
          <h2 className="flex items-center gap-2 font-semibold">
            🔮 {prediction.title}
            {prediction.state === 'LOCKED' && <span className="text-sm text-slate-500">🔒 locked</span>}
          </h2>
          {predNative && (
            <p className="text-sm text-slate-500">This prediction runs on Twitch channel points.</p>
          )}
          {!alreadyWagered && predOpen && (
            <input
              type="number"
              min={1}
              inputMode="numeric"
              value={wagerAmount}
              onChange={(e) => setWagerAmount(e.target.value)}
              placeholder={`Amount to wager (${engagement?.points_name ?? 'Points'})`}
              disabled={predNative}
              className={clsx(
                'w-full rounded-lg border border-slate-300 px-3 py-2 dark:border-slate-700 dark:bg-transparent',
                predNative && 'opacity-60'
              )}
            />
          )}
          {prediction.outcomes.map((o) => {
            const pct = predTotal > 0 ? Math.round((o.total_points / predTotal) * 100) : 0
            const mine = engagement?.wager_outcome_id === o.id
            return (
              <button
                key={o.id}
                onClick={() => void wager(o.idx)}
                disabled={busy || alreadyWagered || !predOpen || predNative}
                className={clsx(
                  'relative flex w-full items-center justify-between overflow-hidden rounded-lg border px-3 py-2 text-left',
                  mine ? 'border-sky-500' : 'border-slate-300 dark:border-slate-700',
                  (busy || alreadyWagered || !predOpen || predNative) && 'opacity-60'
                )}
              >
                <span className="absolute inset-y-0 left-0 bg-sky-500/15" style={{ width: `${pct}%` }} />
                <span className="relative font-medium">
                  {o.label} {mine && `· your wager: ${(engagement?.wager_amount ?? 0).toLocaleString()}`}
                </span>
                <span className="relative text-sm tabular-nums text-slate-500">
                  {o.total_points.toLocaleString()} · {pct}%
                </span>
              </button>
            )
          })}
          {alreadyWagered && <p className="text-sm text-slate-500">You&apos;ve locked in your wager for this round.</p>}
        </section>
      )}

      {!poll && !prediction && (
        <p className="text-center text-slate-400">No active poll or prediction right now. Hang tight!</p>
      )}
    </main>
  )
}
