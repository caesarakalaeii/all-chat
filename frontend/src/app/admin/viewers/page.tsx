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

export default function AdminViewersPage() {
  const router = useRouter()
  const { user } = useAuthStore()

  const [viewers, setViewers] = useState<ViewerSession[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(true)
  // Bumped by mutations (ban/premium) to force a refetch without threading an
  // external fetch function through the effect.
  const [refreshKey, setRefreshKey] = useState(0)

  // Discovery controls (server-side). `searchInput` is the immediate field
  // value; `search` is the debounced value actually sent to the API.
  const [searchInput, setSearchInput] = useState('')
  const [search, setSearch] = useState('')
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
    const t = setTimeout(() => {
      setSearch(searchInput)
      setOffset(0)
    }, 300)
    return () => clearTimeout(t)
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
        toastManager.add({ title: 'Failed to load viewers', type: 'error' })
      } finally {
        if (!cancelled) setLoading(false)
      }
    }

    run()
    return () => {
      cancelled = true
    }
  }, [user, offset, search, statusFilter, premiumFilter, platformFilter, refreshKey])

  const refetchViewers = () => setRefreshKey((k) => k + 1)

  const openActivity = async (viewer: ViewerSession) => {
    setActivityViewer(viewer)
    setActivity(null)
    setActivityLoading(true)
    try {
      const data = await apiClient.get<ViewerActivity>(`/api/v1/admin/viewers/${viewer.id}/activity`)
      setActivity(data)
    } catch (error) {
      console.error('Failed to fetch viewer activity:', error)
      toastManager.add({ title: 'Failed to load activity', type: 'error' })
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
        reason: banReason || 'No reason provided',
      })
      toastManager.add({ title: `${selectedViewer.username} banned successfully`, type: 'success' })
      setShowBanModal(false)
      setSelectedViewer(null)
      setBanReason('')
      refetchViewers()
    } catch (error) {
      console.error('Failed to ban viewer:', error)
      toastManager.add({ title: 'Failed to ban viewer', type: 'error' })
    } finally {
      setBanningId(null)
    }
  }

  const handleUnban = async (viewerId: string, username: string) => {
    try {
      setBanningId(viewerId)
      await apiClient.post(`/api/v1/admin/viewers/${viewerId}/unban`, {})
      toastManager.add({ title: `${username} unbanned successfully`, type: 'success' })
      setUnbanDialogViewer(null)
      refetchViewers()
    } catch (error) {
      console.error('Failed to unban viewer:', error)
      toastManager.add({ title: 'Failed to unban viewer', type: 'error' })
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
        title: `${viewer.username} premium ${viewer.is_premium ? 'revoked' : 'granted'}`,
        type: 'success',
      })
      setPremiumDialogViewer(null)
      refetchViewers()
    } catch (error) {
      console.error('Failed to update viewer premium:', error)
      toastManager.add({ title: 'Failed to update premium status', type: 'error' })
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
        <span className="text-xs text-text-dim" title="Session-only viewer (no linked account)">
          —
        </span>
      )
    }
    return (
      <button
        aria-label={`${viewer.is_premium ? 'Premium' : 'Free'}: change premium status for ${viewer.username}`}
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
        {viewer.is_premium ? 'Premium' : 'Free'}
      </button>
    )
  }

  function renderActionControls(viewer: ViewerSession) {
    return (
      <div className="flex items-center gap-2">
        <Button variant="outline" size="sm" onClick={() => openActivity(viewer)}>
          Activity
        </Button>
        {viewer.is_banned ? (
          <Button
            variant="outline"
            size="sm"
            aria-label={`${banningId === viewer.id ? 'Unbanning' : 'Unban'} ${viewer.username}`}
            disabled={banningId === viewer.id}
            onClick={() => setUnbanDialogViewer(viewer)}
          >
            {banningId === viewer.id ? 'Unbanning...' : 'Unban'}
          </Button>
        ) : (
          <Button
            variant="destructive"
            size="sm"
            aria-label={`Ban ${viewer.username}`}
            disabled={banningId === viewer.id}
            onClick={() => handleBanClick(viewer)}
          >
            Ban
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
          <h1 className="text-2xl font-bold text-text">Viewer Management</h1>
          <p className="mt-1 text-sm text-text-sub">
            Search viewer sessions, inspect activity, and manage bans and premium
          </p>
        </div>
        <span className="text-sm text-text-sub">{total.toLocaleString()} matching</span>
      </div>

      {/* Search + filters (server-side) */}
      <Card className="mb-6 p-4">
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <div className="lg:col-span-2">
            <label htmlFor={searchId} className="mb-2 block text-sm font-medium text-text-sub">
              Search
            </label>
            <input
              id={searchId}
              type="text"
              placeholder="Username, display name, or platform user ID..."
              value={searchInput}
              onChange={(e) => setSearchInput(e.target.value)}
              className="focus-visible:ring-ring block w-full rounded-lg border border-border bg-surface-2 px-3 py-2 text-text placeholder:text-text-dim focus-visible:ring-2 focus-visible:outline-none sm:text-sm"
            />
          </div>
          <div>
            <label htmlFor={platformId} className="mb-2 block text-sm font-medium text-text-sub">
              Platform
            </label>
            <select
              id={platformId}
              value={platformFilter}
              onChange={(e) => {
                setPlatformFilter(e.target.value)
                setOffset(0)
              }}
              className="focus-visible:ring-ring block w-full rounded-lg border border-border bg-surface-2 px-3 py-2 text-text focus-visible:ring-2 focus-visible:outline-none sm:text-sm"
            >
              <option value="all">All platforms</option>
              <option value="twitch">Twitch</option>
              <option value="youtube">YouTube</option>
              <option value="kick">Kick</option>
              <option value="tiktok">TikTok</option>
            </select>
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label htmlFor={statusId} className="mb-2 block text-sm font-medium text-text-sub">
                Status
              </label>
              <select
                id={statusId}
                value={statusFilter}
                onChange={(e) => {
                  setStatusFilter(e.target.value as typeof statusFilter)
                  setOffset(0)
                }}
                className="focus-visible:ring-ring block w-full rounded-lg border border-border bg-surface-2 px-3 py-2 text-text focus-visible:ring-2 focus-visible:outline-none sm:text-sm"
              >
                <option value="all">Any</option>
                <option value="active">Active</option>
                <option value="banned">Banned</option>
              </select>
            </div>
            <div>
              <label htmlFor={premiumId} className="mb-2 block text-sm font-medium text-text-sub">
                Premium
              </label>
              <select
                id={premiumId}
                value={premiumFilter}
                onChange={(e) => {
                  setPremiumFilter(e.target.value as typeof premiumFilter)
                  setOffset(0)
                }}
                className="focus-visible:ring-ring block w-full rounded-lg border border-border bg-surface-2 px-3 py-2 text-text focus-visible:ring-2 focus-visible:outline-none sm:text-sm"
              >
                <option value="all">Any</option>
                <option value="premium">Premium</option>
                <option value="free">Free</option>
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
        <Card className="p-8 text-center text-sm text-text-dim">
          No viewer sessions match your search or filters.
        </Card>
      ) : (
        <>
          <Card className="hidden overflow-hidden md:block">
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <caption className="sr-only">Viewers</caption>
                <thead className="border-b border-border bg-surface-2">
                  <tr>
                    <th scope="col" className="px-4 py-3 text-left font-medium text-text-sub">
                      Viewer
                    </th>
                    <th scope="col" className="px-4 py-3 text-left font-medium text-text-sub">
                      Platform
                    </th>
                    <th scope="col" className="px-4 py-3 text-left font-medium text-text-sub">
                      Last Message
                    </th>
                    <th scope="col" className="px-4 py-3 text-left font-medium text-text-sub">
                      Premium
                    </th>
                    <th scope="col" className="px-4 py-3 text-left font-medium text-text-sub">
                      Status
                    </th>
                    <th scope="col" className="px-4 py-3 text-left font-medium text-text-sub">
                      Actions
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
                          : 'Never'}
                      </td>
                      <td className="px-4 py-3">{renderPremiumControl(viewer)}</td>
                      <td className="px-4 py-3">
                        {viewer.is_banned ? (
                          <div>
                            <span className="bg-destructive/10 text-destructive inline-flex items-center rounded px-2 py-0.5 text-xs font-medium">
                              BANNED
                            </span>
                            {viewer.banned_reason && (
                              <div className="mt-1 text-xs text-text-dim">
                                Reason: {viewer.banned_reason}
                              </div>
                            )}
                          </div>
                        ) : (
                          <span className="inline-flex items-center rounded bg-kick/10 px-2 py-0.5 text-xs font-medium text-kick">
                            Active
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
                    <span className="bg-destructive/10 text-destructive inline-flex shrink-0 items-center rounded px-2 py-0.5 text-xs font-medium">
                      BANNED
                    </span>
                  ) : (
                    <span className="inline-flex shrink-0 items-center rounded bg-kick/10 px-2 py-0.5 text-xs font-medium text-kick">
                      Active
                    </span>
                  )}
                </div>
                <div className="mt-3 flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-text-sub">
                  <PlatformBadge platform={viewer.platform} size="sm" />
                  <span>
                    {viewer.last_message_at
                      ? formatDistanceToNow(new Date(viewer.last_message_at), { addSuffix: true })
                      : 'Never'}
                  </span>
                </div>
                {viewer.is_banned && viewer.banned_reason && (
                  <div className="mt-2 text-xs text-text-dim">Reason: {viewer.banned_reason}</div>
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
              Showing {pageStart.toLocaleString()}–{pageEnd.toLocaleString()} of{' '}
              {total.toLocaleString()}
            </span>
            <div className="flex items-center gap-2">
              <Button
                variant="outline"
                size="sm"
                disabled={!hasPrev}
                onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}
              >
                Previous
              </Button>
              <Button
                variant="outline"
                size="sm"
                disabled={!hasNext}
                onClick={() => setOffset(offset + PAGE_SIZE)}
              >
                Next
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
            <Dialog.Title>Activity for &ldquo;{activityViewer.username}&rdquo;</Dialog.Title>
            <Dialog.Description>
              Messages this viewer has sent through All-Chat, and whose chats they appear in.
            </Dialog.Description>
            {activityLoading ? (
              <div className="mt-4 space-y-2">
                <Skeleton className="h-10 w-full rounded-lg" />
                <Skeleton className="h-10 w-full rounded-lg" />
              </div>
            ) : activity ? (
              <div className="mt-4">
                <div className="mb-4 flex gap-6 text-sm">
                  <div>
                    <div className="text-xs text-text-sub">Total messages</div>
                    <div className="text-lg font-semibold text-text">
                      {activity.total_messages.toLocaleString()}
                    </div>
                  </div>
                  <div>
                    <div className="text-xs text-text-sub">Last message</div>
                    <div className="text-lg font-semibold text-text">
                      {activity.last_sent_at
                        ? formatDistanceToNow(new Date(activity.last_sent_at), { addSuffix: true })
                        : 'Never'}
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
                              {s.streamer_username ? `@${s.streamer_username}` : 'View streamer'}
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
                                overlay
                              </Link>
                            )}
                          </div>
                        </div>
                      </li>
                    ))}
                  </ul>
                ) : (
                  <p className="text-sm text-text-dim italic">
                    No message activity recorded for this viewer.
                  </p>
                )}
              </div>
            ) : null}
            <div className="mt-6 flex justify-end">
              <Dialog.Close render={<Button variant="outline">Close</Button>} />
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
              {premiumDialogViewer.is_premium ? 'Revoke' : 'Grant'} premium for &ldquo;
              {premiumDialogViewer.username}&rdquo;?
            </Dialog.Title>
            <Dialog.Description>
              {premiumDialogViewer.is_premium
                ? 'They will lose access to gradients, avatar frames, and flairs.'
                : 'They will be able to use gradients, avatar frames, and flairs.'}
            </Dialog.Description>
            {premiumDialogViewer.is_premium ? (
              premiumDialogViewer.premium_expires_at && (
                <p className="mt-2 text-xs font-medium text-amber-400/80">
                  Time-limited &mdash; expires{' '}
                  {new Date(premiumDialogViewer.premium_expires_at).toLocaleString()}
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
              <Dialog.Close render={<Button variant="outline">Cancel</Button>} />
              <Button
                variant="default"
                disabled={
                  premiumLoading || (!premiumDialogViewer.is_premium && !grantDurationValid)
                }
                onClick={() => handleTogglePremium(premiumDialogViewer, grantDurationSeconds)}
              >
                {premiumLoading
                  ? 'Updating...'
                  : premiumDialogViewer.is_premium
                    ? 'Revoke Premium'
                    : 'Grant Premium'}
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
            <Dialog.Title>Unban &ldquo;{unbanDialogViewer.username}&rdquo;?</Dialog.Title>
            <Dialog.Description>
              This will restore their ability to send messages.
            </Dialog.Description>
            <div className="mt-6 flex justify-end gap-3">
              <Dialog.Close render={<Button variant="outline">Cancel</Button>} />
              <Button
                variant="default"
                disabled={banningId === unbanDialogViewer.id}
                onClick={() => handleUnban(unbanDialogViewer.id, unbanDialogViewer.username)}
              >
                Unban Viewer
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
          <Dialog.Title>Ban Viewer &ldquo;{selectedViewer?.username}&rdquo;?</Dialog.Title>
          <Dialog.Description>
            This will prevent {selectedViewer?.username} from sending messages.
          </Dialog.Description>
          <div className="mt-4">
            <label htmlFor={banReasonId} className="mb-2 block text-sm font-medium text-text-sub">
              Reason (optional)
            </label>
            <textarea
              id={banReasonId}
              value={banReason}
              onChange={(e) => setBanReason(e.target.value)}
              placeholder="Enter reason for ban..."
              className="focus-visible:ring-ring w-full resize-none rounded-lg border border-border bg-surface-2 px-3 py-2 text-text placeholder:text-text-dim focus-visible:ring-2 focus-visible:outline-none"
              rows={3}
            />
          </div>
          <div className="mt-6 flex justify-end gap-3">
            <Dialog.Close
              render={
                <Button variant="outline" disabled={banningId === selectedViewer?.id}>
                  Cancel
                </Button>
              }
            />
            <Button
              variant="destructive"
              disabled={banningId === selectedViewer?.id}
              onClick={handleBan}
            >
              {banningId === selectedViewer?.id ? 'Banning...' : 'Ban Viewer'}
            </Button>
          </div>
        </Dialog.Content>
      </Dialog.Root>
    </div>
  )
}
