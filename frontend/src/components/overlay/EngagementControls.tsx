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
 * Poll & prediction control panel for the overlay monitor view (issue #523).
 * Owner-only (the caller gates on ownership): start/close a poll and start/
 * lock/resolve/cancel a prediction, with live tallies. State comes from the
 * same public active-poll/active-prediction endpoints the OBS widgets poll,
 * plus the immediate response of each owner mutation. A finished round stays
 * visible (the public endpoint 404s once it's inactive) until dismissed, so
 * the streamer can read the final numbers.
 *
 * Rounds mirrored from Twitch (source === 'twitch_native') are read-only:
 * Twitch owns their lifecycle, so no mutating controls are rendered — only the
 * live tallies, a "Twitch" origin badge, and a hint.
 */

'use client'

import clsx from 'clsx'
import { Plus, X } from 'lucide-react'
import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react'
import toast from 'react-hot-toast'

import { ApiError } from '@/lib/api/client'
import { engagementApi } from '@/lib/api/engagement'
import { useEngagementLive } from '@/lib/hooks/useEngagementLive'
import type { Poll, Prediction } from '@/lib/types/engagement'

const REFRESH_MS = 3000
const MAX_POLL_OPTIONS = 5
const MAX_PREDICTION_OUTCOMES = 10

const inputClass =
  'w-full rounded-lg border border-border bg-surface px-2 py-1.5 text-xs text-text focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none'
const primaryButtonClass =
  'rounded-lg bg-twitch px-3 py-1.5 text-xs font-semibold text-white transition-colors hover:bg-twitch/90 focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50'
const secondaryButtonClass =
  'rounded-lg border border-border px-3 py-1.5 text-xs font-medium text-text-sub transition-colors hover:border-border-md hover:text-text focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50'

function errorMessage(err: unknown, fallback: string): string {
  return err instanceof ApiError ? err.message : fallback
}

/** Editable list of option/outcome labels for the create forms. */
function LabelListEditor({
  labels,
  onChange,
  max,
  placeholder,
  disabled,
}: {
  labels: string[]
  onChange: (next: string[]) => void
  max: number
  placeholder: string
  disabled: boolean
}) {
  return (
    <div className="space-y-1.5">
      {labels.map((label, i) => (
        <div key={i} className="flex items-center gap-1.5">
          <input
            type="text"
            value={label}
            onChange={(e) => onChange(labels.map((l, j) => (j === i ? e.target.value : l)))}
            placeholder={`${placeholder} ${i + 1}`}
            aria-label={`${placeholder} ${i + 1}`}
            disabled={disabled}
            className={inputClass}
          />
          {labels.length > 2 && (
            <button
              type="button"
              onClick={() => onChange(labels.filter((_, j) => j !== i))}
              disabled={disabled}
              title="Remove"
              className="rounded p-1 text-text-dim transition-colors hover:text-text focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
            >
              <X className="h-3.5 w-3.5" />
            </button>
          )}
        </div>
      ))}
      {labels.length < max && (
        <button
          type="button"
          onClick={() => onChange([...labels, ''])}
          disabled={disabled}
          className="flex items-center gap-1 text-xs text-text-sub transition-colors hover:text-text focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
        >
          <Plus className="h-3.5 w-3.5" />
          Add {placeholder.toLowerCase()}
        </button>
      )}
    </div>
  )
}

/** Horizontal tally bar shared by the live poll and prediction views. */
function TallyBar({
  label,
  detail,
  pct,
  accent,
  leading,
}: {
  label: string
  detail: string
  pct: number
  accent: 'twitch' | 'sky'
  leading?: ReactNode
}) {
  return (
    <div className="relative overflow-hidden rounded-md border border-border px-2 py-1.5">
      <span
        className={clsx(
          'absolute inset-y-0 left-0',
          accent === 'twitch' ? 'bg-twitch/15' : 'bg-sky-500/15'
        )}
        style={{ width: `${pct}%` }}
      />
      <div className="relative flex items-center justify-between gap-2 text-xs">
        <span className="flex min-w-0 items-center gap-1.5 font-medium text-text">
          {leading}
          <span className="truncate">{label}</span>
        </span>
        <span className="shrink-0 text-text tabular-nums">{detail}</span>
      </div>
    </div>
  )
}

function StateBadge({ state }: { state: string }) {
  return (
    <span className="rounded border border-border bg-surface px-1.5 py-0.5 text-[10px] font-semibold tracking-wide text-text-sub uppercase">
      {state}
    </span>
  )
}

/** Origin marker for rounds mirrored from Twitch; sits next to the StateBadge. */
function TwitchSourceBadge() {
  return (
    <span className="rounded border border-border bg-surface px-1.5 py-0.5 text-[10px] font-semibold tracking-wide text-text-sub uppercase">
      Twitch
    </span>
  )
}

export function EngagementControls({ overlayId }: { overlayId: string }) {
  const [poll, setPoll] = useState<Poll | null>(null)
  const [prediction, setPrediction] = useState<Prediction | null>(null)
  const [busy, setBusy] = useState(false)
  const [confirmCancel, setConfirmCancel] = useState(false)
  // Payout is irreversible, so it gets the same two-step echoing confirm the
  // (reversible) cancel already has — the stronger guardrail on the stronger action (M3).
  const [confirmResolve, setConfirmResolve] = useState(false)
  // Bumped around every mutation so a refresh whose fetches straddled the
  // mutation can't overwrite the fresher post-mutation state (or wipe the
  // final tallies at the next 404).
  const mutationEpochRef = useRef(0)

  // Create-poll form
  const [question, setQuestion] = useState('')
  const [options, setOptions] = useState<string[]>(['', ''])
  const [allowChange, setAllowChange] = useState(true)
  const [pollDuration, setPollDuration] = useState('')

  // Create-prediction form
  const [title, setTitle] = useState('')
  const [outcomes, setOutcomes] = useState<string[]>(['', ''])
  const [autoLock, setAutoLock] = useState('')
  const [winnerId, setWinnerId] = useState('')

  const refresh = useCallback(async () => {
    const epoch = mutationEpochRef.current
    try {
      const [pRes, prRes] = await Promise.all([
        fetch(`/api/v1/engagement/overlays/${overlayId}/active-poll`, { cache: 'no-store' }),
        fetch(`/api/v1/engagement/overlays/${overlayId}/active-prediction`, { cache: 'no-store' }),
      ])
      const nextPoll = pRes.ok ? ((await pRes.json()) as Poll) : null
      const nextPrediction = prRes.ok ? ((await prRes.json()) as Prediction) : null
      if (mutationEpochRef.current !== epoch) return // stale — a mutation landed meanwhile
      // On 404 only clear a round that was still running (it ended elsewhere,
      // e.g. auto-close); a finished one we're displaying stays until dismissed.
      setPoll((prev) => nextPoll ?? (prev && prev.state !== 'ACTIVE' ? prev : null))
      setPrediction(
        (prev) =>
          nextPrediction ??
          (prev && prev.state !== 'ACTIVE' && prev.state !== 'LOCKED' ? prev : null)
      )
    } catch {
      /* keep last render on transient errors */
    }
  }, [overlayId])

  useEffect(() => {
    const kick = setTimeout(() => void refresh(), 0)
    const t = setInterval(() => void refresh(), REFRESH_MS)
    return () => {
      clearTimeout(kick)
      clearInterval(t)
    }
  }, [refresh])

  // Near-real-time refresh on a poll/prediction WS frame (L-D1); the interval remains
  // the fallback / source of truth.
  useEngagementLive(overlayId, () => void refresh())

  const run = useCallback(
    async (call: () => Promise<void>, failMsg: string) => {
      if (busy) return
      setBusy(true)
      mutationEpochRef.current += 1
      try {
        await call()
      } catch (err) {
        toast.error(errorMessage(err, failMsg))
      } finally {
        mutationEpochRef.current += 1
        setBusy(false)
      }
    },
    [busy]
  )

  // --- Poll actions ----------------------------------------------------------

  // NOTE: a round's question/title and options are immutable once started — there is
  // no edit endpoint — so proofread before Start; fixing a typo means close/cancel +
  // recreate (costly for a prediction that already has wagers) (L-U8).
  const startPoll = () => {
    const labels = options.map((o) => o.trim()).filter(Boolean)
    if (!question.trim() || labels.length < 2) {
      toast.error('A poll needs a question and at least 2 options')
      return
    }
    const duration = Number.parseInt(pollDuration, 10)
    void run(async () => {
      const created = await engagementApi.createPoll(overlayId, {
        question: question.trim(),
        options: labels,
        allow_change: allowChange,
        ...(Number.isFinite(duration) && duration > 0 ? { duration_seconds: duration } : {}),
      })
      setPoll(created)
      setQuestion('')
      setOptions(['', ''])
      setPollDuration('')
      toast.success('Poll started')
    }, 'Could not start the poll')
  }

  const closePoll = (pollId: string) =>
    void run(async () => {
      setPoll(await engagementApi.closePoll(overlayId, pollId))
      toast.success('Poll closed')
    }, 'Could not close the poll')

  // --- Prediction actions ----------------------------------------------------

  const startPrediction = () => {
    const labels = outcomes.map((o) => o.trim()).filter(Boolean)
    if (!title.trim() || labels.length < 2) {
      toast.error('A prediction needs a title and at least 2 outcomes')
      return
    }
    const lockSecs = Number.parseInt(autoLock, 10)
    void run(async () => {
      const created = await engagementApi.createPrediction(overlayId, {
        title: title.trim(),
        outcomes: labels,
        ...(Number.isFinite(lockSecs) && lockSecs > 0 ? { auto_lock_seconds: lockSecs } : {}),
      })
      setPrediction(created)
      setWinnerId('')
      setTitle('')
      setOutcomes(['', ''])
      setAutoLock('')
      toast.success('Prediction started')
    }, 'Could not start the prediction')
  }

  const lockPrediction = (pid: string) =>
    void run(async () => {
      setPrediction(await engagementApi.lockPrediction(overlayId, pid))
      toast.success('Prediction locked — wagers are frozen')
    }, 'Could not lock the prediction')

  const resolvePrediction = (pid: string) => {
    if (!winnerId) {
      toast.error('Pick the winning outcome first')
      return
    }
    void run(async () => {
      const resolved = await engagementApi.resolvePrediction(overlayId, pid, winnerId)
      setConfirmResolve(false)
      if (resolved.state !== 'RESOLVED') {
        // The service only resolves LOCKED predictions; the Pay-out button only renders
        // while LOCKED, so this is a lost race (auto-lock/refresh), not user error —
        // acknowledge it neutrally rather than scolding (L-U7).
        setPrediction(resolved)
        toast('The prediction is no longer locked — refresh and try again')
        return
      }
      setPrediction(resolved)
      setWinnerId('')
      toast.success('Prediction resolved — winners paid out')
    }, 'Could not resolve the prediction')
  }

  const cancelPrediction = (pid: string) =>
    void run(async () => {
      const result = await engagementApi.cancelPrediction(overlayId, pid)
      setPrediction(result)
      setWinnerId('')
      setConfirmCancel(false)
      // Cancel is an idempotent guarded update: a 200 on an already-finished
      // prediction returns it unchanged — don't claim a refund that didn't happen.
      if (result.state === 'CANCELED') {
        toast.success('Prediction canceled — all wagers refunded')
      } else {
        toast.error(`Nothing to cancel — the prediction is already ${result.state.toLowerCase()}`)
      }
    }, 'Could not cancel the prediction')

  // --- Twitch mirroring opt-in ------------------------------------------------

  const startMirrorConsent = useCallback(async () => {
    try {
      window.location.href = await engagementApi.getTwitchMirrorConsentUrl(overlayId)
    } catch {
      toast.error('Could not start Twitch consent. Please try again.')
    }
  }, [overlayId])

  // Disarm the cancel/payout confirmations if they aren't acted on quickly.
  useEffect(() => {
    if (!confirmCancel) return
    const t = setTimeout(() => setConfirmCancel(false), 5000)
    return () => clearTimeout(t)
  }, [confirmCancel])
  useEffect(() => {
    if (!confirmResolve) return
    const t = setTimeout(() => setConfirmResolve(false), 5000)
    return () => clearTimeout(t)
  }, [confirmResolve])

  // --- Render ----------------------------------------------------------------

  const pollTotal = poll?.options.reduce((s, o) => s + o.votes, 0) ?? 0
  const predTotal = prediction?.outcomes.reduce((s, o) => s + o.total_points, 0) ?? 0
  const pollFinished = poll != null && poll.state !== 'ACTIVE'
  const predFinished =
    prediction != null && prediction.state !== 'ACTIVE' && prediction.state !== 'LOCKED'
  // Mirrored native Twitch rounds are read-only here — Twitch owns their lifecycle.
  const pollNative = poll?.source === 'twitch_native'
  const predNative = prediction?.source === 'twitch_native'
  const winnerLabel = prediction?.outcomes.find((o) => o.id === winnerId)?.label

  return (
    <section className="border-b border-border bg-surface px-4 py-3">
      <div className="grid gap-3 md:grid-cols-2">
        {/* Poll column */}
        <div className="rounded-lg border border-border bg-surface-2 p-3">
          <div className="mb-2 flex items-center justify-between">
            <h3 className="text-xs font-semibold tracking-wide text-text-sub uppercase">Poll</h3>
            {poll && (
              <span className="flex items-center gap-1.5">
                {pollNative && <TwitchSourceBadge />}
                <StateBadge state={poll.state} />
              </span>
            )}
          </div>

          {poll ? (
            <div className="space-y-2">
              <p className="text-sm font-medium text-text">{poll.question}</p>
              {poll.options.map((o) => (
                <TallyBar
                  key={o.id}
                  label={`${o.idx}. ${o.label}`}
                  detail={`${pollTotal > 0 ? Math.round((o.votes / pollTotal) * 100) : 0}% (${o.votes.toLocaleString()})`}
                  pct={pollTotal > 0 ? Math.round((o.votes / pollTotal) * 100) : 0}
                  accent="twitch"
                />
              ))}
              <div className="flex flex-wrap items-center justify-between gap-2 pt-1">
                <span className="text-[11px] text-text-sub">
                  {pollTotal.toLocaleString()} votes
                  {poll.ends_at &&
                    poll.state === 'ACTIVE' &&
                    ` · auto-closes ${new Date(poll.ends_at).toLocaleTimeString()}`}
                </span>
                {pollFinished ? (
                  <button
                    type="button"
                    onClick={() => setPoll(null)}
                    className={secondaryButtonClass}
                  >
                    New poll
                  </button>
                ) : (
                  !pollNative && (
                    <button
                      type="button"
                      onClick={() => closePoll(poll.id)}
                      disabled={busy}
                      className={primaryButtonClass}
                    >
                      Close poll
                    </button>
                  )
                )}
              </div>
              {pollNative && (
                <p className="text-[11px] text-text-sub">
                  Mirrored from Twitch — viewers vote in the Twitch UI/chat
                </p>
              )}
            </div>
          ) : (
            <div className="space-y-2">
              <input
                type="text"
                value={question}
                onChange={(e) => setQuestion(e.target.value)}
                placeholder="Question"
                aria-label="Poll question"
                disabled={busy}
                className={inputClass}
              />
              <LabelListEditor
                labels={options}
                onChange={setOptions}
                max={MAX_POLL_OPTIONS}
                placeholder="Option"
                disabled={busy}
              />
              <div className="flex flex-wrap items-center gap-3">
                <label className="flex cursor-pointer items-center gap-1.5 text-xs text-text-sub">
                  <input
                    type="checkbox"
                    checked={allowChange}
                    onChange={(e) => setAllowChange(e.target.checked)}
                    className="accent-twitch"
                  />
                  Allow vote changes
                </label>
                <label className="flex items-center gap-1.5 text-xs text-text-sub">
                  Auto-close after
                  <input
                    type="number"
                    min={0}
                    value={pollDuration}
                    onChange={(e) => setPollDuration(e.target.value)}
                    placeholder="∞"
                    disabled={busy}
                    className={clsx(inputClass, 'w-16')}
                  />
                  s
                </label>
              </div>
              <div className="flex items-center justify-between gap-2">
                <span className="text-[11px] text-text-sub">
                  Viewers vote on the{' '}
                  <a
                    href={`/overlay/${overlayId}/participate`}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="underline hover:text-text focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
                  >
                    participate page
                  </a>{' '}
                  or from chat (<code>!vote 2</code> or just <code>2</code>)
                </span>
                <button type="button" onClick={startPoll} disabled={busy} className={primaryButtonClass}>
                  Start poll
                </button>
              </div>
            </div>
          )}
        </div>

        {/* Prediction column */}
        <div className="rounded-lg border border-border bg-surface-2 p-3">
          <div className="mb-2 flex items-center justify-between">
            <h3 className="text-xs font-semibold tracking-wide text-text-sub uppercase">
              Prediction
            </h3>
            {prediction && (
              <span className="flex items-center gap-1.5">
                {predNative && <TwitchSourceBadge />}
                <StateBadge state={prediction.state} />
              </span>
            )}
          </div>

          {prediction ? (
            <div className="space-y-2">
              <p className="text-sm font-medium text-text">{prediction.title}</p>
              <div
                className="space-y-2"
                role={prediction.state === 'LOCKED' && !predNative ? 'radiogroup' : undefined}
                aria-label={
                  prediction.state === 'LOCKED' && !predNative ? 'Winning outcome' : undefined
                }
              >
                {prediction.outcomes.map((o) => (
                  <TallyBar
                    key={o.id}
                    label={`${o.idx}. ${o.label}`}
                    detail={`${o.total_points.toLocaleString()} pts · ${o.entrants.toLocaleString()} entrants`}
                    pct={predTotal > 0 ? Math.round((o.total_points / predTotal) * 100) : 0}
                    accent="sky"
                    leading={
                      prediction.state === 'LOCKED' && !predNative ? (
                        <input
                          type="radio"
                          name="engagement-winner"
                          checked={winnerId === o.id}
                          onChange={() => {
                            setWinnerId(o.id)
                            setConfirmResolve(false) // re-arm the payout confirm when the winner changes
                          }}
                          aria-label={`Winning outcome: ${o.label}`}
                          className="accent-twitch focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
                        />
                      ) : prediction.winning_outcome_id === o.id ? (
                        <span title="Winning outcome" aria-label="Winning outcome">🏆</span>
                      ) : undefined
                    }
                  />
                ))}
              </div>
              <div className="flex flex-wrap items-center justify-between gap-2 pt-1">
                <span className="text-[11px] text-text-sub">
                  {predTotal.toLocaleString()} points wagered
                  {prediction.auto_lock_at &&
                    prediction.state === 'ACTIVE' &&
                    ` · auto-locks ${new Date(prediction.auto_lock_at).toLocaleTimeString()}`}
                </span>
                <div className="flex flex-wrap gap-2">
                  {!predNative && prediction.state === 'ACTIVE' && (
                    <button
                      type="button"
                      onClick={() => lockPrediction(prediction.id)}
                      disabled={busy}
                      className={primaryButtonClass}
                    >
                      Lock wagers
                    </button>
                  )}
                  {!predNative && prediction.state === 'LOCKED' && (
                    <button
                      type="button"
                      onClick={() => {
                        if (!winnerId) {
                          toast.error('Pick the winning outcome first')
                          return
                        }
                        // First click arms; second click (echoing "final?") commits — the
                        // irreversible payout now has friction matching the cancel confirm (M3).
                        if (!confirmResolve) {
                          setConfirmResolve(true)
                          return
                        }
                        resolvePrediction(prediction.id)
                      }}
                      disabled={busy || !winnerId}
                      title={winnerId ? undefined : 'Select the winning outcome first'}
                      className={primaryButtonClass}
                    >
                      {!winnerLabel
                        ? 'Resolve'
                        : confirmResolve
                          ? `Pay out "${winnerLabel}" — final?`
                          : `Pay out "${winnerLabel}"`}
                    </button>
                  )}
                  {!predNative &&
                    (prediction.state === 'ACTIVE' || prediction.state === 'LOCKED') &&
                    (confirmCancel ? (
                      <button
                        type="button"
                        onClick={() => cancelPrediction(prediction.id)}
                        disabled={busy}
                        className="rounded-lg border border-destructive/50 px-3 py-1.5 text-xs font-semibold text-destructive transition-colors hover:bg-destructive/10 focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50"
                      >
                        Really refund all wagers?
                      </button>
                    ) : (
                      <button
                        type="button"
                        onClick={() => setConfirmCancel(true)}
                        disabled={busy}
                        title="Cancel and refund all wagers"
                        className={secondaryButtonClass}
                      >
                        Cancel & refund
                      </button>
                    ))}
                  {predFinished && (
                    <button
                      type="button"
                      onClick={() => {
                        setPrediction(null)
                        setWinnerId('')
                      }}
                      className={secondaryButtonClass}
                    >
                      New prediction
                    </button>
                  )}
                </div>
              </div>
              {prediction.state === 'LOCKED' && !predNative && (
                <p className="text-[11px] text-text-sub">
                  Pick the winning outcome, then pay out. Payouts are final.
                </p>
              )}
              {predNative && (
                <p className="text-[11px] text-text-sub">
                  Mirrored from Twitch — runs on Twitch channel points
                </p>
              )}
            </div>
          ) : (
            <div className="space-y-2">
              <input
                type="text"
                value={title}
                onChange={(e) => setTitle(e.target.value)}
                placeholder="Title (e.g. Will we win this round?)"
                aria-label="Prediction title"
                disabled={busy}
                className={inputClass}
              />
              <LabelListEditor
                labels={outcomes}
                onChange={setOutcomes}
                max={MAX_PREDICTION_OUTCOMES}
                placeholder="Outcome"
                disabled={busy}
              />
              <label className="flex items-center gap-1.5 text-xs text-text-sub">
                Auto-lock wagers after
                <input
                  type="number"
                  min={0}
                  value={autoLock}
                  onChange={(e) => setAutoLock(e.target.value)}
                  placeholder="∞"
                  disabled={busy}
                  className={clsx(inputClass, 'w-16')}
                />
                s
              </label>
              <div className="flex items-center justify-between gap-2">
                <span className="text-[11px] text-text-sub">
                  Viewers wager on the{' '}
                  <a
                    href={`/overlay/${overlayId}/participate`}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="underline hover:text-text focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
                  >
                    participate page
                  </a>{' '}
                  (they can see their balance) — or from chat: <code>!predict 1 500</code>
                </span>
                <button
                  type="button"
                  onClick={startPrediction}
                  disabled={busy}
                  className={primaryButtonClass}
                >
                  Start prediction
                </button>
              </div>
            </div>
          )}
        </div>
      </div>

      {/* Twitch native mirroring — a stable, always-visible control (M4), so it's
          discoverable regardless of whether a round is running (it used to be a buried
          link shown only in the poll create form). The consent flow returns here, and
          the note sets the expectation that a native round only mirrors after the next
          channel sync (M5), so a streamer who just enabled it doesn't think it's broken. */}
      <div className="mt-3 flex flex-wrap items-center justify-between gap-2 border-t border-border pt-3">
        <p className="text-[11px] text-text-sub">
          Mirror native Twitch polls &amp; predictions onto your overlays (read-only). Opt-in; takes
          effect after the next channel sync (a stream restart or re-adding the source).
        </p>
        <button
          type="button"
          onClick={() => void startMirrorConsent()}
          className={secondaryButtonClass}
        >
          Enable Twitch mirroring
        </button>
      </div>
    </section>
  )
}
