'use client'

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

import clsx from 'clsx'
import { Lock, Send } from 'lucide-react'
import { useId, useMemo, useState } from 'react'

import { PlatformGlyph } from '@/components/overlay/PlatformGlyph'
import { ApiError } from '@/lib/api/client'
import {
  MAX_MESSAGE_LENGTH,
  isSendToAllResponse,
  sendStreamerMessage,
  type SendErrorBody,
  type SendPlatform,
} from '@/lib/api/chat'
import type { SourceCapability } from '@/lib/types/moderation'

/** Platforms the send bar supports (TikTok/Discord are hidden — no send path). */
const SENDABLE_PLATFORMS: ReadonlySet<string> = new Set(['twitch', 'youtube', 'kick'])

/** A platform string the send endpoint accepts as a single target. */
type SingleSendPlatform = Exclude<SendPlatform, 'all'>

/** Currently-selected target: a single source's platform, or fan-out to all. */
type Selection = { kind: 'platform'; platform: SingleSendPlatform } | { kind: 'all' }

interface ChatSendBarProps {
  /** All moderation/source capabilities for the overlay. */
  sources: SourceCapability[]
  /** Re-consent / bot-reinvite flow (shared with moderation) to grant scopes. */
  onEnable: (platform: string) => void
  /** Re-login entry when the platform OAuth token was revoked (`reauth_required`). */
  onReauth: (platform?: string) => void
}

/** Pretty platform label for inline feedback. */
function platformLabel(platform: string): string {
  switch (platform) {
    case 'twitch':
      return 'Twitch'
    case 'youtube':
      return 'YouTube'
    case 'kick':
      return 'Kick'
    default:
      return platform
  }
}

/** Human-readable label for a send-to-all per-platform `error_kind`. */
function humanizeErrorKind(kind: string): string {
  switch (kind) {
    case 'reauth_required':
      return 'reconnect'
    case 'missing_scope':
      return 'locked'
    case 'stream_offline':
      return 'offline'
    case 'quota_exhausted':
      return 'quota'
    case 'send_failed':
      return 'failed'
    default:
      return kind
  }
}

type FeedbackAction = { platform: string; action: 'enable' | 'reauth' }

type Feedback =
  | { kind: 'success'; text: string }
  | { kind: 'partial'; text: string; actions?: FeedbackAction[] }
  | { kind: 'error'; text: string; platform?: string; action?: 'enable' | 'reauth' }

/**
 * Bottom send bar for the overlay monitor. Lets the overlay owner post a chat
 * message to one platform or fan it out to all sendable platforms. A source's
 * pill is enabled only when the backend reports `can_send`; clicking a disabled
 * pill kicks off the consent flow (which also grants send scopes, ADR-0017).
 */
export function ChatSendBar({ sources, onEnable, onReauth }: ChatSendBarProps) {
  const inputId = useId()

  // Sendable sources in a stable order, deduped by platform (one pill per platform).
  const platformSources = useMemo(() => {
    const byPlatform = new Map<SingleSendPlatform, SourceCapability>()
    for (const s of sources) {
      if (!SENDABLE_PLATFORMS.has(s.platform)) continue
      const platform = s.platform as SingleSendPlatform
      // Prefer a sendable source if several share a platform.
      const existing = byPlatform.get(platform)
      if (!existing || (!existing.can_send && s.can_send)) byPlatform.set(platform, s)
    }
    return Array.from(byPlatform.values())
  }, [sources])

  const sendablePlatforms = useMemo(
    () => platformSources.filter((s) => s.can_send === true).map((s) => s.platform),
    [platformSources]
  )

  const [selection, setSelection] = useState<Selection>(() =>
    sendablePlatforms.length > 1
      ? { kind: 'all' }
      : sendablePlatforms.length === 1
        ? { kind: 'platform', platform: sendablePlatforms[0] as SingleSendPlatform }
        : { kind: 'all' }
  )
  const [text, setText] = useState('')
  const [sending, setSending] = useState(false)
  const [feedback, setFeedback] = useState<Feedback | null>(null)

  const showAll = sendablePlatforms.length > 1
  const trimmed = text.trim()
  const canSubmit =
    !sending &&
    trimmed.length > 0 &&
    (selection.kind === 'all'
      ? sendablePlatforms.length > 0
      : sendablePlatforms.includes(selection.platform))

  const handleDisabledPill = (platform: SingleSendPlatform) => {
    // A disabled pill means the scope isn't granted yet — start consent for it.
    setFeedback(null)
    onEnable(platform)
  }

  const targetPlatform: SendPlatform =
    selection.kind === 'all' ? 'all' : selection.platform

  const handleError = (err: unknown) => {
    if (!(err instanceof ApiError)) {
      setFeedback({ kind: 'error', text: 'Could not send. Please try again.' })
      return
    }
    const body = (err.data ?? {}) as Partial<SendErrorBody>
    const code = body.error
    const platform = body.platform

    if (code === 'missing_scope') {
      setFeedback({
        kind: 'error',
        text: `Sending isn't enabled${platform ? ` for ${platformLabel(platform)}` : ''} yet.`,
        platform,
        action: 'enable',
      })
      return
    }
    if (code === 'reauth_required') {
      setFeedback({
        kind: 'error',
        text: `Your ${platform ? platformLabel(platform) : 'platform'} login expired. Please reconnect.`,
        platform,
        action: 'reauth',
      })
      return
    }
    if (err.status === 429) {
      const retry = body.retry_after_seconds
      setFeedback({
        kind: 'error',
        text: retry
          ? `Rate limited — try again in ${retry}s.`
          : 'Rate limited — please slow down.',
      })
      return
    }
    if (err.status === 422) {
      setFeedback({
        kind: 'error',
        text: body.details || 'That channel is not live right now.',
      })
      return
    }
    setFeedback({
      kind: 'error',
      text: body.details || err.message || 'Could not send. Please try again.',
    })
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!canSubmit) return
    setSending(true)
    setFeedback(null)
    try {
      const res = await sendStreamerMessage({ message: trimmed, platform: targetPlatform })
      if (isSendToAllResponse(res)) {
        const parts = res.results.map(
          (r) =>
            `${platformLabel(r.platform)} ${r.success ? '✓' : `✗${r.error_kind ? ` ${humanizeErrorKind(r.error_kind)}` : ''}`}`
        )
        const allOk = res.results.every((r) => r.success)
        // Surface an actionable button per failed platform that needs a user
        // fix (reauth or enable). Without this the partial line just prints
        // `reauth_required` with no way to start the flow (the single-send
        // path handles these via thrown ApiError; send-to-all returns 200).
        const actions: FeedbackAction[] = res.results
          .filter((r) => !r.success)
          .map((r) => {
            if (r.error_kind === 'reauth_required')
              return { platform: r.platform, action: 'reauth' as const }
            if (r.error_kind === 'missing_scope')
              return { platform: r.platform, action: 'enable' as const }
            return null
          })
          .filter((a): a is FeedbackAction => a !== null)
        setFeedback({
          kind: allOk ? 'success' : 'partial',
          text: parts.join(' · '),
          ...(actions.length ? { actions } : {}),
        })
      } else {
        setFeedback({ kind: 'success', text: 'Sent ✓' })
      }
      setText('')
    } catch (err) {
      handleError(err)
    } finally {
      setSending(false)
    }
  }

  return (
    <form
      onSubmit={handleSubmit}
      className="flex flex-col gap-2 border-t border-border bg-surface px-4 py-2.5"
      aria-label="Send a chat message"
    >
      <div className="flex flex-wrap items-center gap-2">
        {/* Platform target pills */}
        <div role="radiogroup" aria-label="Send to" className="flex flex-wrap items-center gap-1.5">
          {platformSources.map((s) => {
            const platform = s.platform as SingleSendPlatform
            const enabled = s.can_send === true
            const active = enabled && selection.kind === 'platform' && selection.platform === platform
            return (
              <button
                key={platform}
                type="button"
                role="radio"
                aria-checked={active}
                aria-label={
                  enabled
                    ? `Send to ${platformLabel(platform)}`
                    : `Enable sending for ${platformLabel(platform)}`
                }
                title={
                  enabled
                    ? platformLabel(platform)
                    : `Enable sending for ${platformLabel(platform)}`
                }
                onClick={() =>
                  enabled
                    ? setSelection({ kind: 'platform', platform })
                    : handleDisabledPill(platform)
                }
                className={clsx(
                  'flex items-center gap-1.5 rounded-full border px-2.5 py-1 text-xs font-medium transition-colors focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none',
                  active
                    ? 'border-twitch bg-twitch/15 text-text'
                    : enabled
                      ? 'border-border text-text-sub hover:border-border-md hover:text-text'
                      : 'border-border/60 text-text-dim hover:border-border-md hover:text-text-sub'
                )}
              >
                <PlatformGlyph platform={platform} className="h-3.5 w-3.5" />
                <span>{platformLabel(platform)}</span>
                {!enabled && <Lock className="h-3 w-3" aria-hidden="true" />}
              </button>
            )
          })}

          {showAll && (
            <button
              type="button"
              role="radio"
              aria-checked={selection.kind === 'all'}
              aria-label="Send to all platforms"
              onClick={() => setSelection({ kind: 'all' })}
              className={clsx(
                'rounded-full border px-2.5 py-1 text-xs font-medium transition-colors focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none',
                selection.kind === 'all'
                  ? 'border-twitch bg-twitch/15 text-text'
                  : 'border-border text-text-sub hover:border-border-md hover:text-text'
              )}
            >
              All
            </button>
          )}
        </div>

        {/* Message input + send */}
        <div className="flex min-w-0 flex-1 items-center gap-2">
          <label htmlFor={inputId} className="sr-only">
            Chat message
          </label>
          <input
            id={inputId}
            type="text"
            value={text}
            onChange={(e) => setText(e.target.value)}
            maxLength={MAX_MESSAGE_LENGTH}
            disabled={sending}
            placeholder={
              selection.kind === 'all' ? 'Message all platforms…' : 'Send a message…'
            }
            autoComplete="off"
            className="min-w-0 flex-1 rounded-lg border border-border bg-bg px-3 py-1.5 text-sm text-text placeholder:text-text-dim focus-visible:border-border-md focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none disabled:opacity-60"
          />
          <button
            type="submit"
            disabled={!canSubmit}
            className="flex shrink-0 items-center gap-1.5 rounded-lg bg-twitch px-3 py-1.5 text-xs font-semibold text-white transition-opacity hover:opacity-90 focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50"
          >
            <Send className="h-3.5 w-3.5" />
            Send
          </button>
        </div>
      </div>

      {/* Inline feedback (success / partial / error). */}
      {feedback && (
        <div
          role="status"
          aria-live="polite"
          className={clsx(
            'flex flex-wrap items-center gap-2 text-xs',
            feedback.kind === 'success' && 'text-kick',
            feedback.kind === 'partial' && 'text-text-sub',
            feedback.kind === 'error' && 'text-youtube'
          )}
        >
          <span>{feedback.text}</span>
          {feedback.kind === 'error' && feedback.action === 'enable' && (
            <button
              type="button"
              onClick={() => feedback.platform && onEnable(feedback.platform)}
              className="font-semibold text-twitch hover:underline focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
            >
              Enable sending
            </button>
          )}
          {feedback.kind === 'error' && feedback.action === 'reauth' && (
            <button
              type="button"
              onClick={() => onReauth(feedback.platform)}
              className="font-semibold text-twitch hover:underline focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
            >
              Reconnect
            </button>
          )}
          {feedback.kind === 'partial' &&
            feedback.actions?.map((a) => (
              <button
                key={`${a.platform}-${a.action}`}
                type="button"
                onClick={() =>
                  a.action === 'reauth' ? onReauth(a.platform) : onEnable(a.platform)
                }
                className="font-semibold text-twitch hover:underline focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
              >
                {a.action === 'reauth'
                  ? `Reconnect ${platformLabel(a.platform)}`
                  : `Enable ${platformLabel(a.platform)}`}
              </button>
            ))}
        </div>
      )}
    </form>
  )
}
