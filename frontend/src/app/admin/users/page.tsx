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


import { useEffect, useState } from 'react'
import { useRouter } from 'next/navigation'
import Link from 'next/link'
import clsx from 'clsx'
import { Card } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { Dialog } from '@/components/ui/dialog'
import { toastManager } from '@/lib/toast'
import { useAuthStore } from '@/lib/stores/auth-store'

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
  is_banned: boolean
  banned_at?: string
  banned_reason?: string
  banned_by?: string
}

interface UserOverlay {
  id: string
  name: string
  sources_count: number
}

export default function UsersPage() {
  const router = useRouter()
  const { token: authToken, startImpersonation, init: initAuth } = useAuthStore()
  const [users, setUsers] = useState<User[]>([])
  const [selectedUser, setSelectedUser] = useState<User | null>(null)
  const [userOverlays, setUserOverlays] = useState<UserOverlay[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [impersonating, setImpersonating] = useState(false)
  const [searchTerm, setSearchTerm] = useState('')
  const [filter, setFilter] = useState<'all' | 'active' | 'banned' | 'premium'>('all')
  const [showBanModal, setShowBanModal] = useState(false)
  const [userToBan, setUserToBan] = useState<User | null>(null)
  const [banReason, setBanReason] = useState('')
  const [banLoading, setBanLoading] = useState(false)
  const [impersonateDialogUser, setImpersonateDialogUser] = useState<User | null>(null)
  const [unbanDialogUser, setUnbanDialogUser] = useState<User | null>(null)
  const [premiumDialogUser, setPremiumDialogUser] = useState<User | null>(null)
  const [premiumLoading, setPremiumLoading] = useState(false)

  // Fetch all users from the database
  useEffect(() => {
    async function fetchUsers() {
      try {
        const token = localStorage.getItem('jwt_token')
        if (!token) {
          setError('Not authenticated')
          setLoading(false)
          return
        }

        const response = await fetch('/api/v1/admin/users', {
          headers: {
            Authorization: `Bearer ${token}`,
          },
        })

        if (!response.ok) {
          throw new Error(`HTTP ${response.status}: ${response.statusText}`)
        }

        const data = await response.json()
        setUsers(data)
        setLoading(false)
      } catch (err) {
        console.error('Failed to load users:', err)
        setError('Failed to load users')
        setLoading(false)
      }
    }

    fetchUsers()
  }, [])

  // Fetch overlays for selected user
  useEffect(() => {
    async function fetchUserOverlays() {
      if (!selectedUser) {
        setUserOverlays([])
        return
      }

      try {
        const token = localStorage.getItem('jwt_token')
        const response = await fetch(`/api/v1/admin/user-overlays/${selectedUser.id}`, {
          headers: {
            Authorization: `Bearer ${token}`,
          },
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

  // Refetch users helper
  const refetchUsers = async () => {
    try {
      const token = localStorage.getItem('jwt_token')
      const response = await fetch('/api/v1/admin/users', {
        headers: { Authorization: `Bearer ${token}` },
      })
      if (response.ok) {
        const data = await response.json()
        setUsers(data)
      }
    } catch (err) {
      console.error('Failed to refetch users:', err)
    }
  }

  // Handle impersonation (called from Dialog confirm)
  const handleImpersonate = async (userId: string) => {
    setImpersonating(true)
    try {
      const token = authToken || localStorage.getItem('jwt_token')
      const response = await fetch(`/api/v1/admin/users/${userId}/impersonate`, {
        method: 'POST',
        headers: {
          Authorization: `Bearer ${token}`,
          'Content-Type': 'application/json',
        },
      })

      if (!response.ok) {
        throw new Error('Failed to impersonate user')
      }

      const data = await response.json()

      // Update the auth store with the impersonation token — this also updates localStorage
      // atomically and makes the banner visible immediately via reactive store subscription.
      startImpersonation(data.token, data.username)

      // Re-initialise auth store so user object reflects impersonated user
      await initAuth()

      // Redirect to home page
      router.push('/')
    } catch (err) {
      console.error('Failed to impersonate user:', err)
      toastManager.add({ title: 'Failed to start impersonation. Please try again.', type: 'error' })
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
      const token = localStorage.getItem('jwt_token')
      const response = await fetch(`/api/v1/admin/users/${userToBan.id}/ban`, {
        method: 'POST',
        headers: {
          Authorization: `Bearer ${token}`,
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ reason }),
      })

      if (!response.ok) {
        const errorData = await response.json()
        throw new Error(errorData.error || 'Failed to ban user')
      }

      toastManager.add({ title: `${userToBan.username} banned successfully`, type: 'success' })
      setShowBanModal(false)
      setUserToBan(null)
      setBanReason('')
      await refetchUsers()

      // Clear selected user if it was the banned one
      if (selectedUser?.id === userToBan.id) {
        setSelectedUser(null)
      }
    } catch (err: any) {
      toastManager.add({ title: err.message || 'Failed to ban user', type: 'error' })
    } finally {
      setBanLoading(false)
    }
  }

  // Handle unban user (called from Dialog confirm)
  const handleUnbanUser = async (userId: string, username: string) => {
    try {
      const token = localStorage.getItem('jwt_token')
      const response = await fetch(`/api/v1/admin/users/${userId}/unban`, {
        method: 'POST',
        headers: {
          Authorization: `Bearer ${token}`,
        },
      })

      if (!response.ok) {
        const errorData = await response.json()
        throw new Error(errorData.error || 'Failed to unban user')
      }

      toastManager.add({ title: `${username} unbanned successfully`, type: 'success' })
      setUnbanDialogUser(null)
      await refetchUsers()
    } catch (err: any) {
      toastManager.add({ title: err.message || 'Failed to unban user', type: 'error' })
    }
  }

  // Handle premium toggle
  const handleSetPremium = async (userId: string, username: string, isPremium: boolean) => {
    setPremiumLoading(true)
    try {
      const token = localStorage.getItem('jwt_token')
      const response = await fetch(`/api/v1/admin/premium/users/${userId}`, {
        method: 'POST',
        headers: {
          Authorization: `Bearer ${token}`,
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ is_premium: isPremium }),
      })

      if (!response.ok) {
        const errorData = await response.json()
        throw new Error(errorData.error || 'Failed to update premium status')
      }

      toastManager.add({
        title: isPremium
          ? `${username} granted premium access`
          : `${username} premium access removed`,
        type: 'success',
      })
      setPremiumDialogUser(null)
      await refetchUsers()

      // Update selectedUser in place so the panel reflects the change immediately
      if (selectedUser?.id === userId) {
        setSelectedUser((u) => (u ? { ...u, is_premium: isPremium } : u))
      }
    } catch (err: any) {
      toastManager.add({ title: err.message || 'Failed to update premium status', type: 'error' })
    } finally {
      setPremiumLoading(false)
    }
  }

  // Filter and search users
  const displayUsers = users.filter((u) => {
    // Filter by status
    if (filter === 'banned' && !u.is_banned) return false
    if (filter === 'active' && u.is_banned) return false
    if (filter === 'premium' && !u.is_premium) return false

    // Search filter
    if (searchTerm) {
      const term = searchTerm.toLowerCase()
      return (
        u.username.toLowerCase().includes(term) ||
        u.display_name.toLowerCase().includes(term) ||
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

  return (
    <div className="mx-auto max-w-7xl px-4 py-8">
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-text">Users</h1>
        <p className="mt-1 text-sm text-text-sub">Manage and view all users in the system</p>
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
                <h3 className="text-base font-medium text-text">All Users ({users.length})</h3>

                {/* Search Input */}
                <div className="mt-4">
                  <input
                    type="text"
                    placeholder="Search by username, display name, or platform ID..."
                    value={searchTerm}
                    onChange={(e) => setSearchTerm(e.target.value)}
                    className="focus-visible:ring-ring w-full rounded-lg border border-border bg-surface-2 px-4 py-2 text-text placeholder:text-text-dim focus-visible:ring-2 focus-visible:outline-none"
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
                    All ({users.length})
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
                    Active ({activeCount})
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
                    Banned ({bannedCount})
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
                    Premium ({premiumCount})
                  </button>
                </div>
              </div>
              <ul className="divide-y divide-border">
                {displayUsers.map((user) => (
                  <li
                    key={user.id}
                    className={clsx(
                      'cursor-pointer px-4 py-4 transition-colors hover:bg-surface-2',
                      selectedUser?.id === user.id && 'bg-surface-2'
                    )}
                    onClick={() => setSelectedUser(user)}
                  >
                    <div className="flex items-center justify-between">
                      <div className="flex-1">
                        <div className="flex items-center">
                          <p className="text-sm font-medium text-text">{user.display_name}</p>
                          <div className="ml-2 flex space-x-1">
                            {user.is_premium && (
                              <span className="inline-flex items-center rounded border border-amber-500/20 bg-amber-500/10 px-2 py-0.5 text-xs font-medium text-amber-400">
                                PREMIUM
                              </span>
                            )}
                            {user.is_banned && (
                              <span className="bg-destructive/10 text-destructive border-destructive/20 inline-flex items-center rounded border px-2 py-0.5 text-xs font-medium">
                                BANNED
                              </span>
                            )}
                            {user.twitch_id && (
                              <span className="inline-flex items-center rounded bg-badge-bg px-2 py-0.5 text-xs font-medium text-twitch">
                                Twitch
                              </span>
                            )}
                            {user.youtube_id && (
                              <span className="inline-flex items-center rounded bg-badge-bg px-2 py-0.5 text-xs font-medium text-youtube">
                                YouTube
                              </span>
                            )}
                            {user.kick_id && (
                              <span className="inline-flex items-center rounded bg-badge-bg px-2 py-0.5 text-xs font-medium text-kick">
                                Kick
                              </span>
                            )}
                          </div>
                        </div>
                        <p className="text-sm text-text-sub">@{user.username}</p>
                        <p className="mt-1 text-xs text-text-dim">
                          Joined {new Date(user.created_at).toLocaleDateString()}
                        </p>
                      </div>
                      <div>
                        <svg
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
                  </li>
                ))}
              </ul>
            </Card>
          )}
        </div>

        {/* User Details Panel */}
        <div className="lg:col-span-1">
          {selectedUser ? (
            <Card className="overflow-hidden">
              <div className="border-b border-border px-4 py-5">
                <h3 className="text-base font-medium text-text">User Details</h3>
              </div>
              <div className="px-4 py-5">
                <dl className="space-y-4">
                  <div>
                    <dt className="text-sm font-medium text-text-sub">ID</dt>
                    <dd className="mt-1 font-mono text-sm break-all text-text">
                      {selectedUser.id}
                    </dd>
                  </div>
                  <div>
                    <dt className="text-sm font-medium text-text-sub">Username</dt>
                    <dd className="mt-1 text-sm text-text">{selectedUser.username}</dd>
                  </div>
                  <div>
                    <dt className="text-sm font-medium text-text-sub">Display Name</dt>
                    <dd className="mt-1 text-sm text-text">{selectedUser.display_name}</dd>
                  </div>
                  <div>
                    <dt className="text-sm font-medium text-text-sub">Auth Provider</dt>
                    <dd className="mt-1 text-sm text-text capitalize">
                      {selectedUser.auth_provider}
                    </dd>
                  </div>
                  <div>
                    <dt className="text-sm font-medium text-text-sub">Connected Platforms</dt>
                    <dd className="mt-2 space-y-1">
                      {selectedUser.twitch_id && (
                        <div className="flex items-center text-sm">
                          <span className="font-medium text-twitch">Twitch:</span>
                          <span className="ml-2 font-mono text-xs text-text-sub">
                            {selectedUser.twitch_id}
                          </span>
                        </div>
                      )}
                      {selectedUser.youtube_id && (
                        <div className="flex items-center text-sm">
                          <span className="font-medium text-youtube">YouTube:</span>
                          <span className="ml-2 font-mono text-xs text-text-sub">
                            {selectedUser.youtube_id}
                          </span>
                        </div>
                      )}
                      {selectedUser.kick_id && (
                        <div className="flex items-center text-sm">
                          <span className="font-medium text-kick">Kick:</span>
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
                          View as {selectedUser.username}
                        </Button>
                      }
                    />
                    <Dialog.Content showCloseButton={false}>
                      <Dialog.Title>
                        Impersonate &ldquo;{selectedUser.username}&rdquo;?
                      </Dialog.Title>
                      <Dialog.Description>
                        This will replace your current session. You can return to admin by using the
                        stored admin token.
                      </Dialog.Description>
                      <div className="mt-6 flex justify-end gap-3">
                        <Dialog.Close render={<Button variant="outline">Cancel</Button>} />
                        <Button
                          variant="default"
                          disabled={impersonating}
                          onClick={() => handleImpersonate(selectedUser.id)}
                        >
                          {impersonating ? 'Switching...' : 'Impersonate'}
                        </Button>
                      </div>
                    </Dialog.Content>
                  </Dialog.Root>
                  <p className="mt-2 text-center text-xs text-text-dim">
                    Temporarily act as this user to debug issues
                  </p>
                </div>

                {/* Premium Section */}
                <div className="mt-6 border-t border-border pt-6">
                  {selectedUser.is_premium ? (
                    <>
                      <div className="mb-3 rounded-lg border border-amber-500/20 bg-amber-500/10 p-3">
                        <p className="text-sm font-medium text-amber-400">Premium access active</p>
                        <p className="mt-1 text-xs text-amber-400/70">
                          This user can create and accept share requests.
                        </p>
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
                              onClick={() => setPremiumDialogUser(selectedUser)}
                            >
                              Revoke Premium
                            </Button>
                          }
                        />
                        <Dialog.Content showCloseButton={false}>
                          <Dialog.Title>
                            Revoke premium for &ldquo;{selectedUser.username}&rdquo;?
                          </Dialog.Title>
                          <Dialog.Description>
                            They will no longer be able to create or accept share requests.
                          </Dialog.Description>
                          <div className="mt-6 flex justify-end gap-3">
                            <Dialog.Close
                              render={
                                <Button variant="outline" disabled={premiumLoading}>
                                  Cancel
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
                              {premiumLoading ? 'Saving...' : 'Revoke Premium'}
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
                            onClick={() => setPremiumDialogUser(selectedUser)}
                          >
                            Grant Premium
                          </Button>
                        }
                      />
                      <Dialog.Content showCloseButton={false}>
                        <Dialog.Title>
                          Grant premium to &ldquo;{selectedUser.username}&rdquo;?
                        </Dialog.Title>
                        <Dialog.Description>
                          They will be able to create and accept chat overlay share requests.
                        </Dialog.Description>
                        <div className="mt-6 flex justify-end gap-3">
                          <Dialog.Close
                            render={
                              <Button variant="outline" disabled={premiumLoading}>
                                Cancel
                              </Button>
                            }
                          />
                          <Button
                            variant="default"
                            disabled={premiumLoading}
                            onClick={() =>
                              handleSetPremium(selectedUser.id, selectedUser.username, true)
                            }
                          >
                            {premiumLoading ? 'Saving...' : 'Grant Premium'}
                          </Button>
                        </div>
                      </Dialog.Content>
                    </Dialog.Root>
                  )}
                </div>

                {/* Ban/Unban Section */}
                <div className="mt-6 border-t border-border pt-6">
                  {selectedUser.is_banned ? (
                    <>
                      <div className="bg-destructive/10 border-destructive/20 mb-3 rounded-lg border p-3">
                        <p className="text-destructive text-sm font-medium">
                          Banned: {selectedUser.banned_reason}
                        </p>
                        <p className="text-destructive/70 mt-1 text-xs">
                          {selectedUser.banned_at &&
                            `Banned on ${new Date(selectedUser.banned_at).toLocaleString()}`}
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
                              onClick={() => setUnbanDialogUser(selectedUser)}
                            >
                              Unban User
                            </Button>
                          }
                        />
                        <Dialog.Content showCloseButton={false}>
                          <Dialog.Title>Unban &ldquo;{selectedUser.username}&rdquo;?</Dialog.Title>
                          <Dialog.Description>
                            This will restore their access to the platform.
                          </Dialog.Description>
                          <div className="mt-6 flex justify-end gap-3">
                            <Dialog.Close render={<Button variant="outline">Cancel</Button>} />
                            <Button
                              variant="default"
                              onClick={() =>
                                handleUnbanUser(selectedUser.id, selectedUser.username)
                              }
                            >
                              Unban User
                            </Button>
                          </div>
                        </Dialog.Content>
                      </Dialog.Root>
                    </>
                  ) : (
                    <Button
                      variant="destructive"
                      className="w-full"
                      onClick={() => {
                        setUserToBan(selectedUser)
                        setBanReason('')
                        setShowBanModal(true)
                      }}
                    >
                      Ban User
                    </Button>
                  )}
                </div>

                <div className="mt-6 border-t border-border pt-6">
                  <h4 className="mb-2 text-sm font-medium text-text-sub">
                    Overlays ({userOverlays.length})
                  </h4>
                  {userOverlays.length > 0 ? (
                    <ul className="space-y-2">
                      {userOverlays.map((overlay) => (
                        <li key={overlay.id}>
                          <Link
                            href={`/overlay/${overlay.id}`}
                            target="_blank"
                            className="block rounded-lg border border-border bg-surface-2 px-3 py-2 transition-colors hover:bg-surface-2/80"
                          >
                            <div className="flex items-center justify-between">
                              <div>
                                <div className="text-sm font-medium text-text">{overlay.name}</div>
                                <div className="text-xs text-text-sub">
                                  {overlay.sources_count} sources
                                </div>
                              </div>
                              <svg
                                className="h-4 w-4 text-text-dim"
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
                            </div>
                          </Link>
                        </li>
                      ))}
                    </ul>
                  ) : (
                    <p className="text-sm text-text-dim italic">No overlays yet</p>
                  )}
                </div>
              </div>
            </Card>
          ) : (
            <Card className="p-6 text-center">
              <svg
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
              <p className="mt-2 text-sm text-text-sub">Select a user to view details</p>
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
          <Dialog.Title>Ban &ldquo;{userToBan?.username}&rdquo;?</Dialog.Title>
          <Dialog.Description>
            This will prevent the user from accessing the platform.
          </Dialog.Description>
          <div className="mt-4">
            <label className="mb-2 block text-sm font-medium text-text-sub">Reason for ban *</label>
            <textarea
              value={banReason}
              onChange={(e) => setBanReason(e.target.value)}
              className="focus-visible:ring-ring w-full resize-none rounded-lg border border-border bg-surface-2 px-3 py-2 text-text placeholder:text-text-dim focus-visible:ring-2 focus-visible:outline-none"
              rows={3}
              placeholder="Spam, abuse, ToS violation, etc..."
            />
          </div>
          <div className="mt-6 flex justify-end gap-3">
            <Dialog.Close
              render={
                <Button variant="outline" disabled={banLoading}>
                  Cancel
                </Button>
              }
            />
            <Button
              variant="destructive"
              disabled={banLoading || !banReason.trim()}
              onClick={() => handleBanUser(banReason)}
            >
              {banLoading ? 'Banning...' : 'Ban User'}
            </Button>
          </div>
        </Dialog.Content>
      </Dialog.Root>
    </div>
  )
}
