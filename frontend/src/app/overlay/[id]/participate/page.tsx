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
import { useTranslations, type TFunction } from '@/lib/i18n'
import type { Poll, Prediction, ViewerEngagement } from '@/lib/types/engagement'

const REFRESH_MS = 3000
const HEARTBEAT_MS = 60000
// The three platforms with a viewer web login. Their display names live in
// common.platforms.*, which several other surfaces already read.
const PLATFORMS = ['twitch', 'youtube', 'kick'] as const
// Glyphs that sit beside copy stating the same thing in words, so they are
// decoration rather than text to translate.
const BALANCE_GLYPH = '🔥'
const POLL_GLYPH = '📊'
const PREDICTION_GLYPH = '🔮'
const LOCKED_GLYPH = '🔒'
const YOUR_VOTE_GLYPH = '✓'

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
function wagerRejectionCopy(
  t: TFunction,
  reason: string | undefined,
  pointsName: string,
  balance: number
): string | null {
  switch (reason) {
    case 'not_found':
      return t('viewerOverlay.participate.rejectNotFound')
    case 'not_active':
      return t('viewerOverlay.participate.rejectNotActive')
    case 'bad_outcome':
      return t('viewerOverlay.participate.rejectBadOutcome')
    case 'already_wagered':
      return t('viewerOverlay.participate.rejectAlreadyWagered')
    case 'insufficient':
      return t('viewerOverlay.participate.rejectInsufficient', {
        pointsName,
        balance: balance.toLocaleString(),
      })
    case 'native':
      return t('viewerOverlay.participate.rejectNative')
    default:
      return null
  }
}

export default function ParticipatePage({ params }: { params: Promise<{ id: string }> }) {
  const t = useTranslations()
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
  // Bumped around every viewer mutation (vote/wager) so an interval/WS refresh whose fetches
  // straddled the mutation can't overwrite the fresher post-mutation snapshot (mirrors
  // EngagementControls' mutationEpochRef).
  const mutationEpochRef = useRef(0)

  // fetchPublic / fetchPrivate return their data so refresh() can apply everything in
  // ONE state update — avoiding the multi-commit flicker of separate async chains (N1).
  const fetchPublic = useCallback(async (): Promise<{
    poll: Poll | null
    prediction: Prediction | null
  } | null> => {
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
    const epoch = mutationEpochRef.current
    const authedNow = hasViewerToken()
    const [pub, priv] = await Promise.all([
      fetchPublic(),
      authedNow ? fetchPrivate() : Promise.resolve<ViewerEngagement | 'unauth' | null>(null),
    ])
    // A vote/wager landed while these fetches were in flight → this snapshot is stale
    // and would clobber the fresher post-mutation state; drop it (its own refresh wins).
    if (mutationEpochRef.current !== epoch) return
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
          setSettled({
            outcomeLabel: wageredRoundRef.current.outcomeLabel,
            amount: wageredRoundRef.current.amount,
          })
          wageredRoundRef.current = null
        } else {
          setSettled(null)
        }
        if (eng?.wager_outcome_id) {
          const o = pub.prediction.outcomes.find((x) => x.id === eng.wager_outcome_id)
          if (o) {
            wageredRoundRef.current = {
              id: pub.prediction.id,
              outcomeLabel: o.label,
              amount: eng.wager_amount ?? 0,
            }
          }
        }
      } else if (wageredRoundRef.current) {
        setSettled({
          outcomeLabel: wageredRoundRef.current.outcomeLabel,
          amount: wageredRoundRef.current.amount,
        })
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
      const set = setTimeout(
        () => setAnnouncement(t('viewerOverlay.participate.lockedAnnouncement')),
        50
      )
      return () => {
        clearTimeout(clear)
        clearTimeout(set)
      }
    }
  }, [prediction?.state, t])

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
        setNotice(t('viewerOverlay.participate.loginFailed'))
      }
    },
    [id, t]
  )

  const vote = useCallback(
    async (optionIdx: number) => {
      // Native Twitch rounds are mirrored read-only — voting happens on Twitch.
      if (!poll || busy || poll.source === 'twitch_native') return
      setBusy(true)
      setNotice(null)
      mutationEpochRef.current += 1 // invalidate any in-flight refresh
      try {
        await viewerApi.votePoll(id, poll.id, optionIdx)
        mutationEpochRef.current += 1 // so this mutation's own refresh captures the newest epoch and wins
        await refresh()
      } catch (err) {
        if (isUnauthorized(err)) {
          setAuthed(false) // session expired mid-vote → bounce to login (L-U10)
          return
        }
        setNotice(err instanceof Error ? err.message : t('viewerOverlay.participate.voteFailed'))
      } finally {
        setBusy(false)
      }
    },
    [poll, busy, id, refresh, t]
  )

  const wager = useCallback(
    async (outcomeIdx: number) => {
      // Native Twitch rounds use Twitch channel points — no All-Chat wagers.
      if (!prediction || busy || prediction.source === 'twitch_native') return
      const amount = Number.parseInt(wagerAmount, 10)
      if (!Number.isFinite(amount) || amount <= 0) {
        setNotice(t('viewerOverlay.participate.wagerNeedsAmount'))
        return
      }
      const balance = engagement?.balance ?? 0
      if (amount > balance) {
        // Pre-empt the server 'insufficient' with a clearer, local message (L-U3).
        setNotice(
          t('viewerOverlay.participate.insufficientLocal', {
            pointsName: engagement?.points_name ?? t('viewerOverlay.participate.defaultPointsName'),
            balance: balance.toLocaleString(),
          })
        )
        return
      }
      setBusy(true)
      setNotice(null)
      mutationEpochRef.current += 1 // invalidate any in-flight refresh
      try {
        await viewerApi.wagerPrediction(id, prediction.id, outcomeIdx, amount)
        mutationEpochRef.current += 1 // so this mutation's own refresh captures the newest epoch and wins
        setWagerAmount('')
        await refresh()
      } catch (err) {
        if (isUnauthorized(err)) {
          setAuthed(false) // L-U10
          return
        }
        const copy = wagerRejectionCopy(
          t,
          apiErrorReason(err),
          engagement?.points_name ?? t('viewerOverlay.participate.defaultPointsName'),
          engagement?.balance ?? 0
        )
        setNotice(
          copy ?? (err instanceof Error ? err.message : t('viewerOverlay.participate.wagerFailed'))
        )
      } finally {
        setBusy(false)
      }
    },
    [prediction, busy, wagerAmount, id, engagement, refresh, t]
  )

  if (authed === null) {
    return (
      <main
        id="main-content"
        tabIndex={-1}
        className="mx-auto max-w-md p-6 text-center text-text-sub"
      >
        {t('viewerOverlay.participate.loading')}
      </main>
    )
  }

  if (!authed) {
    return (
      <main id="main-content" tabIndex={-1} className="mx-auto max-w-md space-y-4 p-6 text-center">
        <h1 className="text-xl font-bold">{t('viewerOverlay.participate.loginHeading')}</h1>
        <p className="text-text-sub">{t('viewerOverlay.participate.loginBlurb')}</p>
        <div className="flex flex-col gap-2">
          {PLATFORMS.map((platform) => (
            <button
              key={platform}
              onClick={() => void login(platform)}
              className="rounded-lg bg-purple-600 px-4 py-2 font-semibold text-white hover:bg-purple-700"
            >
              {t('viewerOverlay.participate.loginWith', {
                platform: t(`common.platforms.${platform}`),
              })}
            </button>
          ))}
        </div>
        {/* N2: TikTok/Discord have no web login — point those viewers at chat commands. */}
        <p className="text-sm text-text-sub">{t('viewerOverlay.participate.noWebLoginNote')}</p>
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
  const pointsName = engagement?.points_name ?? t('viewerOverlay.participate.defaultPointsName')

  return (
    <main id="main-content" tabIndex={-1} className="mx-auto max-w-md space-y-6 p-4">
      <header className="flex items-center justify-between">
        <h1 className="text-lg font-bold">{t('viewerOverlay.participate.heading')}</h1>
        <span
          className="rounded-full bg-surface-2 px-3 py-1 text-sm font-semibold text-text"
          aria-live="polite"
          aria-atomic="true"
          aria-label={t('viewerOverlay.participate.balanceLabel', {
            balance: balance.toLocaleString(),
            pointsName,
          })}
        >
          <span aria-hidden="true">{BALANCE_GLYPH}</span>{' '}
          {t('viewerOverlay.participate.balance', {
            balance: balance.toLocaleString(),
            pointsName,
          })}
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
          {t('viewerOverlay.participate.settledBanner', {
            outcome: settled.outcomeLabel,
            amount: settled.amount.toLocaleString(),
            pointsName,
          })}
        </p>
      )}

      {poll && poll.state === 'ACTIVE' && (
        <section className="space-y-2">
          <h2 className="flex items-center gap-2 font-semibold">
            <span aria-hidden="true">{POLL_GLYPH}</span> {poll.question}
          </h2>
          {pollNative && (
            <p className="text-sm text-text-sub">{t('viewerOverlay.participate.pollNativeNote')}</p>
          )}
          {poll.options.map((o) => {
            const pct = pollTotal > 0 ? Math.round((o.votes / pollTotal) * 100) : 0
            const mine = engagement?.voted_option_id === o.id
            return (
              <button
                key={o.id}
                onClick={() => void vote(o.idx)}
                disabled={busy || pollNative}
                title={
                  pollNative
                    ? t('viewerOverlay.participate.pollVoteNativeTitle')
                    : busy
                      ? t('viewerOverlay.participate.working')
                      : undefined
                }
                className={clsx(
                  'relative flex w-full items-center justify-between overflow-hidden rounded-lg border px-3 py-2 text-left',
                  mine ? 'border-purple-500' : 'border-border',
                  (busy || pollNative) && 'opacity-60'
                )}
              >
                <span
                  className="absolute inset-y-0 left-0 bg-purple-500/15"
                  style={{ width: `${pct}%` }}
                />
                <span className="relative font-medium">
                  {o.idx}. {o.label}
                  {mine && (
                    <>
                      {' '}
                      <span aria-hidden="true">{YOUR_VOTE_GLYPH}</span>
                      <span className="sr-only">{t('viewerOverlay.participate.yourVote')}</span>
                    </>
                  )}
                </span>
                <span className="relative text-sm text-text tabular-nums">
                  {t('viewerOverlay.participate.pollOptionTally', {
                    pct,
                    votes: o.votes.toLocaleString(),
                  })}
                </span>
              </button>
            )
          })}
        </section>
      )}

      {prediction && (prediction.state === 'ACTIVE' || prediction.state === 'LOCKED') && (
        <section className="space-y-2">
          <h2 className="flex items-center gap-2 font-semibold">
            <span aria-hidden="true">{PREDICTION_GLYPH}</span> {prediction.title}
            {prediction.state === 'LOCKED' && (
              <span className="text-sm text-text-sub">
                <span aria-hidden="true">{LOCKED_GLYPH}</span>{' '}
                {t('viewerOverlay.participate.predictionLocked')}
              </span>
            )}
          </h2>
          {predNative && (
            <p className="text-sm text-text-sub">
              {t('viewerOverlay.participate.predictionNativeNote')}
            </p>
          )}
          {!alreadyWagered && predOpen && !predNative && (
            <div className="space-y-1">
              <div className="flex items-center justify-between text-xs text-text-sub">
                <span>
                  {t('viewerOverlay.participate.youHave', {
                    balance: balance.toLocaleString(),
                    pointsName,
                  })}
                </span>
                <button
                  type="button"
                  onClick={() => setWagerAmount(String(balance))}
                  className="rounded px-1.5 py-0.5 font-medium text-text-sub underline hover:text-text focus-visible:ring-2 focus-visible:ring-purple-500 focus-visible:outline-none"
                >
                  {t('viewerOverlay.participate.maxWager')}
                </button>
              </div>
              {balance <= 0 && (
                <p className="text-xs text-text-sub">
                  {t('viewerOverlay.participate.noPointsYet', { pointsName })}
                </p>
              )}
              <label htmlFor="wager-amount" className="sr-only">
                {t('viewerOverlay.participate.wagerAmountLabel', { pointsName })}
              </label>
              <input
                id="wager-amount"
                type="number"
                min={1}
                max={balance}
                inputMode="numeric"
                value={wagerAmount}
                onChange={(e) => setWagerAmount(e.target.value)}
                placeholder={t('viewerOverlay.participate.wagerAmountPlaceholder', {
                  pointsName,
                })}
                className="w-full rounded-lg border border-border bg-transparent px-3 py-2 text-text placeholder:text-text-dim"
              />
            </div>
          )}
          {prediction.outcomes.map((o) => {
            const pct = predTotal > 0 ? Math.round((o.total_points / predTotal) * 100) : 0
            const mine = engagement?.wager_outcome_id === o.id
            const disabled = busy || alreadyWagered || !predOpen || predNative
            const title = predNative
              ? t('viewerOverlay.participate.wagerNativeTitle')
              : alreadyWagered
                ? t('viewerOverlay.participate.wagerAlreadyTitle')
                : !predOpen
                  ? t('viewerOverlay.participate.wagerClosedTitle')
                  : busy
                    ? t('viewerOverlay.participate.working')
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
                <span
                  className="absolute inset-y-0 left-0 bg-sky-500/15"
                  style={{ width: `${pct}%` }}
                />
                <span className="relative font-medium">
                  {o.idx}. {o.label}
                  {mine &&
                    t('viewerOverlay.participate.yourWager', {
                      amount: (engagement?.wager_amount ?? 0).toLocaleString(),
                    })}
                </span>
                <span className="relative text-sm text-text tabular-nums">
                  {t('viewerOverlay.participate.outcomeTally', {
                    points: o.total_points.toLocaleString(),
                    pct,
                  })}
                </span>
              </button>
            )
          })}
          {alreadyWagered && (
            <p className="text-sm text-text-sub">{t('viewerOverlay.participate.alreadyWagered')}</p>
          )}
        </section>
      )}

      {!poll && !prediction && (
        <p className="text-center text-text-sub">{t('viewerOverlay.participate.nothingActive')}</p>
      )}
    </main>
  )
}
