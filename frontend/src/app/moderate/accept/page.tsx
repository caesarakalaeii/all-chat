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

/**
 * Delegation invite acceptance (ADR-0048).
 *
 * The invite secret arrives in the URL because that is the only way a streamer can hand
 * it over, but it never leaves this page in one: both calls put it in a POST body, so it
 * stays out of access logs, proxy logs and `Referer` headers on every hop after this one.
 *
 * Preview deliberately returns no `overlay_id`. An overlay UUID already grants chat READ
 * to anyone holding it, so it is disclosed on acceptance rather than to everyone who
 * merely opens the link — which means "accept" is the first moment this page knows where
 * to send the moderator.
 *
 * Accepting costs the moderator nothing: consent for each platform is deferred to the
 * first time they actually try to moderate on it, so nobody faces a stack of OAuth
 * screens before they have done anything.
 */

import { Suspense, useCallback, useEffect, useState } from 'react'
import Link from 'next/link'
import { useRouter, useSearchParams } from 'next/navigation'
import { Info, ShieldCheck } from 'lucide-react'

import { AppNav } from '@/components/AppNav'
import { useHydrated } from '@/hooks/useHydrated'
import { useAuthStore } from '@/lib/stores/auth-store'
import { InfinityLogo } from '@/components/InfinityLogo'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { PlatformBadge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import { VisuallyHidden } from '@/components/ui/visually-hidden'
import { boundInviteAccount, delegationErrorCode, moderationApi } from '@/lib/api/moderation'
import {
  DELEGATABLE_ACTIONS,
  type InvitePreview,
  type ModerationAction,
} from '@/lib/types/moderation'

const ACTION_LABELS: Record<ModerationAction, string> = {
  delete: 'Delete messages',
  timeout: 'Time viewers out',
  ban: 'Ban viewers',
  unban: 'Lift bans and timeouts',
}

/**
 * Human copy for a failed preview or accept, keyed on the machine-readable `code` rather
 * than the message text — the copy differs by role server-side and is free to change.
 *
 * Every branch ends somewhere the reader can act. "Not found" covers unknown, already
 * redeemed and revoked alike, which the server keeps deliberately indistinguishable, so
 * the copy names all three rather than guessing one.
 */
function inviteErrorMessage(err: unknown): string {
  switch (delegationErrorCode(err)) {
    case 'invite_not_found':
      return 'This invite is not valid any more — it may already have been used, or the streamer may have withdrawn it. Ask them for a new one.'
    case 'invite_expired':
      return 'This invite has expired. Ask the streamer for a new one.'
    case 'already_moderator':
      return 'You already moderate this channel. It is on your channels page.'
    case 'owner_cannot_accept':
      return 'This is your own overlay — you already have full moderation on it.'
    case 'invite_bound_to_other_account': {
      const bound = boundInviteAccount(err)
      const account = bound?.account ? ` (${bound.account})` : ''
      const platform = bound?.platform ? ` ${bound.platform}` : ''
      return `This invite is for a specific${platform} account${account}. Sign in as that account, or ask the streamer to send a new invite for this one.`
    }
    default:
      return 'Could not open this invite. Check the link and try again.'
  }
}

function InvitePreviewSkeleton() {
  return (
    <div role="status" className="space-y-4 rounded-xl border border-border bg-surface p-6">
      <VisuallyHidden>Loading invite</VisuallyHidden>
      <Skeleton className="h-5 w-2/3" />
      <Skeleton className="h-3 w-1/2" />
      <div className="flex gap-1.5">
        <Skeleton className="h-4 w-16 rounded-full" />
        <Skeleton className="h-4 w-16 rounded-full" />
      </div>
    </div>
  )
}

function DeadEnd({ message }: { message: string }) {
  return (
    <Card className="p-6">
      <div className="flex items-start gap-3">
        <Info className="mt-0.5 size-5 shrink-0 text-text-dim" aria-hidden="true" />
        <div className="space-y-4">
          <p className="text-sm text-text">{message}</p>
          <Link
            href="/moderate"
            className="inline-flex w-fit items-center rounded-lg border border-border px-3 py-1.5 text-sm font-medium text-text-sub transition-colors hover:border-border-md hover:text-text focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
          >
            Go to your channels
          </Link>
        </div>
      </div>
    </Card>
  )
}

/**
 * The signed-out state.
 *
 * Deliberately NOT `ProtectedRoute`, which bounces to the homepage with `router.push('/')`
 * and takes the invite secret with it. The URL is the only copy of that secret the
 * recipient has, so this branch leaves it exactly where it is and tells them what to do —
 * the sign-in flow has no return-to for a client-chosen destination, and inventing one by
 * stashing the secret in browser storage would put a live moderation credential somewhere
 * it does not need to be.
 */
function SignInPrompt() {
  return (
    <Card className="p-6">
      <div className="flex items-start gap-3">
        <Info className="mt-0.5 size-5 shrink-0 text-text-dim" aria-hidden="true" />
        <div className="space-y-4">
          <div className="space-y-1">
            <p className="text-sm font-semibold text-text">Sign in to accept this invite</p>
            <p className="text-sm text-text-sub">
              Moderating is tied to an All-Chat account, so we need to know which one to hand this
              to. Sign in, then open the invite link again — it stays valid.
            </p>
          </div>
          <Link
            href="/"
            className="inline-flex w-fit items-center rounded-lg border border-border px-3 py-1.5 text-sm font-medium text-text-sub transition-colors hover:border-border-md hover:text-text focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
          >
            Sign in
          </Link>
        </div>
      </div>
    </Card>
  )
}

function AcceptContent() {
  const router = useRouter()
  const token = useSearchParams().get('token') ?? ''
  const { user, loading, init } = useAuthStore()
  const isHydrated = useHydrated()

  const [preview, setPreview] = useState<InvitePreview | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [accepting, setAccepting] = useState(false)

  useEffect(() => {
    if (isHydrated) init()
  }, [isHydrated, init])

  const loadPreview = useCallback(() => moderationApi.previewInvite(token), [token])

  // Preview has no side effects, so it is safe to run on load; acceptance is a deliberate
  // click. Both endpoints require a session, so this waits for one rather than turning a
  // valid invite into a 401. State is set from the promise callback rather than the effect
  // body, with a `cancelled` flag so a slow response cannot land after unmount.
  useEffect(() => {
    if (!user) return
    if (!token) {
      // eslint-disable-next-line react-hooks/set-state-in-effect -- terminal state for a malformed link, not a fetch result
      setError('This link is missing its invite code. Ask the streamer to send it again.')
      return
    }
    let cancelled = false
    loadPreview()
      .then((p) => {
        if (!cancelled) setPreview(p)
      })
      .catch((err) => {
        if (!cancelled) setError(inviteErrorMessage(err))
      })
    return () => {
      cancelled = true
    }
  }, [user, token, loadPreview])

  const handleAccept = async () => {
    setAccepting(true)
    setError(null)
    try {
      // The overlay id exists only from here on — the preview withholds it.
      const accepted = await moderationApi.acceptInvite(token)
      router.push(`/overlay/${accepted.overlay_id}/view`)
    } catch (err) {
      setError(inviteErrorMessage(err))
      setAccepting(false)
    }
  }

  const actions = preview ? DELEGATABLE_ACTIONS.filter((a) => preview.actions.includes(a)) : []
  const platforms = preview?.platforms.filter((leg) => leg.enabled) ?? []

  return (
    <div className="min-h-screen bg-bg">
      <AppNav />
      <main
        id="main-content"
        tabIndex={-1}
        className="mx-auto max-w-2xl px-4 py-12 sm:px-6 lg:px-8"
      >
        <div className="mb-8 flex items-center gap-3">
          <ShieldCheck className="size-7 text-twitch" strokeWidth={1.5} aria-hidden="true" />
          <h1 className="text-2xl font-bold text-text">Moderation invite</h1>
        </div>

        {!isHydrated || loading ? (
          <div className="flex justify-center py-12">
            <InfinityLogo size={64} />
          </div>
        ) : !user ? (
          <SignInPrompt />
        ) : error && !preview ? (
          <DeadEnd message={error} />
        ) : preview === null ? (
          <InvitePreviewSkeleton />
        ) : (
          <Card className="p-6">
            <div className="space-y-6">
              <div>
                <p className="text-sm text-text-sub">
                  <span className="font-semibold text-text">{preview.owner_display_name}</span> is
                  asking you to help moderate
                </p>
                <p className="mt-1 text-lg font-semibold text-text">{preview.overlay_name}</p>
                {preview.invitee_label && (
                  <p className="mt-1 text-xs text-text-dim">
                    They addressed this invite to &ldquo;{preview.invitee_label}&rdquo;.
                  </p>
                )}
              </div>

              <div>
                <h2 className="text-sm font-semibold text-text">What you would be able to do</h2>
                <ul className="mt-2 space-y-1">
                  {actions.map((action) => (
                    <li key={action} className="text-sm text-text-sub">
                      {ACTION_LABELS[action]}
                    </li>
                  ))}
                </ul>
              </div>

              <div>
                <h2 className="text-sm font-semibold text-text">On these platforms</h2>
                {platforms.length > 0 ? (
                  <div className="mt-2 flex flex-wrap gap-1.5">
                    {platforms.map((leg) => (
                      <PlatformBadge key={leg.platform} platform={leg.platform} size="sm" />
                    ))}
                  </div>
                ) : (
                  <p className="mt-2 text-sm text-text-sub">
                    None yet — {preview.owner_display_name} still has to turn a platform on.
                  </p>
                )}
              </div>

              {/* The one thing a volunteer most needs to know before agreeing: this does
                  not touch their own channel, and it does not ask for anything yet. */}
              <p className="flex items-start gap-2 rounded-lg border border-border bg-surface-2 px-4 py-3 text-xs text-text-sub">
                <Info className="mt-0.5 size-3.5 shrink-0 text-text-dim" aria-hidden="true" />
                <span>
                  You will act with your own platform account, so each platform still checks that{' '}
                  {preview.owner_display_name} made you a moderator there. Nothing is asked of you
                  now — you connect a platform the first time you moderate on it.
                </span>
              </p>

              {preview.expected_account && (
                <p className="text-xs text-text-dim">
                  This invite is meant for {preview.expected_platform ?? ''}{' '}
                  {preview.expected_account}.
                </p>
              )}

              {error && (
                <p role="alert" className="text-destructive text-sm">
                  {error}
                </p>
              )}

              <div className="flex flex-wrap gap-3">
                <Button variant="gradient" onClick={() => void handleAccept()} disabled={accepting}>
                  {accepting ? 'Accepting…' : 'Accept and start moderating'}
                </Button>
                <Link
                  href="/dashboard"
                  className="inline-flex items-center rounded-lg border border-border px-4 py-2 text-sm font-medium text-text-sub transition-colors hover:border-border-md hover:text-text focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
                >
                  Not now
                </Link>
              </div>
            </div>
          </Card>
        )}
      </main>
    </div>
  )
}

export default function AcceptInvitePage() {
  return (
    // useSearchParams (the invite secret) opts this subtree out of static rendering, so it
    // needs its own boundary.
    <Suspense
      fallback={
        <div className="min-h-screen bg-bg">
          <AppNav />
          <main className="mx-auto max-w-2xl px-4 py-12 sm:px-6 lg:px-8">
            <InvitePreviewSkeleton />
          </main>
        </div>
      }
    >
      <AcceptContent />
    </Suspense>
  )
}
