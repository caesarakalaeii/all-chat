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
import { useTranslations, type TFunction } from '@/lib/i18n'
import { interpolateElements } from '@/lib/i18n/emphasise'
import { boundInviteAccount, delegationErrorCode, moderationApi } from '@/lib/api/moderation'
import {
  DELEGATABLE_ACTIONS,
  type InvitePreview,
  type ModerationAction,
} from '@/lib/types/moderation'

/**
 * Catalog key stem per action. `satisfies` rather than an annotation keeps the
 * stems literal, so a typo fails tsc at the t() call.
 */
const ACTION_MESSAGE_STEMS = {
  delete: 'actionDelete',
  timeout: 'actionTimeout',
  ban: 'actionBan',
  unban: 'actionUnban',
} as const satisfies Record<ModerationAction, string>

/**
 * Human copy for a failed preview or accept, keyed on the machine-readable `code` rather
 * than the message text — the copy differs by role server-side and is free to change.
 *
 * Every branch ends somewhere the reader can act. "Not found" covers unknown, already
 * redeemed and revoked alike, which the server keeps deliberately indistinguishable, so
 * the copy names all three rather than guessing one.
 */
function inviteErrorMessage(t: TFunction, err: unknown): string {
  switch (delegationErrorCode(err)) {
    case 'invite_not_found':
      return t('moderation.accept.errorNotFound')
    case 'invite_expired':
      return t('moderation.accept.errorExpired')
    case 'already_moderator':
      return t('moderation.accept.errorAlreadyModerator')
    case 'owner_cannot_accept':
      return t('moderation.accept.errorOwnerCannotAccept')
    case 'invite_bound_to_other_account': {
      // Four whole sentences rather than one with optional clauses appended:
      // the platform and the account are each optional server-side, and a
      // second language will not put them where English does.
      const bound = boundInviteAccount(err)
      if (bound?.platform && bound.account)
        return t('moderation.accept.errorBoundToOtherBoth', {
          platform: bound.platform,
          account: bound.account,
        })
      if (bound?.platform)
        return t('moderation.accept.errorBoundToOtherPlatform', { platform: bound.platform })
      if (bound?.account)
        return t('moderation.accept.errorBoundToOtherAccount', { account: bound.account })
      return t('moderation.accept.errorBoundToOther')
    }
    default:
      return t('moderation.accept.errorUnknown')
  }
}

function InvitePreviewSkeleton() {
  const t = useTranslations()
  return (
    <div role="status" className="space-y-4 rounded-xl border border-border bg-surface p-6">
      <VisuallyHidden>{t('moderation.accept.loadingInvite')}</VisuallyHidden>
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
  const t = useTranslations()
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
            {t('moderation.accept.goToChannels')}
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
  const t = useTranslations()
  return (
    <Card className="p-6">
      <div className="flex items-start gap-3">
        <Info className="mt-0.5 size-5 shrink-0 text-text-dim" aria-hidden="true" />
        <div className="space-y-4">
          <div className="space-y-1">
            <p className="text-sm font-semibold text-text">
              {t('moderation.accept.signInHeading')}
            </p>
            <p className="text-sm text-text-sub">{t('moderation.accept.signInBody')}</p>
          </div>
          <Link
            href="/"
            className="inline-flex w-fit items-center rounded-lg border border-border px-3 py-1.5 text-sm font-medium text-text-sub transition-colors hover:border-border-md hover:text-text focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
          >
            {t('moderation.accept.signIn')}
          </Link>
        </div>
      </div>
    </Card>
  )
}

function AcceptContent() {
  const t = useTranslations()
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
      setError(t('moderation.accept.errorMissingToken'))
      return
    }
    let cancelled = false
    loadPreview()
      .then((p) => {
        if (!cancelled) setPreview(p)
      })
      .catch((err) => {
        if (!cancelled) setError(inviteErrorMessage(t, err))
      })
    return () => {
      cancelled = true
    }
  }, [user, token, loadPreview, t])

  const handleAccept = async () => {
    setAccepting(true)
    setError(null)
    try {
      // The overlay id exists only from here on — the preview withholds it.
      const accepted = await moderationApi.acceptInvite(token)
      router.push(`/overlay/${accepted.overlay_id}/view`)
    } catch (err) {
      setError(inviteErrorMessage(t, err))
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
          <h1 className="text-2xl font-bold text-text">{t('moderation.accept.heading')}</h1>
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
                  {interpolateElements(t('moderation.accept.askingToHelp'), {
                    owner: (
                      <span className="font-semibold text-text">{preview.owner_display_name}</span>
                    ),
                  })}
                </p>
                <p className="mt-1 text-lg font-semibold text-text">{preview.overlay_name}</p>
                {preview.invitee_label && (
                  <p className="mt-1 text-xs text-text-dim">
                    {t('moderation.accept.addressedTo', { label: preview.invitee_label })}
                  </p>
                )}
              </div>

              <div>
                <h2 className="text-sm font-semibold text-text">
                  {t('moderation.accept.actionsHeading')}
                </h2>
                <ul className="mt-2 space-y-1">
                  {actions.map((action) => (
                    <li key={action} className="text-sm text-text-sub">
                      {t(`moderation.accept.${ACTION_MESSAGE_STEMS[action]}`)}
                    </li>
                  ))}
                </ul>
              </div>

              <div>
                <h2 className="text-sm font-semibold text-text">
                  {t('moderation.accept.platformsHeading')}
                </h2>
                {platforms.length > 0 ? (
                  <div className="mt-2 flex flex-wrap gap-1.5">
                    {platforms.map((leg) => (
                      <PlatformBadge key={leg.platform} platform={leg.platform} size="sm" />
                    ))}
                  </div>
                ) : (
                  <p className="mt-2 text-sm text-text-sub">
                    {t('moderation.accept.noPlatforms', { owner: preview.owner_display_name })}
                  </p>
                )}
              </div>

              {/* The one thing a volunteer most needs to know before agreeing: this does
                  not touch their own channel, and it does not ask for anything yet. */}
              <p className="flex items-start gap-2 rounded-lg border border-border bg-surface-2 px-4 py-3 text-xs text-text-sub">
                <Info className="mt-0.5 size-3.5 shrink-0 text-text-dim" aria-hidden="true" />
                <span>
                  {t('moderation.accept.ownAccountNote', { owner: preview.owner_display_name })}
                </span>
              </p>

              {preview.expected_account && (
                <p className="text-xs text-text-dim">
                  {preview.expected_platform
                    ? t('moderation.accept.expectedAccountOnPlatform', {
                        platform: preview.expected_platform,
                        account: preview.expected_account,
                      })
                    : t('moderation.accept.expectedAccount', {
                        account: preview.expected_account,
                      })}
                </p>
              )}

              {error && (
                <p role="alert" className="text-sm text-destructive">
                  {error}
                </p>
              )}

              <div className="flex flex-wrap gap-3">
                <Button variant="gradient" onClick={() => void handleAccept()} disabled={accepting}>
                  {accepting ? t('moderation.accept.accepting') : t('moderation.accept.accept')}
                </Button>
                <Link
                  href="/dashboard"
                  className="inline-flex items-center rounded-lg border border-border px-4 py-2 text-sm font-medium text-text-sub transition-colors hover:border-border-md hover:text-text focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
                >
                  {t('moderation.accept.notNow')}
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
