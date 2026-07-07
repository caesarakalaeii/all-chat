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
 *
 * Styling note: the app is dark-only (no `dark:` variant is configured — see
 * globals.css), so this page uses the semantic design tokens (bg-surface / text-text
 * / border-border …) directly rather than `dark:`-gated slate colours, which never
 * activated on a fixed near-black body (M-A1).
 */

'use client'

import { use, useCallback, useEffect, useRef, useState } from 'react'
import clsx from 'clsx'
import { apiErrorReason, viewerApi } from '@/lib/api/viewer'
import { inMemoryTokens } from '@/lib/auth/in-memory-store'
import { safeExternalRedirect } from '@/lib/auth/redirect-allowlist'
import { useEngagementLive } from '@/lib/hooks/useEngagementLive'
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

function isUnauthorized(err: unknown): boolean {
  return err instanceof Error && err.message === 'Unauthorized'
}

// wagerRejectionCopy maps the server's machine reason for a rejected wager to human
// copy (L-U2), so a failure surfaces something actionable instead of the opaque
// "wager not accepted". Reasons: repository/predictions.go WagerResult.Reason.
function wagerRejectionCopy(reason: string | undefined, pointsName: string, balance: number): string | null {
  switch (reason) {
    case 'not_found':
      return 'This prediction is no longer available.'
    case 'not_active':
      return 'Betting is closed for this round.'
    case 'bad_outcome':
      return 'That outcome is not valid.'
    case 'already_wagered':
      return 'You already placed a wager this round.'
    case 'insufficient':
      return `Not enough ${pointsName}. You have ${balance.toLocaleString()}.`
    case 'native':
      return 'This prediction runs on Twitch channel points.'
    default:
      return null
  }
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
  // M2: a transient banner shown when a prediction the viewer wagered on ends, so the
  // section doesn't just silently unmount (the active endpoint stops returning the
  // round on resolve). Retained until a new round begins.
  const [settled, setSettled] = useState<{ outcomeLabel: string; amount: number } | null>(null)
  const wageredRoundRef = useRef<{ id: string; outcomeLabel: string; amount: number } | null>(null)
  // M-A2: a single dedicated polite live region announces the prediction locking, rather
  // than voicing the whole tally section on every 3s refresh (which would over-announce).
  const [announcement, setAnnouncement] = useState('')
  const prevPredStateRef = useRef<Prediction['state'] | null>(null)

  // fetchPublic / fetchPrivate return their data so refresh() can apply everything in
  // ONE state update — avoiding the multi-commit flicker of separate async chains (N1).
  const fetchPublic = useCallback(async (): Promise<{ poll: Poll | null; prediction: Prediction | null } | null> => {
    try {
      const [pRes, prRes] = await Promise.all([
        fetch(`/api/v1/engagement/overlays/${id}/active-poll`, { cache: 'no-store' }),
        fetch(`/api/v1/engagement/overlays/${id}/active-prediction`, { cache: 'no-store' }),
      ])
      return {
        poll: pRes.ok ? ((await pRes.json()) as Poll) : null,
        prediction: prRes.ok ? ((await prRes.json()) as Prediction) : null,
      }
    } catch {
      return null // keep last render on transient errors
    }
  }, [id])

  const fetchPrivate = useCallback(async (): Promise<ViewerEngagement | 'unauth' | null> => {
    try {
      return await viewerApi.getEngagement(id)
    } catch (err) {
      return isUnauthorized(err) ? 'unauth' : null
    }
  }, [id])

  const refresh = useCallback(async () => {
    const authedNow = hasViewerToken()
    const [pub, priv] = await Promise.all([
      fetchPublic(),
      authedNow ? fetchPrivate() : Promise.resolve<ViewerEngagement | 'unauth' | null>(null),
    ])
    const eng = priv && priv !== 'unauth' ? priv : null
    if (pub) {
      setPoll(pub.poll)
      setPrediction(pub.prediction)
      // M2: track the viewer's wagered round while it's live, and raise the settled
      // banner when it ends (the active endpoint stops returning it on resolve, so the
      // section would otherwise just silently unmount). Done here in the async refresh
      // rather than a render-triggered effect to avoid a cascading re-render.
      if (pub.prediction) {
        // A DIFFERENT round is live than the one the viewer wagered on → the wagered round
        // ended (fast round-to-round handoff). Raise its settled banner BEFORE adopting the
        // new round, so the "your prediction settled" reveal isn't skipped (P3-11). Same
        // round still live → no banner. Cleared unconditionally otherwise.
        if (wageredRoundRef.current && wageredRoundRef.current.id !== pub.prediction.id) {
          setSettled({ outcomeLabel: wageredRoundRef.current.outcomeLabel, amount: wageredRoundRef.current.amount })
          wageredRoundRef.current = null
        } else {
          setSettled(null)
        }
        if (eng?.wager_outcome_id) {
          const o = pub.prediction.outcomes.find((x) => x.id === eng.wager_outcome_id)
          if (o) {
            wageredRoundRef.current = { id: pub.prediction.id, outcomeLabel: o.label, amount: eng.wager_amount ?? 0 }
          }
        }
      } else if (wageredRoundRef.current) {
        setSettled({ outcomeLabel: wageredRoundRef.current.outcomeLabel, amount: wageredRoundRef.current.amount })
        wageredRoundRef.current = null
      }
    }
    if (priv === 'unauth') {
      setAuthed(false)
      setEngagement(null)
    } else if (priv) {
      setEngagement(priv)
      setAuthed(true)
    } else {
      setAuthed(authedNow)
    }
  }, [fetchPublic, fetchPrivate])

  useEffect(() => {
    const kick = setTimeout(() => void refresh(), 0)
    const t = setInterval(() => void refresh(), REFRESH_MS)
    return () => {
      clearTimeout(kick)
      clearInterval(t)
    }
  }, [refresh])

  // Near-real-time refresh on a poll/prediction WS frame (L-D1); the interval remains
  // the fallback / source of truth. viewerParticipant: this is an anonymous viewer tab, so
  // the gateway must not auto-activate sources (no viewer-driven YouTube quota) and
  // reconnects are bounded (no storm from every tab) — P2-3.
  useEngagementLive(id, () => void refresh(), { viewerParticipant: true })

  // M-A2: announce the prediction locking to screen readers when the state transitions
  // ACTIVE → LOCKED. setState is deferred out of the effect body (never synchronous).
  useEffect(() => {
    const state = prediction?.state ?? null
    const prev = prevPredStateRef.current
    prevPredStateRef.current = state
    if (prev !== 'LOCKED' && state === 'LOCKED') {
      // Clear then set (both deferred, never synchronous) so a repeat lock on a later
      // round re-announces even though the message text is identical — an unchanged
      // aria-live node is not re-read by screen readers.
      const clear = setTimeout(() => setAnnouncement(''), 0)
      const set = setTimeout(() => setAnnouncement('Prediction locked — betting is closed.'), 50)
      return () => {
        clearTimeout(clear)
        clearTimeout(set)
      }
    }
  }, [prediction?.state])

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
        await refresh()
      } catch (err) {
        if (isUnauthorized(err)) {
          setAuthed(false) // session expired mid-vote → bounce to login (L-U10)
          return
        }
        setNotice(err instanceof Error ? err.message : 'Vote failed')
      } finally {
        setBusy(false)
      }
    },
    [poll, busy, id, refresh]
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
      const balance = engagement?.balance ?? 0
      if (amount > balance) {
        // Pre-empt the server 'insufficient' with a clearer, local message (L-U3).
        setNotice(`Not enough ${engagement?.points_name ?? 'Points'} — you have ${balance.toLocaleString()}.`)
        return
      }
      setBusy(true)
      setNotice(null)
      try {
        await viewerApi.wagerPrediction(id, prediction.id, outcomeIdx, amount)
        setWagerAmount('')
        await refresh()
      } catch (err) {
        if (isUnauthorized(err)) {
          setAuthed(false) // L-U10
          return
        }
        const copy = wagerRejectionCopy(apiErrorReason(err), engagement?.points_name ?? 'Points', engagement?.balance ?? 0)
        setNotice(copy ?? (err instanceof Error ? err.message : 'Wager failed'))
      } finally {
        setBusy(false)
      }
    },
    [prediction, busy, wagerAmount, id, engagement, refresh]
  )

  if (authed === null) {
    return <main className="mx-auto max-w-md p-6 text-center text-text-sub">Loading…</main>
  }

  if (!authed) {
    return (
      <main className="mx-auto max-w-md space-y-4 p-6 text-center">
        <h1 className="text-xl font-bold">Join the fun</h1>
        <p className="text-text-sub">Log in with your platform account to vote and wager.</p>
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
        {/* N2: TikTok/Discord have no web login — point those viewers at chat commands. */}
        <p className="text-sm text-text-sub">
          Watching on TikTok or Discord? Take part with the on-screen chat commands — web login isn&apos;t
          available for those platforms yet.
        </p>
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
  const balance = engagement?.balance ?? 0
  const pointsName = engagement?.points_name ?? 'Points'

  return (
    <main className="mx-auto max-w-md space-y-6 p-4">
      <header className="flex items-center justify-between">
        <h1 className="text-lg font-bold">Participate</h1>
        <span
          className="rounded-full bg-surface-2 px-3 py-1 text-sm font-semibold text-text"
          aria-live="polite"
          aria-atomic="true"
          aria-label={`Balance: ${balance.toLocaleString()} ${pointsName}`}
        >
          <span aria-hidden="true">🔥</span> {balance.toLocaleString()} {pointsName}
        </span>
      </header>

      {/* M-A2: dedicated polite status region — announces the prediction locking without
          re-voicing the tally on every refresh. */}
      <p role="status" aria-live="polite" className="sr-only">
        {announcement}
      </p>

      {notice && (
        <p role="alert" className="rounded-md bg-red-500/15 px-3 py-2 text-sm text-red-400">
          {notice}
        </p>
      )}

      {settled && (
        <p role="status" className="rounded-md bg-surface-2 px-3 py-2 text-sm text-text">
          Your prediction on “{settled.outcomeLabel}” settled — you wagered {settled.amount.toLocaleString()}{' '}
          {pointsName}. Check your balance above.
        </p>
      )}

      {poll && poll.state === 'ACTIVE' && (
        <section className="space-y-2">
          <h2 className="flex items-center gap-2 font-semibold">
            <span aria-hidden="true">📊</span> {poll.question}
          </h2>
          {pollNative && (
            <p className="text-sm text-text-sub">This poll runs on Twitch — vote in Twitch chat or the Twitch app.</p>
          )}
          {poll.options.map((o) => {
            const pct = pollTotal > 0 ? Math.round((o.votes / pollTotal) * 100) : 0
            const mine = engagement?.voted_option_id === o.id
            return (
              <button
                key={o.id}
                onClick={() => void vote(o.idx)}
                disabled={busy || pollNative}
                title={pollNative ? 'Vote in Twitch chat' : busy ? 'Working…' : undefined}
                className={clsx(
                  'relative flex w-full items-center justify-between overflow-hidden rounded-lg border px-3 py-2 text-left',
                  mine ? 'border-purple-500' : 'border-border',
                  (busy || pollNative) && 'opacity-60'
                )}
              >
                <span className="absolute inset-y-0 left-0 bg-purple-500/15" style={{ width: `${pct}%` }} />
                <span className="relative font-medium">
                  {o.idx}. {o.label}
                  {mine && (
                    <>
                      {' '}
                      <span aria-hidden="true">✓</span>
                      <span className="sr-only">(your vote)</span>
                    </>
                  )}
                </span>
                <span className="relative text-sm tabular-nums text-text">
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
            <span aria-hidden="true">🔮</span> {prediction.title}
            {prediction.state === 'LOCKED' && (
              <span className="text-sm text-text-sub">
                <span aria-hidden="true">🔒</span> locked
              </span>
            )}
          </h2>
          {predNative && <p className="text-sm text-text-sub">This prediction runs on Twitch channel points.</p>}
          {!alreadyWagered && predOpen && !predNative && (
            <div className="space-y-1">
              <div className="flex items-center justify-between text-xs text-text-sub">
                <span>
                  You have {balance.toLocaleString()} {pointsName}
                </span>
                <button
                  type="button"
                  onClick={() => setWagerAmount(String(balance))}
                  className="rounded px-1.5 py-0.5 font-medium text-text-sub underline hover:text-text focus-visible:ring-2 focus-visible:ring-purple-500 focus-visible:outline-none"
                >
                  Max
                </button>
              </div>
              {balance <= 0 && (
                <p className="text-xs text-text-sub">
                  You have no {pointsName} yet. Earn them by keeping this page open and by supporting the
                  stream (subs, bits, donations, gifts), then come back to wager.
                </p>
              )}
              <label htmlFor="wager-amount" className="sr-only">
                Amount to wager in {pointsName}
              </label>
              <input
                id="wager-amount"
                type="number"
                min={1}
                max={balance}
                inputMode="numeric"
                value={wagerAmount}
                onChange={(e) => setWagerAmount(e.target.value)}
                placeholder={`Amount to wager (${pointsName})`}
                className="w-full rounded-lg border border-border bg-transparent px-3 py-2 text-text placeholder:text-text-dim"
              />
            </div>
          )}
          {prediction.outcomes.map((o) => {
            const pct = predTotal > 0 ? Math.round((o.total_points / predTotal) * 100) : 0
            const mine = engagement?.wager_outcome_id === o.id
            const disabled = busy || alreadyWagered || !predOpen || predNative
            const title = predNative
              ? 'Runs on Twitch channel points'
              : alreadyWagered
                ? 'You already wagered this round'
                : !predOpen
                  ? 'Betting is closed'
                  : busy
                    ? 'Working…'
                    : undefined
            return (
              <button
                key={o.id}
                onClick={() => void wager(o.idx)}
                disabled={disabled}
                title={title}
                className={clsx(
                  'relative flex w-full items-center justify-between overflow-hidden rounded-lg border px-3 py-2 text-left',
                  mine ? 'border-sky-500' : 'border-border',
                  disabled && 'opacity-60'
                )}
              >
                <span className="absolute inset-y-0 left-0 bg-sky-500/15" style={{ width: `${pct}%` }} />
                <span className="relative font-medium">
                  {o.idx}. {o.label}
                  {mine && ` · your wager: ${(engagement?.wager_amount ?? 0).toLocaleString()}`}
                </span>
                <span className="relative text-sm tabular-nums text-text">
                  {o.total_points.toLocaleString()} · {pct}%
                </span>
              </button>
            )
          })}
          {alreadyWagered && <p className="text-sm text-text-sub">You&apos;ve locked in your wager for this round.</p>}
        </section>
      )}

      {!poll && !prediction && (
        <p className="text-center text-text-sub">No active poll or prediction right now. Hang tight!</p>
      )}
    </main>
  )
}
