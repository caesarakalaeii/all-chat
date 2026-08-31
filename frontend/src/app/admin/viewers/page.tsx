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
 * Admin Viewer Management Page
 *
 * Find a viewer (server-side search + filters + pagination), inspect their
 * cross-streamer activity, and ban/unban or grant/revoke premium. Search,
 * filters, and the total count are resolved server-side (ADR-0034) so they are
 * correct across the whole dataset, not just the loaded page.
 *
 * Route: /admin/viewers
 */

'use client'

import { useEffect, useId, useState } from 'react'
import Link from 'next/link'
import { useRouter } from 'next/navigation'
import clsx from 'clsx'
import { useAuthStore } from '@/lib/stores/auth-store'
import { apiClient } from '@/lib/api/client'
import { formatDistanceToNow } from 'date-fns'
import { Card } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { Dialog } from '@/components/ui/dialog'
import { toastManager } from '@/lib/toast'
import { PremiumDurationChooser } from '@/components/admin/PremiumDurationChooser'
import { UserAvatar } from '@/components/UserAvatar'
import { PlatformBadge } from '@/components/ui/badge'
import { ChannelLink } from '@/components/ChannelLink'
import { useTranslations } from '@/lib/i18n'

interface ViewerSession {
  id: string
  platform: string
  platform_user_id: string
  username: string
  display_name: string
  avatar_url?: string | null
  last_message_at: string | null
  message_count_1min: number
  message_count_1hour: number
  is_premium: boolean
  premium_expires_at?: string | null
  viewer_id: string | null
  // Linked streamer account (viewer who is also a streamer), when present.
  user_id?: string | null
  is_banned: boolean
  banned_at: string | null
  banned_reason: string | null
  created_at: string
}

interface ViewerListResponse {
  viewers: ViewerSession[]
  total: number
  limit: number
  offset: number
}

interface ViewerActivityStreamer {
  streamer_user_id: string
  streamer_username: string
  overlay_id?: string | null
  channel_name: string
  platform: string
  message_count: number
  last_sent_at: string
}

interface ViewerActivity {
  total_messages: number
  last_sent_at: string | null
  streamers: ViewerActivityStreamer[]
}

const PAGE_SIZE = 50

// The reason recorded when an admin bans without typing one. A request-body
// value, not copy: admin_viewers.go:141 applies the identical default
// server-side, and translating one side would silently split the stored value.
const DEFAULT_BAN_REASON = 'No reason provided'

// The dash shown where a session-only viewer has no linked account. A
// typographic symbol standing in for absent data, not copy; the title attribute
// beside it says the same thing in words.
const NO_ACCOUNT_DASH = '\u2014'

export default function AdminViewersPage() {
  const router = useRouter()
  const { user } = useAuthStore()
  const t = useTranslations()

  const [viewers, setViewers] = useState<ViewerSession[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(true)
  // Bumped by mutations (ban/premium) to force a refetch without threading an
  // external fetch function through the effect.
  const [refreshKey, setRefreshKey] = useState(0)

  // Discovery controls (server-side). `searchInput` is the immediate field
  // value; `search` is the debounced value actually sent to the API. Both are
  // seeded from ?q= (e.g. arriving from global search) via a lazy initializer
  // so there's no synchronous setState in an effect.
  const [initialQuery] = useState(() =>
    typeof window === 'undefined'
      ? ''
      : (new URLSearchParams(window.location.search).get('q') ?? '')
  )
  const [searchInput, setSearchInput] = useState(initialQuery)
  const [search, setSearch] = useState(initialQuery)
  const [statusFilter, setStatusFilter] = useState<'all' | 'banned' | 'active'>('all')
  const [premiumFilter, setPremiumFilter] = useState<'all' | 'premium' | 'free'>('all')
  const [platformFilter, setPlatformFilter] = useState('all')
  const [offset, setOffset] = useState(0)

  const [banningId, setBanningId] = useState<string | null>(null)
  const [banReason, setBanReason] = useState('')
  const [showBanModal, setShowBanModal] = useState(false)
  const [selectedViewer, setSelectedViewer] = useState<ViewerSession | null>(null)
  const [unbanDialogViewer, setUnbanDialogViewer] = useState<ViewerSession | null>(null)
  const [premiumDialogViewer, setPremiumDialogViewer] = useState<ViewerSession | null>(null)
  const [premiumLoading, setPremiumLoading] = useState(false)
  // Time-limited grant selection (ADR-0027). null seconds = permanent; valid=false
  // means the custom day count is empty/out of range and the grant is blocked.
  const [grantDurationSeconds, setGrantDurationSeconds] = useState<number | null>(null)
  const [grantDurationValid, setGrantDurationValid] = useState(true)
  // Activity (streamer context) dialog.
  const [activityViewer, setActivityViewer] = useState<ViewerSession | null>(null)
  const [activity, setActivity] = useState<ViewerActivity | null>(null)
  const [activityLoading, setActivityLoading] = useState(false)
  const banReasonId = useId()
  const searchId = useId()
  const statusId = useId()
  const premiumId = useId()
  const platformId = useId()

  useEffect(() => {
    if (!user?.is_admin) {
      router.push('/dashboard')
    }
  }, [user, router])

  // Debounce the search box so we don't hit the API on every keystroke.
  useEffect(() => {
    const debounce = setTimeout(() => {
      setSearch(searchInput)
      setOffset(0)
    }, 300)
    return () => clearTimeout(debounce)
  }, [searchInput])

  // Fetch is defined inline in the effect (state is set only after the await, so
  // it never runs synchronously in the effect body) and re-runs whenever the
  // filters, page, or refreshKey change. Initial loading=true shows the skeleton
  // on first load; refetches swap the list in place.
  useEffect(() => {
    if (!user?.is_admin) return
    let cancelled = false

    async function run() {
      try {
        const params = new URLSearchParams()
        params.set('limit', String(PAGE_SIZE))
        params.set('offset', String(offset))
        if (search.trim()) params.set('q', search.trim())
        if (statusFilter !== 'all') params.set('is_banned', String(statusFilter === 'banned'))
        if (premiumFilter !== 'all') params.set('is_premium', String(premiumFilter === 'premium'))
        if (platformFilter !== 'all') params.set('platform', platformFilter)

        const response = await apiClient.get<ViewerListResponse>(
          `/api/v1/admin/viewers?${params.toString()}`
        )
        if (cancelled) return
        setViewers(response.viewers ?? [])
        setTotal(response.total ?? 0)
      } catch (error) {
        if (cancelled) return
        console.error('Failed to fetch viewers:', error)
        toastManager.add({ title: t('admin.viewers.loadError'), type: 'error' })
      } finally {
        if (!cancelled) setLoading(false)
      }
    }

    run()
    return () => {
      cancelled = true
    }
  }, [user, offset, search, statusFilter, premiumFilter, platformFilter, refreshKey, t])

  const refetchViewers = () => setRefreshKey((k) => k + 1)

  const openActivity = async (viewer: ViewerSession) => {
    setActivityViewer(viewer)
    setActivity(null)
    setActivityLoading(true)
    try {
      const data = await apiClient.get<ViewerActivity>(
        `/api/v1/admin/viewers/${viewer.id}/activity`
      )
      setActivity(data)
    } catch (error) {
      console.error('Failed to fetch viewer activity:', error)
      toastManager.add({ title: t('admin.viewers.activityLoadError'), type: 'error' })
    } finally {
      setActivityLoading(false)
    }
  }

  const handleBanClick = (viewer: ViewerSession) => {
    setSelectedViewer(viewer)
    setBanReason('')
    setShowBanModal(true)
  }

  const handleBan = async () => {
    if (!selectedViewer) return

    try {
      setBanningId(selectedViewer.id)
      await apiClient.post(`/api/v1/admin/viewers/${selectedViewer.id}/ban`, {
        reason: banReason || DEFAULT_BAN_REASON,
      })
      toastManager.add({
        title: t('admin.viewers.banSuccess', { username: selectedViewer.username }),
        type: 'success',
      })
      setShowBanModal(false)
      setSelectedViewer(null)
      setBanReason('')
      refetchViewers()
    } catch (error) {
      console.error('Failed to ban viewer:', error)
      toastManager.add({ title: t('admin.viewers.banError'), type: 'error' })
    } finally {
      setBanningId(null)
    }
  }

  const handleUnban = async (viewerId: string, username: string) => {
    try {
      setBanningId(viewerId)
      await apiClient.post(`/api/v1/admin/viewers/${viewerId}/unban`, {})
      toastManager.add({ title: t('admin.viewers.unbanSuccess', { username }), type: 'success' })
      setUnbanDialogViewer(null)
      refetchViewers()
    } catch (error) {
      console.error('Failed to unban viewer:', error)
      toastManager.add({ title: t('admin.viewers.unbanError'), type: 'error' })
    } finally {
      setBanningId(null)
    }
  }

  // durationSeconds (grant only) makes the grant time-limited (ADR-0027); null/
  // undefined grants permanently. Ignored when revoking.
  const handleTogglePremium = async (viewer: ViewerSession, durationSeconds?: number | null) => {
    try {
      setPremiumLoading(true)
      const granting = !viewer.is_premium
      const body: { is_premium: boolean; duration_seconds?: number } = { is_premium: granting }
      if (granting && durationSeconds != null) {
        body.duration_seconds = durationSeconds
      }
      await apiClient.post(`/api/v1/admin/viewers/${viewer.id}/premium`, body)
      toastManager.add({
        title: viewer.is_premium
          ? t('admin.viewers.premiumRevoked', { username: viewer.username })
          : t('admin.viewers.premiumGranted', { username: viewer.username }),
        type: 'success',
      })
      setPremiumDialogViewer(null)
      refetchViewers()
    } catch (error) {
      console.error('Failed to update viewer premium:', error)
      toastManager.add({ title: t('admin.viewers.premiumError'), type: 'error' })
    } finally {
      setPremiumLoading(false)
    }
  }

  const pageStart = total === 0 ? 0 : offset + 1
  const pageEnd = Math.min(offset + viewers.length, total)
  const hasPrev = offset > 0
  const hasNext = offset + PAGE_SIZE < total

  // Interactive controls shared between the desktop table and the mobile card list.
  // These render only triggers; the dialogs themselves are hosted once at page level
  // (below) so they aren't duplicated across the table/card breakpoints.
  function renderPremiumControl(viewer: ViewerSession) {
    if (!viewer.viewer_id) {
      return (
        <span className="text-xs text-text-dim" title={t('admin.viewers.sessionOnlyTitle')}>
          {NO_ACCOUNT_DASH}
        </span>
      )
    }
    return (
      <button
        aria-label={
          viewer.is_premium
            ? t('admin.viewers.changePremiumPremiumLabel', { username: viewer.username })
            : t('admin.viewers.changePremiumFreeLabel', { username: viewer.username })
        }
        className={clsx(
          'inline-flex items-center rounded px-2 py-0.5 text-xs font-medium transition-colors',
          viewer.is_premium
            ? 'bg-amber-400/10 text-amber-400 hover:bg-amber-400/20'
            : 'bg-surface-2 text-text-dim hover:bg-surface-2/80'
        )}
        onClick={() => {
          // Reset the duration selection to the default (Permanent) each open.
          setGrantDurationSeconds(null)
          setGrantDurationValid(true)
          setPremiumDialogViewer(viewer)
        }}
      >
        {viewer.is_premium ? t('admin.viewers.premiumBadge') : t('admin.viewers.freeBadge')}
      </button>
    )
  }

  function renderActionControls(viewer: ViewerSession) {
    return (
      <div className="flex items-center gap-2">
        <Button variant="outline" size="sm" onClick={() => openActivity(viewer)}>
          {t('admin.viewers.activityButton')}
        </Button>
        {viewer.is_banned ? (
          <Button
            variant="outline"
            size="sm"
            aria-label={
              banningId === viewer.id
                ? t('admin.viewers.unbanningLabel', { username: viewer.username })
                : t('admin.viewers.unbanLabel', { username: viewer.username })
            }
            disabled={banningId === viewer.id}
            onClick={() => setUnbanDialogViewer(viewer)}
          >
            {banningId === viewer.id
              ? t('admin.viewers.unbanningButton')
              : t('admin.viewers.unbanButton')}
          </Button>
        ) : (
          <Button
            variant="destructive"
            size="sm"
            aria-label={t('admin.viewers.banLabel', { username: viewer.username })}
            disabled={banningId === viewer.id}
            onClick={() => handleBanClick(viewer)}
          >
            {t('admin.viewers.banButton')}
          </Button>
        )}
      </div>
    )
  }

  function renderIdentity(viewer: ViewerSession) {
    return (
      <div className="flex min-w-0 items-center gap-3">
        <UserAvatar
          avatarUrl={viewer.avatar_url ?? undefined}
          displayName={viewer.display_name || viewer.username}
          size={32}
        />
        <div className="min-w-0">
          <ChannelLink
            platform={viewer.platform}
            channelId={viewer.username}
            channelName={viewer.display_name || viewer.username}
            className="truncate text-sm font-medium text-text"
          />
          <div className="truncate text-xs text-text-sub">@{viewer.username}</div>
          <div className="truncate font-mono text-[0.65rem] text-text-dim">
            {viewer.platform_user_id}
          </div>
        </div>
      </div>
    )
  }

  return (
    <div className="mx-auto max-w-7xl px-4 py-8">
      <div className="mb-6 flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-text">{t('admin.viewers.heading')}</h1>
          <p className="mt-1 text-sm text-text-sub">{t('admin.viewers.intro')}</p>
        </div>
        <span className="text-sm text-text-sub">
          {t('admin.viewers.totalMatching', { count: total.toLocaleString() })}
        </span>
      </div>

      {/* Search + filters (server-side) */}
      <Card className="mb-6 p-4">
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <div className="lg:col-span-2">
            <label htmlFor={searchId} className="mb-2 block text-sm font-medium text-text-sub">
              {t('admin.viewers.searchLabel')}
            </label>
            <input
              id={searchId}
              type="text"
              placeholder={t('admin.viewers.searchPlaceholder')}
              value={searchInput}
              onChange={(e) => setSearchInput(e.target.value)}
              className="block w-full rounded-lg border border-border bg-surface-2 px-3 py-2 text-text placeholder:text-text-dim focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none sm:text-sm"
            />
          </div>
          <div>
            <label htmlFor={platformId} className="mb-2 block text-sm font-medium text-text-sub">
              {t('admin.viewers.platformLabel')}
            </label>
            <select
              id={platformId}
              value={platformFilter}
              onChange={(e) => {
                setPlatformFilter(e.target.value)
                setOffset(0)
              }}
              className="block w-full rounded-lg border border-border bg-surface-2 px-3 py-2 text-text focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none sm:text-sm"
            >
              <option value="all">{t('admin.viewers.platformAll')}</option>
              <option value="twitch">{t('common.platforms.twitch')}</option>
              <option value="youtube">{t('common.platforms.youtube')}</option>
              <option value="kick">{t('common.platforms.kick')}</option>
              <option value="tiktok">{t('common.platforms.tiktok')}</option>
            </select>
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label htmlFor={statusId} className="mb-2 block text-sm font-medium text-text-sub">
                {t('admin.viewers.statusLabel')}
              </label>
              <select
                id={statusId}
                value={statusFilter}
                onChange={(e) => {
                  setStatusFilter(e.target.value as typeof statusFilter)
                  setOffset(0)
                }}
                className="block w-full rounded-lg border border-border bg-surface-2 px-3 py-2 text-text focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none sm:text-sm"
              >
                <option value="all">{t('admin.viewers.statusAny')}</option>
                <option value="active">{t('admin.viewers.statusActive')}</option>
                <option value="banned">{t('admin.viewers.statusBanned')}</option>
              </select>
            </div>
            <div>
              <label htmlFor={premiumId} className="mb-2 block text-sm font-medium text-text-sub">
                {t('admin.viewers.premiumLabel')}
              </label>
              <select
                id={premiumId}
                value={premiumFilter}
                onChange={(e) => {
                  setPremiumFilter(e.target.value as typeof premiumFilter)
                  setOffset(0)
                }}
                className="block w-full rounded-lg border border-border bg-surface-2 px-3 py-2 text-text focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none sm:text-sm"
              >
                <option value="all">{t('admin.viewers.premiumAny')}</option>
                <option value="premium">{t('admin.viewers.premiumOnly')}</option>
                <option value="free">{t('admin.viewers.premiumFree')}</option>
              </select>
            </div>
          </div>
        </div>
      </Card>

      {/* Viewers Table (desktop) */}
      {loading ? (
        <Card className="space-y-3 p-6">
          {Array.from({ length: 8 }).map((_, i) => (
            <Skeleton key={i} className="h-10 w-full rounded-lg" />
          ))}
        </Card>
      ) : viewers.length === 0 ? (
        <Card className="p-8 text-center text-sm text-text-dim">{t('admin.viewers.empty')}</Card>
      ) : (
        <>
          <Card className="hidden overflow-hidden md:block">
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <caption className="sr-only">{t('admin.viewers.tableCaption')}</caption>
                <thead className="border-b border-border bg-surface-2">
                  <tr>
                    <th scope="col" className="px-4 py-3 text-left font-medium text-text-sub">
                      {t('admin.viewers.columnViewer')}
                    </th>
                    <th scope="col" className="px-4 py-3 text-left font-medium text-text-sub">
                      {t('admin.viewers.columnPlatform')}
                    </th>
                    <th scope="col" className="px-4 py-3 text-left font-medium text-text-sub">
                      {t('admin.viewers.columnLastMessage')}
                    </th>
                    <th scope="col" className="px-4 py-3 text-left font-medium text-text-sub">
                      {t('admin.viewers.columnPremium')}
                    </th>
                    <th scope="col" className="px-4 py-3 text-left font-medium text-text-sub">
                      {t('admin.viewers.columnStatus')}
                    </th>
                    <th scope="col" className="px-4 py-3 text-left font-medium text-text-sub">
                      {t('admin.viewers.columnActions')}
                    </th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-border">
                  {viewers.map((viewer) => (
                    <tr key={viewer.id} className="transition-colors hover:bg-surface-2">
                      <th scope="row" className="px-4 py-3 text-left font-normal">
                        {renderIdentity(viewer)}
                      </th>
                      <td className="px-4 py-3">
                        <PlatformBadge platform={viewer.platform} size="sm" />
                      </td>
                      <td className="px-4 py-3 text-sm text-text-sub">
                        {viewer.last_message_at
                          ? formatDistanceToNow(new Date(viewer.last_message_at), {
                              addSuffix: true,
                            })
                          : t('admin.viewers.neverMessaged')}
                      </td>
                      <td className="px-4 py-3">{renderPremiumControl(viewer)}</td>
                      <td className="px-4 py-3">
                        {viewer.is_banned ? (
                          <div>
                            <span className="inline-flex items-center rounded bg-destructive/10 px-2 py-0.5 text-xs font-medium text-destructive">
                              {t('admin.viewers.badgeBanned')}
                            </span>
                            {viewer.banned_reason && (
                              <div className="mt-1 text-xs text-text-dim">
                                {t('admin.viewers.banReason', { reason: viewer.banned_reason })}
                              </div>
                            )}
                          </div>
                        ) : (
                          <span className="inline-flex items-center rounded bg-kick/10 px-2 py-0.5 text-xs font-medium text-kick">
                            {t('admin.viewers.badgeActive')}
                          </span>
                        )}
                      </td>
                      <td className="px-4 py-3">{renderActionControls(viewer)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </Card>

          {/* Mobile card list */}
          <div className="space-y-3 md:hidden">
            {viewers.map((viewer) => (
              <Card key={viewer.id} className="p-4">
                <div className="flex items-start justify-between gap-3">
                  {renderIdentity(viewer)}
                  {viewer.is_banned ? (
                    <span className="inline-flex shrink-0 items-center rounded bg-destructive/10 px-2 py-0.5 text-xs font-medium text-destructive">
                      {t('admin.viewers.badgeBanned')}
                    </span>
                  ) : (
                    <span className="inline-flex shrink-0 items-center rounded bg-kick/10 px-2 py-0.5 text-xs font-medium text-kick">
                      {t('admin.viewers.badgeActive')}
                    </span>
                  )}
                </div>
                <div className="mt-3 flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-text-sub">
                  <PlatformBadge platform={viewer.platform} size="sm" />
                  <span>
                    {viewer.last_message_at
                      ? formatDistanceToNow(new Date(viewer.last_message_at), { addSuffix: true })
                      : t('admin.viewers.neverMessaged')}
                  </span>
                </div>
                {viewer.is_banned && viewer.banned_reason && (
                  <div className="mt-2 text-xs text-text-dim">
                    {t('admin.viewers.banReason', { reason: viewer.banned_reason })}
                  </div>
                )}
                <div className="mt-3 flex items-center justify-between gap-3">
                  {renderPremiumControl(viewer)}
                  {renderActionControls(viewer)}
                </div>
              </Card>
            ))}
          </div>

          {/* Pagination */}
          <div className="mt-4 flex items-center justify-between text-sm text-text-sub">
            <span>
              {t('admin.viewers.pageRange', {
                start: pageStart.toLocaleString(),
                end: pageEnd.toLocaleString(),
                total: total.toLocaleString(),
              })}
            </span>
            <div className="flex items-center gap-2">
              <Button
                variant="outline"
                size="sm"
                disabled={!hasPrev}
                onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}
              >
                {t('admin.viewers.previousPage')}
              </Button>
              <Button
                variant="outline"
                size="sm"
                disabled={!hasNext}
                onClick={() => setOffset(offset + PAGE_SIZE)}
              >
                {t('admin.viewers.nextPage')}
              </Button>
            </div>
          </div>
        </>
      )}

      {/* Activity dialog — streamer context for a viewer */}
      <Dialog.Root
        open={!!activityViewer}
        onOpenChange={(open) => {
          if (!open) {
            setActivityViewer(null)
            setActivity(null)
          }
        }}
      >
        {activityViewer && (
          <Dialog.Content>
            <Dialog.Title>
              {t('admin.viewers.activityTitle', { username: activityViewer.username })}
            </Dialog.Title>
            <Dialog.Description>{t('admin.viewers.activityDescription')}</Dialog.Description>
            {activityLoading ? (
              <div className="mt-4 space-y-2">
                <Skeleton className="h-10 w-full rounded-lg" />
                <Skeleton className="h-10 w-full rounded-lg" />
              </div>
            ) : activity ? (
              <div className="mt-4">
                <div className="mb-4 flex gap-6 text-sm">
                  <div>
                    <div className="text-xs text-text-sub">
                      {t('admin.viewers.activityTotalMessages')}
                    </div>
                    <div className="text-lg font-semibold text-text">
                      {activity.total_messages.toLocaleString()}
                    </div>
                  </div>
                  <div>
                    <div className="text-xs text-text-sub">
                      {t('admin.viewers.activityLastMessage')}
                    </div>
                    <div className="text-lg font-semibold text-text">
                      {activity.last_sent_at
                        ? formatDistanceToNow(new Date(activity.last_sent_at), { addSuffix: true })
                        : t('admin.viewers.neverMessaged')}
                    </div>
                  </div>
                </div>
                {activity.streamers.length > 0 ? (
                  <ul className="max-h-72 space-y-2 overflow-y-auto">
                    {activity.streamers.map((s, i) => (
                      <li
                        key={`${s.streamer_user_id}-${s.overlay_id ?? i}`}
                        className="rounded-lg border border-border p-3"
                      >
                        <div className="flex items-center justify-between gap-3">
                          <div className="min-w-0">
                            <Link
                              href={`/admin/users?user=${s.streamer_user_id}`}
                              className="text-sm font-medium text-primary hover:underline"
                            >
                              {s.streamer_username
                                ? `@${s.streamer_username}`
                                : t('admin.viewers.activityStreamerFallback')}
                            </Link>
                            <div className="mt-0.5 flex items-center gap-2 text-xs text-text-sub">
                              <PlatformBadge platform={s.platform} size="sm" />
                              <span className="truncate">{s.channel_name}</span>
                            </div>
                          </div>
                          <div className="shrink-0 text-right">
                            <div className="text-sm font-semibold text-text">
                              {s.message_count.toLocaleString()}
                            </div>
                            {s.overlay_id && (
                              <Link
                                href={`/admin/overlays?overlay=${s.overlay_id}`}
                                className="text-xs text-text-sub hover:underline"
                              >
                                {t('admin.viewers.activityOverlayLink')}
                              </Link>
                            )}
                          </div>
                        </div>
                      </li>
                    ))}
                  </ul>
                ) : (
                  <p className="text-sm text-text-dim italic">{t('admin.viewers.activityEmpty')}</p>
                )}
              </div>
            ) : null}
            <div className="mt-6 flex justify-end">
              <Dialog.Close
                render={<Button variant="outline">{t('admin.viewers.activityClose')}</Button>}
              />
            </div>
          </Dialog.Content>
        )}
      </Dialog.Root>

      {/* Premium toggle confirmation — single instance shared by table + cards */}
      <Dialog.Root
        open={!!premiumDialogViewer}
        onOpenChange={(open) => {
          if (!open) setPremiumDialogViewer(null)
        }}
      >
        {premiumDialogViewer && (
          <Dialog.Content showCloseButton={false}>
            <Dialog.Title>
              {premiumDialogViewer.is_premium
                ? t('admin.viewers.revokePremiumTitle', {
                    username: premiumDialogViewer.username,
                  })
                : t('admin.viewers.grantPremiumTitle', { username: premiumDialogViewer.username })}
            </Dialog.Title>
            <Dialog.Description>
              {premiumDialogViewer.is_premium
                ? t('admin.viewers.revokePremiumBody')
                : t('admin.viewers.grantPremiumBody')}
            </Dialog.Description>
            {premiumDialogViewer.is_premium ? (
              premiumDialogViewer.premium_expires_at && (
                <p className="mt-2 text-xs font-medium text-amber-400/80">
                  {t('admin.viewers.premiumExpires', {
                    timestamp: new Date(premiumDialogViewer.premium_expires_at).toLocaleString(),
                  })}
                </p>
              )
            ) : (
              <PremiumDurationChooser
                disabled={premiumLoading}
                onChange={(seconds, valid) => {
                  setGrantDurationSeconds(seconds)
                  setGrantDurationValid(valid)
                }}
              />
            )}
            <div className="mt-6 flex justify-end gap-3">
              <Dialog.Close
                render={<Button variant="outline">{t('admin.viewers.premiumDialogCancel')}</Button>}
              />
              <Button
                variant="default"
                disabled={
                  premiumLoading || (!premiumDialogViewer.is_premium && !grantDurationValid)
                }
                onClick={() => handleTogglePremium(premiumDialogViewer, grantDurationSeconds)}
              >
                {premiumLoading
                  ? t('admin.viewers.premiumUpdating')
                  : premiumDialogViewer.is_premium
                    ? t('admin.viewers.revokePremiumConfirm')
                    : t('admin.viewers.grantPremiumConfirm')}
              </Button>
            </div>
          </Dialog.Content>
        )}
      </Dialog.Root>

      {/* Unban confirmation — single instance shared by table + cards */}
      <Dialog.Root
        open={!!unbanDialogViewer}
        onOpenChange={(open) => {
          if (!open) setUnbanDialogViewer(null)
        }}
      >
        {unbanDialogViewer && (
          <Dialog.Content showCloseButton={false}>
            <Dialog.Title>
              {t('admin.viewers.unbanTitle', { username: unbanDialogViewer.username })}
            </Dialog.Title>
            <Dialog.Description>{t('admin.viewers.unbanBody')}</Dialog.Description>
            <div className="mt-6 flex justify-end gap-3">
              <Dialog.Close
                render={<Button variant="outline">{t('admin.viewers.unbanCancel')}</Button>}
              />
              <Button
                variant="default"
                disabled={banningId === unbanDialogViewer.id}
                onClick={() => handleUnban(unbanDialogViewer.id, unbanDialogViewer.username)}
              >
                {t('admin.viewers.unbanConfirm')}
              </Button>
            </div>
          </Dialog.Content>
        )}
      </Dialog.Root>

      {/* Ban Modal — Dialog with reason textarea */}
      <Dialog.Root
        open={showBanModal}
        onOpenChange={(open) => {
          if (!open) {
            setShowBanModal(false)
            setSelectedViewer(null)
            setBanReason('')
          }
        }}
      >
        <Dialog.Content showCloseButton={false}>
          <Dialog.Title>
            {t('admin.viewers.banTitle', { username: selectedViewer?.username ?? '' })}
          </Dialog.Title>
          <Dialog.Description>
            {t('admin.viewers.banBody', { username: selectedViewer?.username ?? '' })}
          </Dialog.Description>
          <div className="mt-4">
            <label htmlFor={banReasonId} className="mb-2 block text-sm font-medium text-text-sub">
              {t('admin.viewers.banReasonLabel')}
            </label>
            <textarea
              id={banReasonId}
              value={banReason}
              onChange={(e) => setBanReason(e.target.value)}
              placeholder={t('admin.viewers.banReasonPlaceholder')}
              className="w-full resize-none rounded-lg border border-border bg-surface-2 px-3 py-2 text-text placeholder:text-text-dim focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
              rows={3}
            />
          </div>
          <div className="mt-6 flex justify-end gap-3">
            <Dialog.Close
              render={
                <Button variant="outline" disabled={banningId === selectedViewer?.id}>
                  {t('admin.viewers.banCancel')}
                </Button>
              }
            />
            <Button
              variant="destructive"
              disabled={banningId === selectedViewer?.id}
              onClick={handleBan}
            >
              {banningId === selectedViewer?.id
                ? t('admin.viewers.banning')
                : t('admin.viewers.banConfirm')}
            </Button>
          </div>
        </Dialog.Content>
      </Dialog.Root>
    </div>
  )
}
