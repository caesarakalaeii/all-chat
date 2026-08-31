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
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { Plus, X } from 'lucide-react'
import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react'
import { toastManager } from '@/lib/toast'

import { ApiError } from '@/lib/api/client'
import { engagementApi } from '@/lib/api/engagement'
import { useEngagementLive } from '@/lib/hooks/useEngagementLive'
import { type TFunction, formatNumber, formatTime, useTranslations } from '@/lib/i18n'
import { interpolateElements } from '@/lib/i18n/emphasise'
import type { Poll, Prediction } from '@/lib/types/engagement'

const REFRESH_MS = 3000
const MAX_POLL_OPTIONS = 5
const MAX_PREDICTION_OUTCOMES = 10

// Chat commands are protocol, not copy: the parser matches these bytes, so a
// translation would stop the command working.
const VOTE_COMMAND = '!vote 2'
const VOTE_SHORTHAND = '2'
const PREDICT_COMMAND = '!predict 1 500'
// The unbounded-duration marker for the two optional seconds inputs. A symbol,
// not a word, so it reads the same in every language.
const NO_LIMIT_PLACEHOLDER = '∞'
// Sits beside the outcome's own "Winning outcome" aria-label, which states in
// words what the trophy shows.
const WINNER_GLYPH = '🏆'

function errorMessage(err: unknown, fallback: string): string {
  return err instanceof ApiError ? err.message : fallback
}

/**
 * Editable list of option/outcome labels for the create forms.
 *
 * `noun` is the already-resolved list noun ('Option' / 'Outcome'); it names each
 * numbered row and, lowercased, the add button.
 */
function LabelListEditor({
  labels,
  onChange,
  max,
  noun,
  disabled,
}: {
  labels: string[]
  onChange: (next: string[]) => void
  max: number
  noun: string
  disabled: boolean
}) {
  const t = useTranslations()
  return (
    <div className="space-y-1.5">
      {labels.map((label, i) => (
        <div key={i} className="flex items-center gap-1.5">
          <Input
            type="text"
            value={label}
            onChange={(e) => onChange(labels.map((l, j) => (j === i ? e.target.value : l)))}
            placeholder={t('viewerOverlay.engagement.labelListEntry', {
              noun,
              index: i + 1,
            })}
            aria-label={t('viewerOverlay.engagement.labelListEntry', { noun, index: i + 1 })}
            disabled={disabled}
            size="sm"
          />
          {labels.length > 2 && (
            <Button
              type="button"
              onClick={() => onChange(labels.filter((_, j) => j !== i))}
              disabled={disabled}
              title={t('viewerOverlay.engagement.labelListRemove')}
              variant="ghost"
              size="icon-xs"
            >
              <X className="h-3.5 w-3.5" />
            </Button>
          )}
        </div>
      ))}
      {labels.length < max && (
        <Button
          type="button"
          onClick={() => onChange([...labels, ''])}
          disabled={disabled}
          variant="ghost"
          size="xs"
          className="px-0"
        >
          <Plus className="h-3.5 w-3.5" />
          {t('viewerOverlay.engagement.labelListAdd', { noun: noun.toLowerCase() })}
        </Button>
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
function TwitchSourceBadge({ t }: { t: TFunction }) {
  return (
    <span className="rounded border border-border bg-surface px-1.5 py-0.5 text-[10px] font-semibold tracking-wide text-text-sub uppercase">
      {t('viewerOverlay.engagement.twitchSourceBadge')}
    </span>
  )
}

export function EngagementControls({ overlayId }: { overlayId: string }) {
  const t = useTranslations()
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
      // Only a genuine 404 means "no active round". A transient non-ok (5xx) must NOT blank
      // the owner's live round (P3-9) — keep the previous render for that branch.
      const pollGone = pRes.status === 404
      const predGone = prRes.status === 404
      if (mutationEpochRef.current !== epoch) return // stale — a mutation landed meanwhile
      // On 404 only clear a round that was still running (it ended elsewhere, e.g.
      // auto-close); a finished one we're displaying stays until dismissed.
      setPoll(
        (prev) => nextPoll ?? (pollGone ? (prev && prev.state !== 'ACTIVE' ? prev : null) : prev)
      )
      setPrediction(
        (prev) =>
          nextPrediction ??
          (predGone
            ? prev && prev.state !== 'ACTIVE' && prev.state !== 'LOCKED'
              ? prev
              : null
            : prev)
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
        toastManager.add({ title: errorMessage(err, failMsg), type: 'error' })
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
      toastManager.add({
        title: t('viewerOverlay.engagement.pollIncompleteToast'),
        type: 'error',
      })
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
      toastManager.add({ title: t('viewerOverlay.engagement.pollStartedToast'), type: 'success' })
    }, t('viewerOverlay.engagement.pollStartFailed'))
  }

  const closePoll = (pollId: string) =>
    void run(async () => {
      setPoll(await engagementApi.closePoll(overlayId, pollId))
      toastManager.add({ title: t('viewerOverlay.engagement.pollClosedToast'), type: 'success' })
    }, t('viewerOverlay.engagement.pollCloseFailed'))

  // --- Prediction actions ----------------------------------------------------

  const startPrediction = () => {
    const labels = outcomes.map((o) => o.trim()).filter(Boolean)
    if (!title.trim() || labels.length < 2) {
      toastManager.add({
        title: t('viewerOverlay.engagement.predictionIncompleteToast'),
        type: 'error',
      })
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
      toastManager.add({
        title: t('viewerOverlay.engagement.predictionStartedToast'),
        type: 'success',
      })
    }, t('viewerOverlay.engagement.predictionStartFailed'))
  }

  const lockPrediction = (pid: string) =>
    void run(async () => {
      setPrediction(await engagementApi.lockPrediction(overlayId, pid))
      toastManager.add({
        title: t('viewerOverlay.engagement.predictionLockedToast'),
        type: 'success',
      })
    }, t('viewerOverlay.engagement.predictionLockFailed'))

  const resolvePrediction = (pid: string) => {
    if (!winnerId) {
      toastManager.add({ title: t('viewerOverlay.engagement.pickWinnerToast'), type: 'error' })
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
        toastManager.add({ title: t('viewerOverlay.engagement.predictionNoLongerLockedToast') })
        return
      }
      setPrediction(resolved)
      setWinnerId('')
      toastManager.add({
        title: t('viewerOverlay.engagement.predictionResolvedToast'),
        type: 'success',
      })
    }, t('viewerOverlay.engagement.predictionResolveFailed'))
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
        toastManager.add({
          title: t('viewerOverlay.engagement.predictionCanceledToast'),
          type: 'success',
        })
      } else {
        toastManager.add({
          title: t('viewerOverlay.engagement.nothingToCancelToast', {
            state: result.state.toLowerCase(),
          }),
          type: 'error',
        })
      }
    }, t('viewerOverlay.engagement.predictionCancelFailed'))

  // --- Twitch mirroring opt-in ------------------------------------------------

  const startMirrorConsent = useCallback(async () => {
    try {
      window.location.href = await engagementApi.getTwitchMirrorConsentUrl(overlayId)
    } catch {
      toastManager.add({
        title: t('viewerOverlay.engagement.twitchConsentFailedToast'),
        type: 'error',
      })
    }
  }, [overlayId, t])

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

  // Reset the winner selection and armed confirmations when the ACTIVE round identity changes
  // underneath us (e.g. a fast LOCKED round A -> LOCKED round B handoff via refresh()). Otherwise
  // a winnerId from the previous round keeps Resolve enabled and would post a foreign outcome id.
  //
  // Done during render, not in an effect: an effect would be one setState per state and a whole
  // extra committed render with the stale winner still armed, which is what
  // react-hooks/set-state-in-effect is about. React re-runs this render before painting.
  const [armedRoundId, setArmedRoundId] = useState(prediction?.id)
  if (prediction?.id !== armedRoundId) {
    setArmedRoundId(prediction?.id)
    setWinnerId('')
    setConfirmResolve(false)
    setConfirmCancel(false)
  }

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
            <h3 className="text-xs font-semibold tracking-wide text-text-sub uppercase">
              {t('viewerOverlay.engagement.pollHeading')}
            </h3>
            {poll && (
              <span className="flex items-center gap-1.5">
                {pollNative && <TwitchSourceBadge t={t} />}
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
                  detail={`${pollTotal > 0 ? Math.round((o.votes / pollTotal) * 100) : 0}% (${formatNumber(o.votes)})`}
                  pct={pollTotal > 0 ? Math.round((o.votes / pollTotal) * 100) : 0}
                  accent="twitch"
                />
              ))}
              <div className="flex flex-wrap items-center justify-between gap-2 pt-1">
                <span className="text-[11px] text-text-sub">
                  {t('viewerOverlay.engagement.pollVotes', { total: formatNumber(pollTotal) })}
                  {poll.ends_at &&
                    poll.state === 'ACTIVE' &&
                    t('viewerOverlay.engagement.pollAutoCloses', {
                      time: formatTime(new Date(poll.ends_at)),
                    })}
                </span>
                {pollFinished ? (
                  <Button type="button" onClick={() => setPoll(null)} variant="outline" size="xs">
                    {t('viewerOverlay.engagement.pollNew')}
                  </Button>
                ) : (
                  !pollNative && (
                    <Button
                      type="button"
                      onClick={() => closePoll(poll.id)}
                      disabled={busy}
                      size="xs"
                    >
                      {t('viewerOverlay.engagement.pollClose')}
                    </Button>
                  )
                )}
              </div>
              {pollNative && (
                <p className="text-[11px] text-text-sub">
                  {t('viewerOverlay.engagement.pollMirroredNote')}
                </p>
              )}
            </div>
          ) : (
            <div className="space-y-2">
              <Input
                type="text"
                value={question}
                onChange={(e) => setQuestion(e.target.value)}
                placeholder={t('viewerOverlay.engagement.pollQuestionPlaceholder')}
                aria-label={t('viewerOverlay.engagement.pollQuestionLabel')}
                disabled={busy}
                size="sm"
              />
              <LabelListEditor
                labels={options}
                onChange={setOptions}
                max={MAX_POLL_OPTIONS}
                noun={t('viewerOverlay.engagement.pollOptionNoun')}
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
                  {t('viewerOverlay.engagement.pollAllowChange')}
                </label>
                <label className="flex items-center gap-1.5 text-xs text-text-sub">
                  {t('viewerOverlay.engagement.pollAutoCloseAfter')}
                  <Input
                    type="number"
                    min={0}
                    value={pollDuration}
                    onChange={(e) => setPollDuration(e.target.value)}
                    placeholder={NO_LIMIT_PLACEHOLDER}
                    disabled={busy}
                    size="sm"
                    className="w-16"
                  />
                  {t('viewerOverlay.engagement.secondsSuffix')}
                </label>
              </div>
              <div className="flex items-center justify-between gap-2">
                <span className="text-[11px] text-text-sub">
                  {interpolateElements(t('viewerOverlay.engagement.pollParticipateHint'), {
                    link: (
                      <a
                        href={`/overlay/${overlayId}/participate`}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="underline hover:text-text focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
                      >
                        {t('viewerOverlay.engagement.participateLink')}
                      </a>
                    ),
                    voteCommand: <code>{VOTE_COMMAND}</code>,
                    shortCommand: <code>{VOTE_SHORTHAND}</code>,
                  })}
                </span>
                <Button type="button" onClick={startPoll} disabled={busy} size="xs">
                  {t('viewerOverlay.engagement.pollStart')}
                </Button>
              </div>
            </div>
          )}
        </div>

        {/* Prediction column */}
        <div className="rounded-lg border border-border bg-surface-2 p-3">
          <div className="mb-2 flex items-center justify-between">
            <h3 className="text-xs font-semibold tracking-wide text-text-sub uppercase">
              {t('viewerOverlay.engagement.predictionHeading')}
            </h3>
            {prediction && (
              <span className="flex items-center gap-1.5">
                {predNative && <TwitchSourceBadge t={t} />}
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
                  prediction.state === 'LOCKED' && !predNative
                    ? t('viewerOverlay.engagement.winningOutcome')
                    : undefined
                }
              >
                {prediction.outcomes.map((o) => (
                  <TallyBar
                    key={o.id}
                    label={`${o.idx}. ${o.label}`}
                    detail={t('viewerOverlay.engagement.predictionOutcomeTally', {
                      points: formatNumber(o.total_points),
                      entrants: formatNumber(o.entrants),
                    })}
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
                          aria-label={t('viewerOverlay.engagement.winningOutcomeChoice', {
                            label: o.label,
                          })}
                          className="accent-twitch focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
                        />
                      ) : prediction.winning_outcome_id === o.id ? (
                        <span
                          title={t('viewerOverlay.engagement.winningOutcome')}
                          aria-label={t('viewerOverlay.engagement.winningOutcome')}
                        >
                          {WINNER_GLYPH}
                        </span>
                      ) : undefined
                    }
                  />
                ))}
              </div>
              <div className="flex flex-wrap items-center justify-between gap-2 pt-1">
                <span className="text-[11px] text-text-sub">
                  {t('viewerOverlay.engagement.predictionPointsWagered', {
                    total: formatNumber(predTotal),
                  })}
                  {prediction.auto_lock_at &&
                    prediction.state === 'ACTIVE' &&
                    t('viewerOverlay.engagement.predictionAutoLocks', {
                      time: formatTime(new Date(prediction.auto_lock_at)),
                    })}
                </span>
                <div className="flex flex-wrap gap-2">
                  {!predNative && prediction.state === 'ACTIVE' && (
                    <Button
                      type="button"
                      onClick={() => lockPrediction(prediction.id)}
                      disabled={busy}
                      size="xs"
                    >
                      {t('viewerOverlay.engagement.predictionLock')}
                    </Button>
                  )}
                  {!predNative && prediction.state === 'LOCKED' && (
                    <Button
                      type="button"
                      onClick={() => {
                        if (!winnerId) {
                          toastManager.add({
                            title: t('viewerOverlay.engagement.pickWinnerToast'),
                            type: 'error',
                          })
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
                      title={
                        winnerId
                          ? undefined
                          : t('viewerOverlay.engagement.predictionResolveDisabledTitle')
                      }
                      size="xs"
                    >
                      {!winnerLabel
                        ? t('viewerOverlay.engagement.predictionResolve')
                        : confirmResolve
                          ? t('viewerOverlay.engagement.predictionPayOutConfirm', {
                              label: winnerLabel,
                            })
                          : t('viewerOverlay.engagement.predictionPayOut', { label: winnerLabel })}
                    </Button>
                  )}
                  {!predNative &&
                    (prediction.state === 'ACTIVE' || prediction.state === 'LOCKED') &&
                    (confirmCancel ? (
                      <Button
                        type="button"
                        onClick={() => cancelPrediction(prediction.id)}
                        disabled={busy}
                        variant="destructive"
                        size="xs"
                      >
                        {t('viewerOverlay.engagement.predictionCancelConfirm')}
                      </Button>
                    ) : (
                      <Button
                        type="button"
                        onClick={() => setConfirmCancel(true)}
                        disabled={busy}
                        title={t('viewerOverlay.engagement.predictionCancelTitle')}
                        variant="outline"
                        size="xs"
                      >
                        {t('viewerOverlay.engagement.predictionCancel')}
                      </Button>
                    ))}
                  {predFinished && (
                    <Button
                      type="button"
                      onClick={() => {
                        setPrediction(null)
                        setWinnerId('')
                      }}
                      variant="outline"
                      size="xs"
                    >
                      {t('viewerOverlay.engagement.predictionNew')}
                    </Button>
                  )}
                </div>
              </div>
              {prediction.state === 'LOCKED' && !predNative && (
                <p className="text-[11px] text-text-sub">
                  {t('viewerOverlay.engagement.predictionLockedNote')}
                </p>
              )}
              {predNative && (
                <p className="text-[11px] text-text-sub">
                  {t('viewerOverlay.engagement.predictionMirroredNote')}
                </p>
              )}
            </div>
          ) : (
            <div className="space-y-2">
              <Input
                type="text"
                value={title}
                onChange={(e) => setTitle(e.target.value)}
                placeholder={t('viewerOverlay.engagement.predictionTitlePlaceholder')}
                aria-label={t('viewerOverlay.engagement.predictionTitleLabel')}
                disabled={busy}
                size="sm"
              />
              <LabelListEditor
                labels={outcomes}
                onChange={setOutcomes}
                max={MAX_PREDICTION_OUTCOMES}
                noun={t('viewerOverlay.engagement.predictionOutcomeNoun')}
                disabled={busy}
              />
              <label className="flex items-center gap-1.5 text-xs text-text-sub">
                {t('viewerOverlay.engagement.predictionAutoLockAfter')}
                <Input
                  type="number"
                  min={0}
                  value={autoLock}
                  onChange={(e) => setAutoLock(e.target.value)}
                  placeholder={NO_LIMIT_PLACEHOLDER}
                  disabled={busy}
                  size="sm"
                  className="w-16"
                />
                {t('viewerOverlay.engagement.secondsSuffix')}
              </label>
              <div className="flex items-center justify-between gap-2">
                <span className="text-[11px] text-text-sub">
                  {interpolateElements(t('viewerOverlay.engagement.predictionParticipateHint'), {
                    link: (
                      <a
                        href={`/overlay/${overlayId}/participate`}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="underline hover:text-text focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
                      >
                        {t('viewerOverlay.engagement.participateLink')}
                      </a>
                    ),
                    predictCommand: <code>{PREDICT_COMMAND}</code>,
                  })}
                </span>
                <Button type="button" onClick={startPrediction} disabled={busy} size="xs">
                  {t('viewerOverlay.engagement.predictionStart')}
                </Button>
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
        <p className="text-[11px] text-text-sub">{t('viewerOverlay.engagement.mirrorNote')}</p>
        <Button type="button" onClick={() => void startMirrorConsent()} variant="outline" size="xs">
          {t('viewerOverlay.engagement.mirrorEnable')}
        </Button>
      </div>
    </section>
  )
}
