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

import { useEffect, useId, useState } from 'react'
import { useRouter } from 'next/navigation'
import Link from 'next/link'
import clsx from 'clsx'
import { Card } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { Dialog } from '@/components/ui/dialog'
import { toastManager } from '@/lib/toast'
import { useAuthStore } from '@/lib/stores/auth-store'
import { PremiumDurationChooser } from '@/components/admin/PremiumDurationChooser'
import { UserAvatar } from '@/components/UserAvatar'
import { formatDate, formatTimestamp, useTranslations } from '@/lib/i18n'

interface User {
  id: string
  username: string
  display_name: string
  auth_provider: string
  profile_image_url: string
  created_at: string
  twitch_id?: string
  youtube_id?: string
  kick_id?: string
  is_premium: boolean
  is_beta_tester: boolean
  is_ambassador: boolean
  ambassador_tagline?: string | null
  ambassador_sort_order?: number
  premium_expires_at?: string | null
  is_banned: boolean
  banned_at?: string
  banned_reason?: string
  banned_by?: string
  // Overlay-setup counts (from the admin users endpoint). A user with
  // sources_count === 0 signed up but never configured a working overlay.
  overlays_count: number
  sources_count: number
}

interface UserOverlay {
  id: string
  name: string
  sources_count: number
}

export default function UsersPage() {
  const router = useRouter()
  const { startImpersonation } = useAuthStore()
  const t = useTranslations()
  const [users, setUsers] = useState<User[]>([])
  const [selectedUser, setSelectedUser] = useState<User | null>(null)
  const [userOverlays, setUserOverlays] = useState<UserOverlay[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [impersonating, setImpersonating] = useState(false)
  const [searchTerm, setSearchTerm] = useState('')
  const [filter, setFilter] = useState<
    'all' | 'active' | 'banned' | 'premium' | 'beta' | 'unconfigured'
  >('all')
  const [showBanModal, setShowBanModal] = useState(false)
  const [userToBan, setUserToBan] = useState<User | null>(null)
  const [banReason, setBanReason] = useState('')
  const [banLoading, setBanLoading] = useState(false)
  const [impersonateDialogUser, setImpersonateDialogUser] = useState<User | null>(null)
  const [unbanDialogUser, setUnbanDialogUser] = useState<User | null>(null)
  const [premiumDialogUser, setPremiumDialogUser] = useState<User | null>(null)
  const [premiumLoading, setPremiumLoading] = useState(false)
  // Time-limited grant selection (ADR-0027). null seconds = permanent; valid=false
  // means the custom day count is empty/out of range and the grant is blocked.
  const [grantDurationSeconds, setGrantDurationSeconds] = useState<number | null>(null)
  const [grantDurationValid, setGrantDurationValid] = useState(true)
  const [betaDialogUser, setBetaDialogUser] = useState<User | null>(null)
  const [betaLoading, setBetaLoading] = useState(false)
  // Ambassador role + curated showcase card (ADR-0041).
  const [ambassadorDialogUser, setAmbassadorDialogUser] = useState<User | null>(null)
  const [ambassadorLoading, setAmbassadorLoading] = useState(false)
  const [ambassadorTagline, setAmbassadorTagline] = useState('')
  const [ambassadorSortOrder, setAmbassadorSortOrder] = useState('0')
  // Seed the ambassador showcase inputs from the selected user (ADR-0041) so the
  // admin edits the CURRENT card. Adjust-state-during-render (not an effect) is the
  // React-recommended way to reset editable state when the selected row changes; it
  // only fires when the user id changes, so a save never clobbers in-progress edits.
  const [ambassadorSeedId, setAmbassadorSeedId] = useState<string | null>(null)
  if (selectedUser && selectedUser.id !== ambassadorSeedId) {
    setAmbassadorSeedId(selectedUser.id)
    setAmbassadorTagline(selectedUser.ambassador_tagline ?? '')
    setAmbassadorSortOrder(String(selectedUser.ambassador_sort_order ?? 0))
  }
  const banReasonId = useId()

  // Fetch all users from the database
  useEffect(() => {
    async function fetchUsers() {
      try {
        // Auth is via the httpOnly session cookie (same-origin); the gateway
        // CookieToBearer middleware copies the access cookie to Authorization
        // before backend validation (no JS-readable token).
        const response = await fetch('/api/v1/admin/users', {
          credentials: 'same-origin',
        })

        if (!response.ok) {
          throw new Error(`HTTP ${response.status}: ${response.statusText}`)
        }

        const data = await response.json()
        setUsers(data)
        setLoading(false)

        // Deep-links (read post-await so they're client-only, not a synchronous
        // setState in the effect body). ?user=<id> auto-selects a user (from the
        // Overlays/Sources owner links); ?filter=<tab> pre-selects a filter tab
        // (from the dashboard stat cards).
        const params = new URLSearchParams(window.location.search)
        const targetId = params.get('user')
        if (targetId) {
          const match = (data as User[]).find((u) => u.id === targetId)
          if (match) {
            setSelectedUser(match)
            setSearchTerm(match.username)
          }
        }
        const f = params.get('filter')
        if (
          f === 'active' ||
          f === 'banned' ||
          f === 'premium' ||
          f === 'beta' ||
          f === 'unconfigured'
        ) {
          setFilter(f)
        }
      } catch (err) {
        console.error('Failed to load users:', err)
        setError(t('admin.users.loadError'))
        setLoading(false)
      }
    }

    fetchUsers()
  }, [t])

  // Fetch overlays for selected user
  useEffect(() => {
    async function fetchUserOverlays() {
      if (!selectedUser) {
        setUserOverlays([])
        return
      }

      try {
        const response = await fetch(`/api/v1/admin/user-overlays/${selectedUser.id}`, {
          credentials: 'same-origin',
        })

        if (response.ok) {
          const overlays = await response.json()
          setUserOverlays(overlays)
        }
      } catch (err) {
        console.error('Failed to fetch user overlays:', err)
      }
    }

    fetchUserOverlays()
  }, [selectedUser])

  // Refetch users helper. Returns the fresh list so callers can re-sync a selected
  // user from backend truth (e.g. after a role change that recomputed is_premium).
  const refetchUsers = async (): Promise<User[]> => {
    try {
      const response = await fetch('/api/v1/admin/users', {
        credentials: 'same-origin',
      })
      if (response.ok) {
        const data = (await response.json()) as User[]
        setUsers(data)
        return data
      }
    } catch (err) {
      console.error('Failed to refetch users:', err)
    }
    return []
  }

  // Handle impersonation (called from Dialog confirm). H3 cookie auth: the
  // server sets an impersonated-user access cookie; no token is swapped in JS.
  const handleImpersonate = async (userId: string) => {
    setImpersonating(true)
    try {
      await startImpersonation(userId)
      // Redirect to home page
      router.push('/')
    } catch (err) {
      console.error('Failed to impersonate user:', err)
      toastManager.add({ title: t('admin.users.impersonateError'), type: 'error' })
    } finally {
      setImpersonating(false)
      setImpersonateDialogUser(null)
    }
  }

  // Handle ban user
  const handleBanUser = async (reason: string) => {
    if (!userToBan) return

    setBanLoading(true)
    try {
      const response = await fetch(`/api/v1/admin/users/${userToBan.id}/ban`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        credentials: 'same-origin',
        body: JSON.stringify({ reason }),
      })

      if (!response.ok) {
        const errorData = await response.json()
        throw new Error(errorData.error || t('admin.users.banError'))
      }

      toastManager.add({
        title: t('admin.users.banSuccess', { username: userToBan.username }),
        type: 'success',
      })
      setShowBanModal(false)
      setUserToBan(null)
      setBanReason('')
      await refetchUsers()

      // Clear selected user if it was the banned one
      if (selectedUser?.id === userToBan.id) {
        setSelectedUser(null)
      }
    } catch (err: any) {
      toastManager.add({ title: err.message || t('admin.users.banError'), type: 'error' })
    } finally {
      setBanLoading(false)
    }
  }

  // Handle unban user (called from Dialog confirm)
  const handleUnbanUser = async (userId: string, username: string) => {
    try {
      const response = await fetch(`/api/v1/admin/users/${userId}/unban`, {
        method: 'POST',
        credentials: 'same-origin',
      })

      if (!response.ok) {
        const errorData = await response.json()
        throw new Error(errorData.error || t('admin.users.unbanError'))
      }

      toastManager.add({
        title: t('admin.users.unbanSuccess', { username }),
        type: 'success',
      })
      setUnbanDialogUser(null)
      await refetchUsers()
    } catch (err: any) {
      toastManager.add({ title: err.message || t('admin.users.unbanError'), type: 'error' })
    }
  }

  // Handle premium toggle. durationSeconds (grant only) makes it time-limited
  // (ADR-0027); null/undefined grants permanently. Ignored when revoking.
  const handleSetPremium = async (
    userId: string,
    username: string,
    isPremium: boolean,
    durationSeconds?: number | null
  ) => {
    setPremiumLoading(true)
    try {
      const body: { is_premium: boolean; duration_seconds?: number } = { is_premium: isPremium }
      if (isPremium && durationSeconds != null) {
        body.duration_seconds = durationSeconds
      }
      const response = await fetch(`/api/v1/admin/premium/users/${userId}`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        credentials: 'same-origin',
        body: JSON.stringify(body),
      })

      if (!response.ok) {
        const errorData = await response.json()
        throw new Error(errorData.error || t('admin.users.premiumError'))
      }

      toastManager.add({
        title: isPremium
          ? t('admin.users.premiumGranted', { username })
          : t('admin.users.premiumRemoved', { username }),
        type: 'success',
      })
      setPremiumDialogUser(null)
      await refetchUsers()

      // Update selectedUser in place so the panel reflects the change immediately.
      // The server computes the exact deadline from its own clock; this local
      // estimate (accurate to a few seconds) is refreshed by refetchUsers() on the
      // next selection.
      if (selectedUser?.id === userId) {
        const expiresAt =
          isPremium && durationSeconds != null
            ? new Date(Date.now() + durationSeconds * 1000).toISOString()
            : null
        setSelectedUser((u) =>
          u ? { ...u, is_premium: isPremium, premium_expires_at: expiresAt } : u
        )
      }
    } catch (err: any) {
      toastManager.add({ title: err.message || t('admin.users.premiumError'), type: 'error' })
    } finally {
      setPremiumLoading(false)
    }
  }

  // handleSetBetaTester grants/revokes the beta-tester role (ADR-0020): all premium
  // features plus early-access ones. This is the manual grandfathering mechanism for
  // the pre-monetization premium users — there is no data migration.
  const handleSetBetaTester = async (userId: string, username: string, isBetaTester: boolean) => {
    setBetaLoading(true)
    try {
      const response = await fetch(`/api/v1/admin/beta-tester/users/${userId}`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        credentials: 'same-origin',
        body: JSON.stringify({ is_beta_tester: isBetaTester }),
      })

      if (!response.ok) {
        const errorData = await response.json()
        throw new Error(errorData.error || t('admin.users.betaError'))
      }

      toastManager.add({
        title: isBetaTester
          ? t('admin.users.betaGranted', { username })
          : t('admin.users.betaRemoved', { username }),
        type: 'success',
      })
      setBetaDialogUser(null)
      await refetchUsers()

      // Update selectedUser in place. Beta testers are premium (the backend recompute
      // folds is_beta_tester into is_premium), so reflect that here too.
      if (selectedUser?.id === userId) {
        setSelectedUser((u) =>
          u
            ? { ...u, is_beta_tester: isBetaTester, is_premium: isBetaTester ? true : u.is_premium }
            : u
        )
      }
    } catch (err: any) {
      toastManager.add({
        title: err.message || t('admin.users.betaError'),
        type: 'error',
      })
    } finally {
      setBetaLoading(false)
    }
  }

  // handleSetAmbassador grants/revokes the ambassador role (ADR-0041): all premium
  // + early-access features, plus eligibility for the public homepage showcase. On a
  // grant it also curates the card (tagline + sort_order); the streamer opts into
  // being shown publicly themselves. Sending null for a card field preserves it.
  const handleSetAmbassador = async (
    userId: string,
    username: string,
    isAmbassador: boolean,
    card?: { tagline: string | null; sortOrder: number }
  ) => {
    setAmbassadorLoading(true)
    try {
      const body: { is_ambassador: boolean; tagline?: string | null; sort_order?: number } = {
        is_ambassador: isAmbassador,
      }
      if (isAmbassador && card) {
        // Empty tagline => null (preserve the existing one); a non-empty value sets it.
        body.tagline = card.tagline && card.tagline.trim() !== '' ? card.tagline.trim() : null
        body.sort_order = card.sortOrder
      }
      const response = await fetch(`/api/v1/admin/ambassadors/users/${userId}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'same-origin',
        body: JSON.stringify(body),
      })
      if (!response.ok) {
        const errorData = await response.json()
        throw new Error(errorData.error || t('admin.users.ambassadorError'))
      }

      toastManager.add({
        title: isAmbassador
          ? t('admin.users.ambassadorGranted', { username })
          : t('admin.users.ambassadorRemoved', { username }),
        type: 'success',
      })
      setAmbassadorDialogUser(null)
      const fresh = await refetchUsers()

      // Re-sync the detail panel from backend truth. A role change recomputes
      // is_premium server-side (ambassador is folded in, and a revoke may drop it if
      // no subscription/override remains), so patching locally would go stale —
      // adopt the freshly-listed row instead of guessing.
      if (selectedUser?.id === userId) {
        setSelectedUser(fresh.find((u) => u.id === userId) ?? null)
      }
    } catch (err: any) {
      toastManager.add({
        title: err.message || t('admin.users.ambassadorError'),
        type: 'error',
      })
    } finally {
      setAmbassadorLoading(false)
    }
  }

  // Filter and search users
  const displayUsers = users.filter((u) => {
    // Filter by status
    if (filter === 'banned' && !u.is_banned) return false
    if (filter === 'active' && u.is_banned) return false
    if (filter === 'premium' && !u.is_premium) return false
    if (filter === 'beta' && !u.is_beta_tester) return false
    // "Never set up an overlay": no configured chat source anywhere, which
    // covers both users with zero overlays and users with an empty overlay.
    if (filter === 'unconfigured' && u.sources_count > 0) return false

    // Search filter (id lets an owner deep-link land even before it's typed)
    if (searchTerm) {
      const term = searchTerm.toLowerCase()
      return (
        u.username.toLowerCase().includes(term) ||
        u.display_name.toLowerCase().includes(term) ||
        u.id.toLowerCase().includes(term) ||
        u.twitch_id?.toLowerCase().includes(term) ||
        u.youtube_id?.toLowerCase().includes(term) ||
        u.kick_id?.toLowerCase().includes(term)
      )
    }

    return true
  })

  if (error) {
    return (
      <div className="mx-auto max-w-7xl px-4 py-8">
        <Card className="border-destructive p-4">
          <p className="text-destructive">{error}</p>
        </Card>
      </div>
    )
  }

  const bannedCount = users.filter((u) => u.is_banned).length
  const activeCount = users.filter((u) => !u.is_banned).length
  const premiumCount = users.filter((u) => u.is_premium).length
  const betaCount = users.filter((u) => u.is_beta_tester).length
  const unconfiguredCount = users.filter((u) => u.sources_count === 0).length

  return (
    <div className="mx-auto max-w-7xl px-4 py-8">
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-text">{t('admin.users.heading')}</h1>
        <p className="mt-1 text-sm text-text-sub">{t('admin.users.intro')}</p>
      </div>

      <div className="grid grid-cols-1 gap-6 lg:grid-cols-3">
        {/* Users List */}
        <div className="lg:col-span-2">
          {loading ? (
            <Card className="space-y-3 p-6">
              {Array.from({ length: 6 }).map((_, i) => (
                <Skeleton key={i} className="h-14 w-full rounded-lg" />
              ))}
            </Card>
          ) : (
            <Card className="overflow-hidden">
              <div className="border-b border-border px-4 py-5">
                <h3 className="text-base font-medium text-text">
                  {t('admin.users.listHeading', { count: users.length })}
                </h3>

                {/* Search Input */}
                <div className="mt-4">
                  <input
                    type="text"
                    placeholder={t('admin.users.searchPlaceholder')}
                    value={searchTerm}
                    onChange={(e) => setSearchTerm(e.target.value)}
                    className="w-full rounded-lg border border-border bg-surface-2 px-4 py-2 text-text placeholder:text-text-dim focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
                  />
                </div>

                {/* Filter Tabs */}
                <div className="mt-4 flex space-x-4 border-b border-border">
                  <button
                    onClick={() => setFilter('all')}
                    className={clsx(
                      'border-b-2 px-1 pb-2 text-sm font-medium transition-colors',
                      filter === 'all'
                        ? 'border-primary text-primary'
                        : 'border-transparent text-text-sub hover:border-border hover:text-text'
                    )}
                  >
                    {t('admin.users.tabAll', { count: users.length })}
                  </button>
                  <button
                    onClick={() => setFilter('active')}
                    className={clsx(
                      'border-b-2 px-1 pb-2 text-sm font-medium transition-colors',
                      filter === 'active'
                        ? 'border-primary text-primary'
                        : 'border-transparent text-text-sub hover:border-border hover:text-text'
                    )}
                  >
                    {t('admin.users.tabActive', { count: activeCount })}
                  </button>
                  <button
                    onClick={() => setFilter('banned')}
                    className={clsx(
                      'border-b-2 px-1 pb-2 text-sm font-medium transition-colors',
                      filter === 'banned'
                        ? 'border-primary text-primary'
                        : 'border-transparent text-text-sub hover:border-border hover:text-text'
                    )}
                  >
                    {t('admin.users.tabBanned', { count: bannedCount })}
                  </button>
                  <button
                    onClick={() => setFilter('premium')}
                    className={clsx(
                      'border-b-2 px-1 pb-2 text-sm font-medium transition-colors',
                      filter === 'premium'
                        ? 'border-amber-400 text-amber-400'
                        : 'border-transparent text-text-sub hover:border-border hover:text-text'
                    )}
                  >
                    {t('admin.users.tabPremium', { count: premiumCount })}
                  </button>
                  <button
                    onClick={() => setFilter('beta')}
                    className={clsx(
                      'border-b-2 px-1 pb-2 text-sm font-medium transition-colors',
                      filter === 'beta'
                        ? 'border-violet-400 text-violet-400'
                        : 'border-transparent text-text-sub hover:border-border hover:text-text'
                    )}
                  >
                    {t('admin.users.tabBeta', { count: betaCount })}
                  </button>
                  <button
                    onClick={() => setFilter('unconfigured')}
                    title={t('admin.users.tabNoSetupTitle')}
                    className={clsx(
                      'border-b-2 px-1 pb-2 text-sm font-medium transition-colors',
                      filter === 'unconfigured'
                        ? 'border-amber-400 text-amber-400'
                        : 'border-transparent text-text-sub hover:border-border hover:text-text'
                    )}
                  >
                    {t('admin.users.tabNoSetup', { count: unconfiguredCount })}
                  </button>
                </div>
              </div>
              <ul className="max-h-[70vh] divide-y divide-border overflow-y-auto">
                {displayUsers.map((user) => (
                  <li key={user.id}>
                    <button
                      type="button"
                      onClick={() => setSelectedUser(user)}
                      aria-current={selectedUser?.id === user.id ? 'true' : undefined}
                      className={clsx(
                        'w-full cursor-pointer px-4 py-4 text-left transition-colors hover:bg-surface-2',
                        selectedUser?.id === user.id && 'bg-surface-2'
                      )}
                    >
                      <div className="flex items-center justify-between gap-3">
                        <div className="flex min-w-0 flex-1 items-center gap-3">
                          <UserAvatar
                            avatarUrl={user.profile_image_url}
                            displayName={user.display_name}
                            size={36}
                          />
                          <div className="min-w-0 flex-1">
                            <div className="flex items-center">
                              <p className="text-sm font-medium text-text">{user.display_name}</p>
                              <div className="ml-2 flex space-x-1">
                                {user.is_ambassador && (
                                  <span className="inline-flex items-center rounded border border-sky-500/20 bg-sky-500/10 px-2 py-0.5 text-xs font-medium text-sky-400">
                                    {t('admin.users.badgeAmbassador')}
                                  </span>
                                )}
                                {user.is_beta_tester && !user.is_ambassador && (
                                  <span className="inline-flex items-center rounded border border-violet-500/20 bg-violet-500/10 px-2 py-0.5 text-xs font-medium text-violet-400">
                                    {t('admin.users.badgeBeta')}
                                  </span>
                                )}
                                {user.is_premium && !user.is_beta_tester && !user.is_ambassador && (
                                  <span className="inline-flex items-center rounded border border-amber-500/20 bg-amber-500/10 px-2 py-0.5 text-xs font-medium text-amber-400">
                                    {t('admin.users.badgePremium')}
                                  </span>
                                )}
                                {user.is_banned && (
                                  <span className="inline-flex items-center rounded border border-destructive/20 bg-destructive/10 px-2 py-0.5 text-xs font-medium text-destructive">
                                    {t('admin.users.badgeBanned')}
                                  </span>
                                )}
                                {user.sources_count === 0 && (
                                  <span
                                    className="inline-flex items-center rounded border border-amber-500/20 bg-amber-500/10 px-2 py-0.5 text-xs font-medium text-amber-400"
                                    title={t('admin.users.badgeNoSetupTitle')}
                                  >
                                    {user.overlays_count === 0
                                      ? t('admin.users.badgeNoOverlay')
                                      : t('admin.users.badgeNoSources')}
                                  </span>
                                )}
                                {user.twitch_id && (
                                  <span className="inline-flex items-center rounded bg-badge-bg px-2 py-0.5 text-xs font-medium text-twitch">
                                    {t('common.platforms.twitch')}
                                  </span>
                                )}
                                {user.youtube_id && (
                                  <span className="inline-flex items-center rounded bg-badge-bg px-2 py-0.5 text-xs font-medium text-youtube">
                                    {t('common.platforms.youtube')}
                                  </span>
                                )}
                                {user.kick_id && (
                                  <span className="inline-flex items-center rounded bg-badge-bg px-2 py-0.5 text-xs font-medium text-kick">
                                    {t('common.platforms.kick')}
                                  </span>
                                )}
                              </div>
                            </div>
                            <p className="text-sm text-text-sub">@{user.username}</p>
                            <p className="mt-1 text-xs text-text-dim">
                              {t('admin.users.rowJoined', {
                                date: formatDate(new Date(user.created_at)),
                              })}
                            </p>
                          </div>
                        </div>
                        <div>
                          <svg
                            aria-hidden="true"
                            className="h-5 w-5 text-text-dim"
                            fill="none"
                            stroke="currentColor"
                            viewBox="0 0 24 24"
                          >
                            <path
                              strokeLinecap="round"
                              strokeLinejoin="round"
                              strokeWidth="2"
                              d="M9 5l7 7-7 7"
                            />
                          </svg>
                        </div>
                      </div>
                    </button>
                  </li>
                ))}
                {displayUsers.length === 0 && (
                  <li className="px-4 py-10 text-center text-sm text-text-dim">
                    {t('admin.users.empty')}
                  </li>
                )}
              </ul>
            </Card>
          )}
        </div>

        {/* User Details Panel */}
        <div className="lg:sticky lg:top-8 lg:col-span-1 lg:self-start">
          {selectedUser ? (
            <Card className="overflow-hidden">
              <div className="flex items-center gap-3 border-b border-border px-4 py-5">
                <UserAvatar
                  avatarUrl={selectedUser.profile_image_url}
                  displayName={selectedUser.display_name}
                  size={44}
                />
                <div className="min-w-0">
                  <h3 className="truncate text-base font-medium text-text">
                    {selectedUser.display_name}
                  </h3>
                  <p className="truncate text-sm text-text-sub">@{selectedUser.username}</p>
                </div>
              </div>
              <div className="px-4 py-5">
                <dl className="space-y-4">
                  <div>
                    <dt className="text-sm font-medium text-text-sub">
                      {t('admin.users.detailId')}
                    </dt>
                    <dd className="mt-1 font-mono text-sm break-all text-text">
                      {selectedUser.id}
                    </dd>
                  </div>
                  <div>
                    <dt className="text-sm font-medium text-text-sub">
                      {t('admin.users.detailUsername')}
                    </dt>
                    <dd className="mt-1 text-sm text-text">{selectedUser.username}</dd>
                  </div>
                  <div>
                    <dt className="text-sm font-medium text-text-sub">
                      {t('admin.users.detailDisplayName')}
                    </dt>
                    <dd className="mt-1 text-sm text-text">{selectedUser.display_name}</dd>
                  </div>
                  <div>
                    <dt className="text-sm font-medium text-text-sub">
                      {t('admin.users.detailAuthProvider')}
                    </dt>
                    <dd className="mt-1 text-sm text-text capitalize">
                      {selectedUser.auth_provider}
                    </dd>
                  </div>
                  <div>
                    <dt className="text-sm font-medium text-text-sub">
                      {t('admin.users.detailPlatforms')}
                    </dt>
                    <dd className="mt-2 space-y-1">
                      {selectedUser.twitch_id && (
                        <div className="flex items-center text-sm">
                          <span className="font-medium text-twitch">
                            {t('admin.users.platformIdTwitch')}
                          </span>
                          <span className="ml-2 font-mono text-xs text-text-sub">
                            {selectedUser.twitch_id}
                          </span>
                        </div>
                      )}
                      {selectedUser.youtube_id && (
                        <div className="flex items-center text-sm">
                          <span className="font-medium text-youtube">
                            {t('admin.users.platformIdYouTube')}
                          </span>
                          <span className="ml-2 font-mono text-xs text-text-sub">
                            {selectedUser.youtube_id}
                          </span>
                        </div>
                      )}
                      {selectedUser.kick_id && (
                        <div className="flex items-center text-sm">
                          <span className="font-medium text-kick">
                            {t('admin.users.platformIdKick')}
                          </span>
                          <span className="ml-2 font-mono text-xs text-text-sub">
                            {selectedUser.kick_id}
                          </span>
                        </div>
                      )}
                    </dd>
                  </div>
                </dl>

                {/* Impersonate — Dialog confirmation */}
                <div className="mt-6 border-t border-border pt-6">
                  <Dialog.Root
                    open={impersonateDialogUser?.id === selectedUser.id}
                    onOpenChange={(open) => {
                      if (!open) setImpersonateDialogUser(null)
                    }}
                  >
                    <Dialog.Trigger
                      render={
                        <Button
                          variant="outline"
                          className="flex w-full items-center gap-2"
                          disabled={impersonating}
                          onClick={() => setImpersonateDialogUser(selectedUser)}
                        >
                          <svg
                            aria-hidden="true"
                            className="h-4 w-4"
                            fill="none"
                            stroke="currentColor"
                            viewBox="0 0 24 24"
                          >
                            <path
                              strokeLinecap="round"
                              strokeLinejoin="round"
                              strokeWidth={2}
                              d="M8 7h12m0 0l-4-4m4 4l-4 4m0 6H4m0 0l4 4m-4-4l4-4"
                            />
                          </svg>
                          {t('admin.users.viewAsButton', { username: selectedUser.username })}
                        </Button>
                      }
                    />
                    <Dialog.Content showCloseButton={false}>
                      <Dialog.Title>
                        {t('admin.users.impersonateTitle', { username: selectedUser.username })}
                      </Dialog.Title>
                      <Dialog.Description>{t('admin.users.impersonateBody')}</Dialog.Description>
                      <div className="mt-6 flex justify-end gap-3">
                        <Dialog.Close
                          render={
                            <Button variant="outline">{t('admin.users.impersonateCancel')}</Button>
                          }
                        />
                        <Button
                          variant="default"
                          disabled={impersonating}
                          onClick={() => handleImpersonate(selectedUser.id)}
                        >
                          {impersonating
                            ? t('admin.users.impersonateSwitching')
                            : t('admin.users.impersonateConfirm')}
                        </Button>
                      </div>
                    </Dialog.Content>
                  </Dialog.Root>
                  <p className="mt-2 text-center text-xs text-text-dim">
                    {t('admin.users.impersonateHint')}
                  </p>
                </div>

                {/* Premium Section */}
                <div className="mt-6 border-t border-border pt-6">
                  {selectedUser.is_premium ? (
                    <>
                      <div className="mb-3 rounded-lg border border-amber-500/20 bg-amber-500/10 p-3">
                        <p className="text-sm font-medium text-amber-400">
                          {t('admin.users.premiumActiveTitle')}
                        </p>
                        <p className="mt-1 text-xs text-amber-400/70">
                          {t('admin.users.premiumActiveBody')}
                        </p>
                        {selectedUser.premium_expires_at && (
                          <p className="mt-1 text-xs font-medium text-amber-400/70">
                            {t('admin.users.premiumExpires', {
                              timestamp: formatTimestamp(new Date(selectedUser.premium_expires_at)),
                            })}
                          </p>
                        )}
                      </div>
                      <Dialog.Root
                        open={
                          premiumDialogUser?.id === selectedUser.id &&
                          premiumDialogUser?.is_premium === true
                        }
                        onOpenChange={(open) => {
                          if (!open) setPremiumDialogUser(null)
                        }}
                      >
                        <Dialog.Trigger
                          render={
                            <Button
                              variant="outline"
                              className="w-full"
                              aria-label={t('admin.users.revokePremiumLabel', {
                                username: selectedUser.username,
                              })}
                              onClick={() => setPremiumDialogUser(selectedUser)}
                            >
                              {t('admin.users.revokePremiumButton')}
                            </Button>
                          }
                        />
                        <Dialog.Content showCloseButton={false}>
                          <Dialog.Title>
                            {t('admin.users.revokePremiumTitle', {
                              username: selectedUser.username,
                            })}
                          </Dialog.Title>
                          <Dialog.Description>
                            {t('admin.users.revokePremiumBody')}
                          </Dialog.Description>
                          <div className="mt-6 flex justify-end gap-3">
                            <Dialog.Close
                              render={
                                <Button variant="outline" disabled={premiumLoading}>
                                  {t('admin.users.revokePremiumCancel')}
                                </Button>
                              }
                            />
                            <Button
                              variant="destructive"
                              disabled={premiumLoading}
                              onClick={() =>
                                handleSetPremium(selectedUser.id, selectedUser.username, false)
                              }
                            >
                              {premiumLoading
                                ? t('admin.users.saving')
                                : t('admin.users.revokePremiumButton')}
                            </Button>
                          </div>
                        </Dialog.Content>
                      </Dialog.Root>
                    </>
                  ) : (
                    <Dialog.Root
                      open={
                        premiumDialogUser?.id === selectedUser.id &&
                        premiumDialogUser?.is_premium === false
                      }
                      onOpenChange={(open) => {
                        if (!open) setPremiumDialogUser(null)
                      }}
                    >
                      <Dialog.Trigger
                        render={
                          <Button
                            variant="outline"
                            className="w-full border-amber-500/40 text-amber-400 hover:border-amber-500/60 hover:bg-amber-500/10"
                            aria-label={t('admin.users.grantPremiumLabel', {
                              username: selectedUser.username,
                            })}
                            onClick={() => {
                              // Reset the duration selection to the default (Permanent)
                              // each time the grant dialog opens.
                              setGrantDurationSeconds(null)
                              setGrantDurationValid(true)
                              setPremiumDialogUser(selectedUser)
                            }}
                          >
                            {t('admin.users.grantPremiumButton')}
                          </Button>
                        }
                      />
                      <Dialog.Content showCloseButton={false}>
                        <Dialog.Title>
                          {t('admin.users.grantPremiumTitle', { username: selectedUser.username })}
                        </Dialog.Title>
                        <Dialog.Description>{t('admin.users.grantPremiumBody')}</Dialog.Description>
                        <PremiumDurationChooser
                          disabled={premiumLoading}
                          onChange={(seconds, valid) => {
                            setGrantDurationSeconds(seconds)
                            setGrantDurationValid(valid)
                          }}
                        />
                        <div className="mt-6 flex justify-end gap-3">
                          <Dialog.Close
                            render={
                              <Button variant="outline" disabled={premiumLoading}>
                                {t('admin.users.grantPremiumCancel')}
                              </Button>
                            }
                          />
                          <Button
                            variant="default"
                            disabled={premiumLoading || !grantDurationValid}
                            onClick={() =>
                              handleSetPremium(
                                selectedUser.id,
                                selectedUser.username,
                                true,
                                grantDurationSeconds
                              )
                            }
                          >
                            {premiumLoading
                              ? t('admin.users.saving')
                              : t('admin.users.grantPremiumButton')}
                          </Button>
                        </div>
                      </Dialog.Content>
                    </Dialog.Root>
                  )}
                </div>

                {/* Beta Tester Section (ADR-0020): all premium + early-access features */}
                <div className="mt-6 border-t border-border pt-6">
                  {selectedUser.is_beta_tester ? (
                    <>
                      <div className="mb-3 rounded-lg border border-violet-500/20 bg-violet-500/10 p-3">
                        <p className="text-sm font-medium text-violet-400">
                          {t('admin.users.betaActiveTitle')}
                        </p>
                        <p className="mt-1 text-xs text-violet-400/70">
                          {t('admin.users.betaActiveBody')}
                        </p>
                      </div>
                      <Dialog.Root
                        open={
                          betaDialogUser?.id === selectedUser.id &&
                          betaDialogUser?.is_beta_tester === true
                        }
                        onOpenChange={(open) => {
                          if (!open) setBetaDialogUser(null)
                        }}
                      >
                        <Dialog.Trigger
                          render={
                            <Button
                              variant="outline"
                              className="w-full"
                              aria-label={t('admin.users.revokeBetaLabel', {
                                username: selectedUser.username,
                              })}
                              onClick={() => setBetaDialogUser(selectedUser)}
                            >
                              {t('admin.users.revokeBetaButton')}
                            </Button>
                          }
                        />
                        <Dialog.Content showCloseButton={false}>
                          <Dialog.Title>
                            {t('admin.users.revokeBetaTitle', { username: selectedUser.username })}
                          </Dialog.Title>
                          <Dialog.Description>{t('admin.users.revokeBetaBody')}</Dialog.Description>
                          <div className="mt-6 flex justify-end gap-3">
                            <Dialog.Close
                              render={
                                <Button variant="outline" disabled={betaLoading}>
                                  {t('admin.users.revokeBetaCancel')}
                                </Button>
                              }
                            />
                            <Button
                              variant="destructive"
                              disabled={betaLoading}
                              onClick={() =>
                                handleSetBetaTester(selectedUser.id, selectedUser.username, false)
                              }
                            >
                              {betaLoading
                                ? t('admin.users.saving')
                                : t('admin.users.revokeBetaButton')}
                            </Button>
                          </div>
                        </Dialog.Content>
                      </Dialog.Root>
                    </>
                  ) : (
                    <Dialog.Root
                      open={
                        betaDialogUser?.id === selectedUser.id &&
                        betaDialogUser?.is_beta_tester === false
                      }
                      onOpenChange={(open) => {
                        if (!open) setBetaDialogUser(null)
                      }}
                    >
                      <Dialog.Trigger
                        render={
                          <Button
                            variant="outline"
                            className="w-full border-violet-500/40 text-violet-400 hover:border-violet-500/60 hover:bg-violet-500/10"
                            aria-label={t('admin.users.grantBetaLabel', {
                              username: selectedUser.username,
                            })}
                            onClick={() => setBetaDialogUser(selectedUser)}
                          >
                            {t('admin.users.grantBetaButton')}
                          </Button>
                        }
                      />
                      <Dialog.Content showCloseButton={false}>
                        <Dialog.Title>
                          {t('admin.users.grantBetaTitle', { username: selectedUser.username })}
                        </Dialog.Title>
                        <Dialog.Description>{t('admin.users.grantBetaBody')}</Dialog.Description>
                        <div className="mt-6 flex justify-end gap-3">
                          <Dialog.Close
                            render={
                              <Button variant="outline" disabled={betaLoading}>
                                {t('admin.users.grantBetaCancel')}
                              </Button>
                            }
                          />
                          <Button
                            variant="default"
                            disabled={betaLoading}
                            onClick={() =>
                              handleSetBetaTester(selectedUser.id, selectedUser.username, true)
                            }
                          >
                            {betaLoading
                              ? t('admin.users.saving')
                              : t('admin.users.grantBetaButton')}
                          </Button>
                        </div>
                      </Dialog.Content>
                    </Dialog.Root>
                  )}
                </div>

                {/* Ambassador Section (ADR-0041): premium + early-access + public showcase */}
                <div className="mt-6 border-t border-border pt-6">
                  {selectedUser.is_ambassador ? (
                    <>
                      <div className="mb-3 rounded-lg border border-sky-500/20 bg-sky-500/10 p-3">
                        <p className="text-sm font-medium text-sky-400">
                          {t('admin.users.ambassadorActiveTitle')}
                        </p>
                        <p className="mt-1 text-xs text-sky-400/70">
                          {t('admin.users.ambassadorActiveBody')}
                        </p>
                      </div>

                      {/* Admin-curated showcase card */}
                      <div className="space-y-3">
                        <div>
                          <p className="mb-1 text-xs font-medium text-text-sub">
                            {t('admin.users.taglineLabel')}
                          </p>
                          <input
                            type="text"
                            value={ambassadorTagline}
                            onChange={(e) => setAmbassadorTagline(e.target.value)}
                            maxLength={120}
                            placeholder={t('admin.users.taglinePlaceholder')}
                            aria-label={t('admin.users.taglineFieldLabel')}
                            className="w-full rounded-lg border border-border bg-surface-2 px-4 py-2 text-text placeholder:text-text-dim focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
                          />
                        </div>
                        <div>
                          <p className="mb-1 text-xs font-medium text-text-sub">
                            {t('admin.users.sortOrderLabel')}
                          </p>
                          <input
                            type="number"
                            value={ambassadorSortOrder}
                            onChange={(e) => setAmbassadorSortOrder(e.target.value)}
                            aria-label={t('admin.users.sortOrderFieldLabel')}
                            className="w-full rounded-lg border border-border bg-surface-2 px-4 py-2 text-text placeholder:text-text-dim focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
                          />
                        </div>
                        <Button
                          variant="outline"
                          className="w-full"
                          disabled={ambassadorLoading}
                          aria-label={t('admin.users.saveShowcaseLabel', {
                            username: selectedUser.username,
                          })}
                          onClick={() =>
                            handleSetAmbassador(selectedUser.id, selectedUser.username, true, {
                              tagline: ambassadorTagline,
                              sortOrder: Number.parseInt(ambassadorSortOrder, 10) || 0,
                            })
                          }
                        >
                          {ambassadorLoading
                            ? t('admin.users.saving')
                            : t('admin.users.saveShowcaseButton')}
                        </Button>
                      </div>

                      <Dialog.Root
                        open={ambassadorDialogUser?.id === selectedUser.id}
                        onOpenChange={(open) => {
                          if (!open) setAmbassadorDialogUser(null)
                        }}
                      >
                        <Dialog.Trigger
                          render={
                            <Button
                              variant="outline"
                              className="mt-3 w-full"
                              aria-label={t('admin.users.revokeAmbassadorLabel', {
                                username: selectedUser.username,
                              })}
                              onClick={() => setAmbassadorDialogUser(selectedUser)}
                            >
                              {t('admin.users.revokeAmbassadorButton')}
                            </Button>
                          }
                        />
                        <Dialog.Content showCloseButton={false}>
                          <Dialog.Title>
                            {t('admin.users.revokeAmbassadorTitle', {
                              username: selectedUser.username,
                            })}
                          </Dialog.Title>
                          <Dialog.Description>
                            {t('admin.users.revokeAmbassadorBody')}
                          </Dialog.Description>
                          <div className="mt-6 flex justify-end gap-3">
                            <Dialog.Close
                              render={
                                <Button variant="outline" disabled={ambassadorLoading}>
                                  {t('admin.users.revokeAmbassadorCancel')}
                                </Button>
                              }
                            />
                            <Button
                              variant="destructive"
                              disabled={ambassadorLoading}
                              onClick={() =>
                                handleSetAmbassador(selectedUser.id, selectedUser.username, false)
                              }
                            >
                              {ambassadorLoading
                                ? t('admin.users.saving')
                                : t('admin.users.revokeAmbassadorButton')}
                            </Button>
                          </div>
                        </Dialog.Content>
                      </Dialog.Root>
                    </>
                  ) : (
                    <div className="space-y-3">
                      <div>
                        <p className="mb-1 text-xs font-medium text-text-sub">
                          {t('admin.users.taglineOptionalLabel')}
                        </p>
                        <input
                          type="text"
                          value={ambassadorTagline}
                          onChange={(e) => setAmbassadorTagline(e.target.value)}
                          maxLength={120}
                          placeholder={t('admin.users.taglinePlaceholder')}
                          aria-label={t('admin.users.taglineFieldLabel')}
                          className="w-full rounded-lg border border-border bg-surface-2 px-4 py-2 text-text placeholder:text-text-dim focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
                        />
                      </div>
                      <Button
                        variant="outline"
                        className="w-full border-sky-500/40 text-sky-400 hover:border-sky-500/60 hover:bg-sky-500/10"
                        disabled={ambassadorLoading}
                        aria-label={t('admin.users.grantAmbassadorLabel', {
                          username: selectedUser.username,
                        })}
                        onClick={() =>
                          handleSetAmbassador(selectedUser.id, selectedUser.username, true, {
                            tagline: ambassadorTagline,
                            sortOrder: Number.parseInt(ambassadorSortOrder, 10) || 0,
                          })
                        }
                      >
                        {ambassadorLoading
                          ? t('admin.users.saving')
                          : t('admin.users.grantAmbassadorButton')}
                      </Button>
                    </div>
                  )}
                </div>

                {/* Ban/Unban Section */}
                <div className="mt-6 border-t border-border pt-6">
                  {selectedUser.is_banned ? (
                    <>
                      <div className="mb-3 rounded-lg border border-destructive/20 bg-destructive/10 p-3">
                        <p className="text-sm font-medium text-destructive">
                          {t('admin.users.bannedReason', {
                            reason: selectedUser.banned_reason ?? '',
                          })}
                        </p>
                        <p className="mt-1 text-xs text-destructive/70">
                          {selectedUser.banned_at &&
                            t('admin.users.bannedOn', {
                              timestamp: formatTimestamp(new Date(selectedUser.banned_at)),
                            })}
                        </p>
                      </div>
                      {/* Unban — Dialog confirmation */}
                      <Dialog.Root
                        open={unbanDialogUser?.id === selectedUser.id}
                        onOpenChange={(open) => {
                          if (!open) setUnbanDialogUser(null)
                        }}
                      >
                        <Dialog.Trigger
                          render={
                            <Button
                              variant="outline"
                              className="w-full"
                              aria-label={t('admin.users.unbanLabel', {
                                username: selectedUser.username,
                              })}
                              onClick={() => setUnbanDialogUser(selectedUser)}
                            >
                              {t('admin.users.unbanButton')}
                            </Button>
                          }
                        />
                        <Dialog.Content showCloseButton={false}>
                          <Dialog.Title>
                            {t('admin.users.unbanTitle', { username: selectedUser.username })}
                          </Dialog.Title>
                          <Dialog.Description>{t('admin.users.unbanBody')}</Dialog.Description>
                          <div className="mt-6 flex justify-end gap-3">
                            <Dialog.Close
                              render={
                                <Button variant="outline">{t('admin.users.unbanCancel')}</Button>
                              }
                            />
                            <Button
                              variant="default"
                              onClick={() =>
                                handleUnbanUser(selectedUser.id, selectedUser.username)
                              }
                            >
                              {t('admin.users.unbanButton')}
                            </Button>
                          </div>
                        </Dialog.Content>
                      </Dialog.Root>
                    </>
                  ) : (
                    <Button
                      variant="destructive"
                      className="w-full"
                      aria-label={t('admin.users.banLabel', {
                        username: selectedUser.username,
                      })}
                      onClick={() => {
                        setUserToBan(selectedUser)
                        setBanReason('')
                        setShowBanModal(true)
                      }}
                    >
                      {t('admin.users.banButton')}
                    </Button>
                  )}
                </div>

                <div className="mt-6 border-t border-border pt-6">
                  <h4 className="mb-2 text-sm font-medium text-text-sub">
                    {t('admin.users.overlaysHeading', { count: userOverlays.length })}
                  </h4>
                  {userOverlays.length > 0 ? (
                    <ul className="space-y-2">
                      {userOverlays.map((overlay) => (
                        <li
                          key={overlay.id}
                          className="flex items-center gap-2 rounded-lg border border-border bg-surface-2 px-3 py-2 transition-colors hover:bg-surface-2/80"
                        >
                          {/* Primary: admin detail (owner-linked, in-app). */}
                          <Link
                            href={`/admin/overlays?overlay=${overlay.id}`}
                            className="min-w-0 flex-1"
                          >
                            <div className="truncate text-sm font-medium text-text">
                              {overlay.name}
                            </div>
                            <div className="text-xs text-text-sub">
                              {t('admin.users.overlaySourceCount', {
                                count: overlay.sources_count,
                              })}
                            </div>
                          </Link>
                          {/* Secondary: open the live overlay in a new tab. */}
                          <Link
                            href={`/overlay/${overlay.id}`}
                            target="_blank"
                            rel="noopener noreferrer"
                            aria-label={t('admin.users.openOverlayLabel', { name: overlay.name })}
                            className="shrink-0 text-text-dim transition-colors hover:text-text"
                          >
                            <svg
                              aria-hidden="true"
                              className="h-4 w-4"
                              fill="none"
                              stroke="currentColor"
                              viewBox="0 0 24 24"
                            >
                              <path
                                strokeLinecap="round"
                                strokeLinejoin="round"
                                strokeWidth="2"
                                d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14"
                              />
                            </svg>
                          </Link>
                        </li>
                      ))}
                    </ul>
                  ) : (
                    <p className="text-sm text-text-dim italic">{t('admin.users.overlaysEmpty')}</p>
                  )}
                  <Link
                    href={`/admin/sources?user=${selectedUser.id}`}
                    className="mt-3 inline-block text-xs font-medium text-primary hover:underline"
                  >
                    {t('admin.users.viewSourcesLink')}
                  </Link>
                </div>
              </div>
            </Card>
          ) : (
            <Card className="p-6 text-center">
              <svg
                aria-hidden="true"
                className="mx-auto h-12 w-12 text-text-dim"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth="2"
                  d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z"
                />
              </svg>
              <p className="mt-2 text-sm text-text-sub">{t('admin.users.selectPrompt')}</p>
            </Card>
          )}
        </div>
      </div>

      {/* Ban Modal — Dialog for ban with reason input */}
      <Dialog.Root
        open={showBanModal}
        onOpenChange={(open) => {
          if (!open) {
            setShowBanModal(false)
            setUserToBan(null)
            setBanReason('')
          }
        }}
      >
        <Dialog.Content showCloseButton={false}>
          <Dialog.Title>
            {t('admin.users.banTitle', { username: userToBan?.username ?? '' })}
          </Dialog.Title>
          <Dialog.Description>{t('admin.users.banBody')}</Dialog.Description>
          <div className="mt-4">
            <label htmlFor={banReasonId} className="mb-2 block text-sm font-medium text-text-sub">
              {t('admin.users.banReasonLabel')}
            </label>
            <textarea
              id={banReasonId}
              value={banReason}
              onChange={(e) => setBanReason(e.target.value)}
              className="w-full resize-none rounded-lg border border-border bg-surface-2 px-3 py-2 text-text placeholder:text-text-dim focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
              rows={3}
              placeholder={t('admin.users.banReasonPlaceholder')}
            />
          </div>
          <div className="mt-6 flex justify-end gap-3">
            <Dialog.Close
              render={
                <Button variant="outline" disabled={banLoading}>
                  {t('admin.users.banCancel')}
                </Button>
              }
            />
            <Button
              variant="destructive"
              disabled={banLoading || !banReason.trim()}
              onClick={() => handleBanUser(banReason)}
            >
              {banLoading ? t('admin.users.banning') : t('admin.users.banButton')}
            </Button>
          </div>
        </Dialog.Content>
      </Dialog.Root>
    </div>
  )
}
