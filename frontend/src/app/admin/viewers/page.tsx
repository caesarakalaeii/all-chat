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
 * Allows admins to view all viewer sessions and ban/unban users.
 *
 * Features:
 * - List all viewer sessions
 * - View message counts and rate limits
 * - Ban viewers with reason
 * - Unban viewers
 * - See ban status and reasons
 *
 * Route: /admin/viewers
 */

'use client'

import { useEffect, useState } from 'react'
import { useRouter } from 'next/navigation'
import { useAuthStore } from '@/lib/stores/auth-store'
import { apiClient } from '@/lib/api/client'
import { formatDistanceToNow } from 'date-fns'
import { Card } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { Dialog } from '@/components/ui/dialog'
import { toastManager } from '@/lib/toast'

interface ViewerSession {
  id: string
  platform: string
  platform_user_id: string
  username: string
  display_name: string
  last_message_at: string | null
  message_count_1min: number
  message_count_1hour: number
  is_premium: boolean
  viewer_id: string | null
  is_banned: boolean
  banned_at: string | null
  banned_reason: string | null
  created_at: string
}

export default function AdminViewersPage() {
  const router = useRouter()
  const { user } = useAuthStore()

  const [viewers, setViewers] = useState<ViewerSession[]>([])
  const [loading, setLoading] = useState(true)
  const [banningId, setBanningId] = useState<string | null>(null)
  const [banReason, setBanReason] = useState('')
  const [showBanModal, setShowBanModal] = useState(false)
  const [selectedViewer, setSelectedViewer] = useState<ViewerSession | null>(null)
  const [unbanDialogViewer, setUnbanDialogViewer] = useState<ViewerSession | null>(null)
  const [premiumDialogViewer, setPremiumDialogViewer] = useState<ViewerSession | null>(null)
  const [premiumLoading, setPremiumLoading] = useState(false)

  useEffect(() => {
    if (!user?.is_admin) {
      router.push('/dashboard')
      return
    }

    fetchViewers()
  }, [user, router])

  const fetchViewers = async () => {
    try {
      setLoading(true)
      const response = await apiClient.get<{ viewers: ViewerSession[] }>(
        '/api/v1/admin/viewers?limit=100'
      )
      setViewers(response.viewers)
    } catch (error) {
      console.error('Failed to fetch viewers:', error)
      toastManager.add({ title: 'Failed to load viewers', type: 'error' })
    } finally {
      setLoading(false)
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
      fetchViewers()
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
      fetchViewers()
    } catch (error) {
      console.error('Failed to unban viewer:', error)
      toastManager.add({ title: 'Failed to unban viewer', type: 'error' })
    } finally {
      setBanningId(null)
    }
  }

  const handleTogglePremium = async (viewer: ViewerSession) => {
    try {
      setPremiumLoading(true)
      await apiClient.post(`/api/v1/admin/viewers/${viewer.id}/premium`, {
        is_premium: !viewer.is_premium,
      })
      toastManager.add({
        title: `${viewer.username} premium ${viewer.is_premium ? 'revoked' : 'granted'}`,
        type: 'success',
      })
      setPremiumDialogViewer(null)
      fetchViewers()
    } catch (error) {
      console.error('Failed to update viewer premium:', error)
      toastManager.add({ title: 'Failed to update premium status', type: 'error' })
    } finally {
      setPremiumLoading(false)
    }
  }

  const bannedCount = viewers.filter((v) => v.is_banned).length
  const activeCount = viewers.filter((v) => !v.is_banned).length
  const premiumCount = viewers.filter((v) => v.is_premium).length

  return (
    <div className="mx-auto max-w-7xl px-4 py-8">
      <div className="mb-6 flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-text">Viewer Management</h1>
          <p className="mt-1 text-sm text-text-sub">Manage viewer sessions and bans</p>
        </div>
        <span className="text-sm text-text-sub">{viewers.length} total</span>
      </div>

      {/* Stats */}
      <div className="mb-6 grid grid-cols-4 gap-4">
        <Card className="p-4">
          <div className="text-xs text-text-sub">Total Viewers</div>
          <div className="text-2xl font-bold text-text">{viewers.length}</div>
        </Card>
        <Card className="p-4">
          <div className="text-xs text-text-sub">Premium</div>
          <div className="text-2xl font-bold text-amber-400">{premiumCount}</div>
        </Card>
        <Card className="p-4">
          <div className="text-xs text-text-sub">Banned</div>
          <div className="text-destructive text-2xl font-bold">{bannedCount}</div>
        </Card>
        <Card className="p-4">
          <div className="text-xs text-text-sub">Active</div>
          <div className="text-2xl font-bold text-kick">{activeCount}</div>
        </Card>
      </div>

      {/* Viewers Table */}
      {loading ? (
        <Card className="space-y-3 p-6">
          {Array.from({ length: 8 }).map((_, i) => (
            <Skeleton key={i} className="h-10 w-full rounded-lg" />
          ))}
        </Card>
      ) : (
        <Card className="overflow-hidden">
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead className="border-b border-border bg-surface-2">
                <tr>
                  <th className="px-4 py-3 text-left font-medium text-text-sub">Username</th>
                  <th className="px-4 py-3 text-left font-medium text-text-sub">Platform</th>
                  <th className="px-4 py-3 text-left font-medium text-text-sub">Last Message</th>
                  <th className="px-4 py-3 text-left font-medium text-text-sub">
                    Msg Count (1m/1h)
                  </th>
                  <th className="px-4 py-3 text-left font-medium text-text-sub">Premium</th>
                  <th className="px-4 py-3 text-left font-medium text-text-sub">Status</th>
                  <th className="px-4 py-3 text-left font-medium text-text-sub">Actions</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border">
                {viewers.length === 0 ? (
                  <tr>
                    <td colSpan={7} className="px-4 py-8 text-center text-text-dim">
                      No viewer sessions found
                    </td>
                  </tr>
                ) : (
                  viewers.map((viewer) => (
                    <tr key={viewer.id} className="transition-colors hover:bg-surface-2">
                      <td className="px-4 py-3">
                        <div className="text-sm font-medium text-text">{viewer.username}</div>
                        <div className="text-xs text-text-sub">{viewer.display_name}</div>
                      </td>
                      <td className="px-4 py-3">
                        <span className="text-sm text-text-sub capitalize">{viewer.platform}</span>
                      </td>
                      <td className="px-4 py-3 text-sm text-text-sub">
                        {viewer.last_message_at
                          ? formatDistanceToNow(new Date(viewer.last_message_at), {
                              addSuffix: true,
                            })
                          : 'Never'}
                      </td>
                      <td className="px-4 py-3 text-sm text-text-sub">
                        {viewer.message_count_1min}/{viewer.message_count_1hour}
                      </td>
                      <td className="px-4 py-3">
                        {viewer.viewer_id ? (
                          <Dialog.Root
                            open={premiumDialogViewer?.id === viewer.id}
                            onOpenChange={(open) => {
                              if (!open) setPremiumDialogViewer(null)
                            }}
                          >
                            <Dialog.Trigger
                              render={
                                <button
                                  className={`inline-flex items-center rounded px-2 py-0.5 text-xs font-medium transition-colors ${
                                    viewer.is_premium
                                      ? 'bg-amber-400/10 text-amber-400 hover:bg-amber-400/20'
                                      : 'bg-surface-2 text-text-dim hover:bg-surface-2/80'
                                  }`}
                                  onClick={() => setPremiumDialogViewer(viewer)}
                                >
                                  {viewer.is_premium ? 'Premium' : 'Free'}
                                </button>
                              }
                            />
                            <Dialog.Content showCloseButton={false}>
                              <Dialog.Title>
                                {viewer.is_premium ? 'Revoke' : 'Grant'} premium for &ldquo;
                                {viewer.username}&rdquo;?
                              </Dialog.Title>
                              <Dialog.Description>
                                {viewer.is_premium
                                  ? 'They will lose access to gradients, avatar frames, and flairs.'
                                  : 'They will be able to use gradients, avatar frames, and flairs.'}
                              </Dialog.Description>
                              <div className="mt-6 flex justify-end gap-3">
                                <Dialog.Close render={<Button variant="outline">Cancel</Button>} />
                                <Button
                                  variant="default"
                                  disabled={premiumLoading}
                                  onClick={() => handleTogglePremium(viewer)}
                                >
                                  {premiumLoading
                                    ? 'Updating...'
                                    : viewer.is_premium
                                      ? 'Revoke Premium'
                                      : 'Grant Premium'}
                                </Button>
                              </div>
                            </Dialog.Content>
                          </Dialog.Root>
                        ) : (
                          <span className="text-xs text-text-dim">—</span>
                        )}
                      </td>
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
                      <td className="px-4 py-3">
                        {viewer.is_banned ? (
                          <Dialog.Root
                            open={unbanDialogViewer?.id === viewer.id}
                            onOpenChange={(open) => {
                              if (!open) setUnbanDialogViewer(null)
                            }}
                          >
                            <Dialog.Trigger
                              render={
                                <Button
                                  variant="outline"
                                  size="sm"
                                  disabled={banningId === viewer.id}
                                  onClick={() => setUnbanDialogViewer(viewer)}
                                >
                                  {banningId === viewer.id ? 'Unbanning...' : 'Unban'}
                                </Button>
                              }
                            />
                            <Dialog.Content showCloseButton={false}>
                              <Dialog.Title>Unban &ldquo;{viewer.username}&rdquo;?</Dialog.Title>
                              <Dialog.Description>
                                This will restore their ability to send messages.
                              </Dialog.Description>
                              <div className="mt-6 flex justify-end gap-3">
                                <Dialog.Close render={<Button variant="outline">Cancel</Button>} />
                                <Button
                                  variant="default"
                                  disabled={banningId === viewer.id}
                                  onClick={() => handleUnban(viewer.id, viewer.username)}
                                >
                                  Unban Viewer
                                </Button>
                              </div>
                            </Dialog.Content>
                          </Dialog.Root>
                        ) : (
                          <Button
                            variant="destructive"
                            size="sm"
                            disabled={banningId === viewer.id}
                            onClick={() => handleBanClick(viewer)}
                          >
                            Ban
                          </Button>
                        )}
                      </td>
                    </tr>
                  ))
                )}
              </tbody>
            </table>
          </div>
        </Card>
      )}

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
            <label className="mb-2 block text-sm font-medium text-text-sub">
              Reason (optional)
            </label>
            <textarea
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
