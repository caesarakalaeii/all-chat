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
 * "Channels you moderate" (ADR-0048).
 *
 * This page is not a convenience. `GET /api/v1/overlays` is owner-filtered and there is
 * no shared-with-me listing, so without it an accepted delegation is unreachable — the
 * moderator would hold a grant with no way to open the overlay it applies to.
 *
 * Two things it deliberately does NOT do:
 *
 * - It never links a moderator to `/upgrade`. Entitlement is the STREAMER's (the gate is
 *   keyed on the overlay owner), so a lapsed plan is reported as the streamer's plan,
 *   with no call to action a volunteer could act on.
 * - It does not claim per-platform readiness. Whether the moderator has connected their
 *   own account for a platform is answered per source by the capabilities endpoint when
 *   they open the monitor, which is also where the "Connect to moderate" action lives.
 *   Repeating a guess here would be a second, staler source of truth.
 */

import { Suspense, useCallback, useEffect, useState } from 'react'
import Link from 'next/link'
import { useSearchParams } from 'next/navigation'
import { Info, MonitorPlay, ShieldCheck } from 'lucide-react'

import { AppNav } from '@/components/AppNav'
import { ProtectedRoute } from '@/components/ProtectedRoute'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { PlatformBadge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import { VisuallyHidden } from '@/components/ui/visually-hidden'
import { getDiscordIdentity, startDiscordAccountLink } from '@/lib/api/discord'
import { moderationApi } from '@/lib/api/moderation'
import { DELEGATABLE_ACTIONS, DELEGATABLE_PLATFORMS, type Delegation } from '@/lib/types/moderation'
import { useTranslations, type TFunction } from '@/lib/i18n'

/**
 * Copy for the mod-consent redirect's query string. The OAuth flow returns here rather
 * than to the overlay because one consent covers every streamer who delegated that
 * platform, so there is no single overlay it belongs to.
 */
function consentNotice(
  t: TFunction,
  connected: string | null,
  error: string | null,
  discordAccount: string | null
): string | null {
  // `already_linked` is the one failure worth its own words: that Discord account backs a
  // different All-Chat account, and no amount of retrying changes it. One Discord identity may
  // back at most one account, or a second could inherit the first's server permissions.
  if (error === 'already_linked') {
    return t('moderation.channels.noticeDiscordAlreadyLinked')
  }
  if (error) {
    return t('moderation.channels.noticeConnectFailed')
  }
  if (discordAccount === 'linked') {
    return t('moderation.channels.noticeDiscordLinked')
  }
  if (connected) {
    // `connected` is a query-string value, so it may name a platform the
    // catalog has no display name for. Falling back to the raw value keeps the
    // notice readable rather than printing a catalog key.
    const known = DELEGATABLE_PLATFORMS.find((p) => p === connected)
    const name = known ? t(`common.platforms.${known}`) : connected
    return t('moderation.channels.noticeConnected', { platform: name })
  }
  return null
}

function DelegationSkeleton() {
  const t = useTranslations()
  return (
    <div role="status" className="grid grid-cols-1 gap-6 md:grid-cols-2">
      <VisuallyHidden>{t('moderation.channels.loading')}</VisuallyHidden>
      {Array.from({ length: 2 }).map((_, i) => (
        <div key={i} className="space-y-3 rounded-xl border border-border bg-surface p-6">
          <Skeleton className="h-4 w-1/2" />
          <Skeleton className="h-3 w-1/3" />
          <div className="flex gap-1.5">
            <Skeleton className="h-4 w-16 rounded-full" />
            <Skeleton className="h-4 w-16 rounded-full" />
          </div>
        </div>
      ))}
    </div>
  )
}

function EmptyState() {
  const t = useTranslations()
  return (
    <div className="flex flex-col items-center gap-4 py-24 text-center">
      <ShieldCheck className="size-16 text-text-dim" strokeWidth={1} aria-hidden="true" />
      <h2 className="text-xl font-semibold text-text">{t('moderation.channels.emptyHeading')}</h2>
      <p className="max-w-md text-sm text-text-sub">{t('moderation.channels.emptyBody')}</p>
    </div>
  )
}

function DelegationCard({ delegation }: { delegation: Delegation }) {
  const t = useTranslations()
  // Rendered in the backend's canonical order rather than the array's, so two grants with
  // the same permissions always read identically.
  const actions = DELEGATABLE_ACTIONS.filter((a) => delegation.actions.includes(a))
  const platforms = delegation.platforms.filter((leg) => leg.enabled)
  const suspended = delegation.status === 'suspended'

  return (
    <Card className="flex flex-col overflow-hidden">
      <div className="flex flex-1 flex-col gap-4 p-6">
        <div className="min-w-0">
          <h2 className="truncate font-semibold text-text">{delegation.overlay_name}</h2>
          <p className="truncate text-sm text-text-sub">
            {t('moderation.channels.forOwner', { owner: delegation.owner_display_name })}
          </p>
        </div>

        {platforms.length > 0 ? (
          <div className="flex flex-wrap gap-1.5">
            {platforms.map((leg) => (
              <PlatformBadge key={leg.platform} platform={leg.platform} size="sm" />
            ))}
          </div>
        ) : (
          <p className="text-xs text-text-dim">
            {t('moderation.channels.noPlatforms', { owner: delegation.owner_display_name })}
          </p>
        )}

        <ul className="flex flex-wrap gap-1.5">
          {actions.map((action) => (
            <li
              key={action}
              className="rounded-full border border-border bg-surface-2 px-2 py-0.5 text-[0.65rem] text-text-sub"
            >
              {t(`moderation.actions.${action}`)}
            </li>
          ))}
        </ul>

        {suspended && (
          <p className="flex items-start gap-2 text-xs text-text-sub">
            <Info className="mt-0.5 size-3.5 shrink-0 text-text-dim" aria-hidden="true" />
            <span>
              {t('moderation.channels.suspendedNote', { owner: delegation.owner_display_name })}
            </span>
          </p>
        )}

        {/* The streamer's plan, never the moderator's: there is nothing here a volunteer
            could buy, so this states the cause and stops. */}
        {!delegation.available && (
          <p className="flex items-start gap-2 text-xs text-text-sub">
            <Info className="mt-0.5 size-3.5 shrink-0 text-text-dim" aria-hidden="true" />
            <span>
              {t('moderation.channels.unavailableNote', { owner: delegation.owner_display_name })}
            </span>
          </p>
        )}

        {/* Always reachable, even suspended or unavailable: reading the chat is not the
            capability in question, and a dead card would leave the moderator nowhere. */}
        <Link
          href={`/overlay/${delegation.overlay_id}/view`}
          className="mt-auto inline-flex w-fit items-center gap-2 rounded-lg border border-border px-3 py-1.5 text-sm font-medium text-text-sub transition-colors hover:border-border-md hover:text-text focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
        >
          <MonitorPlay className="size-4" aria-hidden="true" />
          {t('moderation.channels.openMonitor')}
        </Link>
      </div>
    </Card>
  )
}

function ModerateContent() {
  const t = useTranslations()
  const params = useSearchParams()
  const notice = consentNotice(
    t,
    params.get('connected'),
    params.get('error'),
    params.get('discord_account')
  )

  const [delegations, setDelegations] = useState<Delegation[] | null>(null)
  const [loadFailed, setLoadFailed] = useState(false)

  const fetchDelegations = useCallback(() => moderationApi.listDelegations(), [])

  // State is set from the promise callback, not the effect body: a synchronous setState
  // in an effect cascades renders, and `cancelled` keeps a slow response from landing
  // after unmount.
  useEffect(() => {
    let cancelled = false
    fetchDelegations()
      .then((list) => {
        if (cancelled) return
        setDelegations(list.delegations)
        setLoadFailed(false)
      })
      .catch(() => {
        if (!cancelled) setLoadFailed(true)
      })
    return () => {
      cancelled = true
    }
  }, [fetchDelegations])

  const reload = useCallback(async () => {
    try {
      setDelegations((await fetchDelegations()).delegations)
      setLoadFailed(false)
    } catch {
      setLoadFailed(true)
    }
  }, [fetchDelegations])

  // Whether this moderator has linked a Discord account. null = not known yet, and the prompt
  // stays hidden until it is: offering "link Discord" to someone already linked is noise.
  const [discordLinked, setDiscordLinked] = useState<boolean | null>(null)
  useEffect(() => {
    let cancelled = false
    getDiscordIdentity()
      .then((identity) => {
        if (!cancelled) setDiscordLinked(identity.linked)
      })
      .catch(() => {
        // Unknown, not unlinked: prompting on a failed read would nag someone who is already set up.
        if (!cancelled) setDiscordLinked(null)
      })
    return () => {
      cancelled = true
    }
  }, [notice])

  // Only worth asking for if some streamer actually delegated Discord to them. Discord is the one
  // platform where the link is the prerequisite rather than an OAuth consent — the shared bot
  // writes, so All-Chat checks the acting human's own server permissions (ADR-0048).
  const hasDiscordLeg = (delegations ?? []).some((d) =>
    d.platforms.some((p) => p.platform === 'discord' && p.enabled)
  )

  return (
    <div className="min-h-screen bg-bg">
      <AppNav />
      <main id="main-content" tabIndex={-1} className="mx-auto max-w-5xl px-4 py-8 sm:px-6 lg:px-8">
        <div className="mb-8">
          <h1 className="text-2xl font-bold text-text">{t('moderation.channels.heading')}</h1>
          <p className="mt-1 text-sm text-text-sub">{t('moderation.channels.subheading')}</p>
        </div>

        {notice && (
          <div className="mb-6 flex items-start gap-2 rounded-lg border border-border bg-surface-2 px-4 py-3 text-sm text-text-sub">
            <Info className="mt-0.5 size-4 shrink-0 text-text-dim" aria-hidden="true" />
            <span>{notice}</span>
          </div>
        )}

        {hasDiscordLeg && discordLinked === false && (
          <div className="mb-6 flex flex-wrap items-start justify-between gap-3 rounded-lg border border-border bg-surface-2 px-4 py-3 text-sm text-text-sub">
            <span className="flex items-start gap-2">
              <Info className="mt-0.5 size-4 shrink-0 text-text-dim" aria-hidden="true" />
              <span>{t('moderation.channels.discordPromptBody')}</span>
            </span>
            <Button size="sm" onClick={() => void startDiscordAccountLink('moderate')}>
              {t('moderation.channels.linkDiscord')}
            </Button>
          </div>
        )}

        {loadFailed ? (
          <div className="space-y-3">
            <p className="text-sm text-text-sub">{t('moderation.channels.loadFailed')}</p>
            <Button variant="outline" size="sm" onClick={() => void reload()}>
              {t('moderation.channels.tryAgain')}
            </Button>
          </div>
        ) : delegations === null ? (
          <DelegationSkeleton />
        ) : delegations.length === 0 ? (
          <EmptyState />
        ) : (
          <div className="grid grid-cols-1 gap-6 md:grid-cols-2">
            {delegations.map((d) => (
              <DelegationCard key={d.grant_id} delegation={d} />
            ))}
          </div>
        )}
      </main>
    </div>
  )
}

export default function ModeratePage() {
  return (
    <ProtectedRoute>
      {/* useSearchParams (the mod-consent redirect's ?connected= / ?error=) opts this
          subtree out of static rendering, so it needs its own boundary. */}
      <Suspense
        fallback={
          <div className="min-h-screen bg-bg">
            <AppNav />
            <main className="mx-auto max-w-5xl px-4 py-8 sm:px-6 lg:px-8">
              <DelegationSkeleton />
            </main>
          </div>
        }
      >
        <ModerateContent />
      </Suspense>
    </ProtectedRoute>
  )
}
