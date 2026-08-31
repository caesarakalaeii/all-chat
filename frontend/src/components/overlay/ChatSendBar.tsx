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
import { useTranslations, type TFunction } from '@/lib/i18n'
import type { SourceCapability } from '@/lib/types/moderation'

/** Platforms the send bar supports (TikTok/Discord are hidden — no send path). */
const SENDABLE_PLATFORM_NAMES = ['twitch', 'youtube', 'kick'] as const
const SENDABLE_PLATFORMS: ReadonlySet<string> = new Set(SENDABLE_PLATFORM_NAMES)

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

/**
 * Display name for a platform, from the shared catalog.
 *
 * A send-to-all result can name a platform we do not know, so an unrecognised
 * value falls through as-is rather than rendering a key that echoes itself.
 */
function platformLabel(t: TFunction, platform: string): string {
  const known = SENDABLE_PLATFORM_NAMES.find((name) => name === platform)
  return known ? t(`common.platforms.${known}`) : platform
}

/**
 * One word for a send-to-all per-platform `error_kind`.
 *
 * An unrecognised kind is shown verbatim: it is more use to the streamer than
 * nothing, and the backend can add kinds without a frontend release.
 */
function humanizeErrorKind(t: TFunction, kind: string): string {
  switch (kind) {
    case 'reauth_required':
      return t('viewerOverlay.chatSend.reasonReauthRequired')
    case 'missing_scope':
      return t('viewerOverlay.chatSend.reasonMissingScope')
    case 'stream_offline':
      return t('viewerOverlay.chatSend.reasonStreamOffline')
    case 'quota_exhausted':
      return t('viewerOverlay.chatSend.reasonQuotaExhausted')
    case 'send_failed':
      return t('viewerOverlay.chatSend.reasonSendFailed')
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
  const t = useTranslations()
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

  const targetPlatform: SendPlatform = selection.kind === 'all' ? 'all' : selection.platform

  const handleError = (err: unknown) => {
    if (!(err instanceof ApiError)) {
      setFeedback({ kind: 'error', text: t('viewerOverlay.chatSend.sendFailed') })
      return
    }
    const body = (err.data ?? {}) as Partial<SendErrorBody>
    const code = body.error
    const platform = body.platform

    if (code === 'missing_scope') {
      setFeedback({
        kind: 'error',
        text: platform
          ? t('viewerOverlay.chatSend.missingScopeFor', {
              platform: platformLabel(t, platform),
            })
          : t('viewerOverlay.chatSend.missingScope'),
        platform,
        action: 'enable',
      })
      return
    }
    if (code === 'reauth_required') {
      setFeedback({
        kind: 'error',
        text: platform
          ? t('viewerOverlay.chatSend.reauthRequired', { platform: platformLabel(t, platform) })
          : t('viewerOverlay.chatSend.reauthRequiredGeneric'),
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
          ? t('viewerOverlay.chatSend.rateLimitedRetry', { seconds: retry })
          : t('viewerOverlay.chatSend.rateLimited'),
      })
      return
    }
    if (err.status === 422) {
      setFeedback({
        kind: 'error',
        text: body.details || t('viewerOverlay.chatSend.streamOffline'),
      })
      return
    }
    setFeedback({
      kind: 'error',
      text: body.details || err.message || t('viewerOverlay.chatSend.sendFailed'),
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
        const parts = res.results.map((r) => {
          const platform = platformLabel(t, r.platform)
          if (r.success) return t('viewerOverlay.chatSend.resultOk', { platform })
          return r.error_kind
            ? t('viewerOverlay.chatSend.resultFailedWhy', {
                platform,
                why: humanizeErrorKind(t, r.error_kind),
              })
            : t('viewerOverlay.chatSend.resultFailed', { platform })
        })
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
        setFeedback({ kind: 'success', text: t('viewerOverlay.chatSend.sent') })
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
        <div
          role="radiogroup"
          aria-label={t('viewerOverlay.chatSend.targetGroupLabel')}
          className="flex flex-wrap items-center gap-1.5"
        >
          {platformSources.map((s) => {
            const platform = s.platform as SingleSendPlatform
            const enabled = s.can_send === true
            const active =
              enabled && selection.kind === 'platform' && selection.platform === platform
            return (
              <button
                key={platform}
                type="button"
                role="radio"
                aria-checked={active}
                aria-label={
                  enabled
                    ? t('viewerOverlay.chatSend.sendToPlatform', {
                        platform: platformLabel(t, platform),
                      })
                    : t('viewerOverlay.chatSend.enableSendingFor', {
                        platform: platformLabel(t, platform),
                      })
                }
                title={
                  enabled
                    ? platformLabel(t, platform)
                    : t('viewerOverlay.chatSend.enableSendingFor', {
                        platform: platformLabel(t, platform),
                      })
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
                <span>{platformLabel(t, platform)}</span>
                {!enabled && <Lock className="h-3 w-3" aria-hidden="true" />}
              </button>
            )
          })}

          {showAll && (
            <button
              type="button"
              role="radio"
              aria-checked={selection.kind === 'all'}
              aria-label={t('viewerOverlay.chatSend.allLabel')}
              onClick={() => setSelection({ kind: 'all' })}
              className={clsx(
                'rounded-full border px-2.5 py-1 text-xs font-medium transition-colors focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none',
                selection.kind === 'all'
                  ? 'border-twitch bg-twitch/15 text-text'
                  : 'border-border text-text-sub hover:border-border-md hover:text-text'
              )}
            >
              {t('viewerOverlay.chatSend.allText')}
            </button>
          )}
        </div>

        {/* Message input + send */}
        <div className="flex min-w-0 flex-1 items-center gap-2">
          <label htmlFor={inputId} className="sr-only">
            {t('viewerOverlay.chatSend.messageLabel')}
          </label>
          <input
            id={inputId}
            type="text"
            value={text}
            onChange={(e) => setText(e.target.value)}
            maxLength={MAX_MESSAGE_LENGTH}
            disabled={sending}
            placeholder={
              selection.kind === 'all'
                ? t('viewerOverlay.chatSend.placeholderAll')
                : t('viewerOverlay.chatSend.placeholderOne')
            }
            autoComplete="off"
            className="min-w-0 flex-1 rounded-lg border border-border bg-bg px-3 py-1.5 text-sm text-text placeholder:text-text-dim focus-visible:border-border-md focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none disabled:opacity-60"
          />
          <button
            type="submit"
            disabled={!canSubmit}
            className="flex shrink-0 items-center gap-1.5 rounded-lg bg-twitch px-3 py-1.5 text-xs font-semibold text-bg transition-opacity hover:opacity-90 focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50"
          >
            <Send className="h-3.5 w-3.5" />
            {t('viewerOverlay.chatSend.send')}
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
              {t('viewerOverlay.chatSend.enableSending')}
            </button>
          )}
          {feedback.kind === 'error' && feedback.action === 'reauth' && (
            <button
              type="button"
              onClick={() => onReauth(feedback.platform)}
              className="font-semibold text-twitch hover:underline focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
            >
              {t('viewerOverlay.chatSend.reconnect')}
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
                  ? t('viewerOverlay.chatSend.reconnectPlatform', {
                      platform: platformLabel(t, a.platform),
                    })
                  : t('viewerOverlay.chatSend.enablePlatform', {
                      platform: platformLabel(t, a.platform),
                    })}
              </button>
            ))}
        </div>
      )}
    </form>
  )
}
