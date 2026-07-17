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

import Link from 'next/link'
import { useRouter, useSearchParams } from 'next/navigation'
import { useCallback, useEffect, useState } from 'react'
import { AppNav } from '@/components/AppNav'
import { ProtectedRoute } from '@/components/ProtectedRoute'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Dialog } from '@/components/ui/dialog'
import { Skeleton } from '@/components/ui/skeleton'
import { toastManager } from '@/lib/toast'
import { PATREON_JOIN_URL } from '@/lib/constants'
import {
  disconnectPatreon,
  getPaymentStatus,
  startPatreonConnect,
  type PaymentStatus,
} from '@/lib/api/payment'

function statusLabel(status?: string): string {
  switch (status) {
    case 'active':
      return 'Active'
    case 'declined':
      return 'Payment declined'
    case 'former':
      return 'Ended'
    case 'expired':
      return 'Below premium tier'
    default:
      return 'Not subscribed'
  }
}

function PremiumContent() {
  const router = useRouter()
  const searchParams = useSearchParams()

  const [status, setStatus] = useState<PaymentStatus | null>(null)
  const [loading, setLoading] = useState(true)
  const [connecting, setConnecting] = useState(false)

  const fetchStatus = useCallback(async () => {
    try {
      const data = await getPaymentStatus()
      setStatus(data)
    } catch {
      setStatus({ connected: false })
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void (async () => {
      await fetchStatus()
    })()
  }, [fetchStatus])

  useEffect(() => {
    const result = searchParams.get('patreon')
    if (result === 'connected') {
      toastManager.add({ title: 'Patreon connected!', type: 'success' })
      router.replace('/settings/premium')
      void (async () => {
        await fetchStatus()
      })()
    } else if (result === 'error') {
      toastManager.add({
        title: 'Could not connect Patreon',
        description: 'Please try again.',
        type: 'error',
      })
      router.replace('/settings/premium')
    }
  }, [searchParams, router, fetchStatus])

  async function handleConnect() {
    setConnecting(true)
    try {
      await startPatreonConnect()
    } catch {
      setConnecting(false)
      toastManager.add({ title: 'Could not start Patreon connect', type: 'error' })
    }
  }

  async function handleDisconnect() {
    try {
      await disconnectPatreon()
      toastManager.add({ title: 'Patreon disconnected', type: 'success' })
      fetchStatus()
    } catch {
      toastManager.add({ title: 'Failed to disconnect', type: 'error' })
    }
  }

  const isPremium = status?.is_premium === true

  return (
    <div className="min-h-screen bg-bg">
      <AppNav />
      <main id="main-content" tabIndex={-1} className="mx-auto max-w-2xl space-y-6 px-4 py-12">
        <div className="space-y-1">
          <Link
            href="/settings"
            className="text-sm text-text-sub transition-colors hover:text-text"
          >
            ← Back to Settings
          </Link>
          <h1 className="text-2xl font-bold text-text">Premium</h1>
        </div>

        <Card className="p-6">
          <h2 className="mb-4 text-lg font-semibold text-text">Patreon</h2>

          {loading ? (
            <Skeleton className="h-10 w-full" />
          ) : !status?.connected ? (
            <div className="space-y-3">
              <div className="flex items-center justify-between gap-4">
                <p className="text-sm text-text-sub">
                  Back All-Chat on Patreon to unlock premium features automatically.
                </p>
                <Button onClick={handleConnect} disabled={connecting}>
                  {connecting ? 'Redirecting…' : 'Connect Patreon'}
                </Button>
              </div>
              <p className="text-sm text-text-sub">
                Not a patron yet?{' '}
                <a
                  href={PATREON_JOIN_URL}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="font-medium text-twitch hover:underline"
                >
                  Subscribe on Patreon
                </a>
                , then connect.
              </p>
            </div>
          ) : (
            <div className="space-y-4">
              <div className="flex items-center justify-between">
                <span className="text-sm text-text-sub">Premium</span>
                <span className="font-medium text-text">{isPremium ? 'Active' : 'Inactive'}</span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-sm text-text-sub">Subscription</span>
                <span className="font-medium text-text">{statusLabel(status.status)}</span>
              </div>
              {status.renews_at && (
                <div className="flex items-center justify-between">
                  <span className="text-sm text-text-sub">Renews</span>
                  <span className="font-medium text-text">
                    {new Date(status.renews_at).toLocaleDateString()}
                  </span>
                </div>
              )}

              {!isPremium && (
                <p className="text-sm text-text-sub">
                  Your Patreon is linked but not granting premium. Make sure your pledge is active
                  and at or above the premium tier.
                </p>
              )}

              <div className="flex justify-end pt-2">
                <Dialog.Root>
                  <Dialog.Trigger
                    render={<Button variant="destructive">Disconnect Patreon</Button>}
                  />
                  <Dialog.Content showCloseButton={false}>
                    <Dialog.Title>Disconnect Patreon?</Dialog.Title>
                    <Dialog.Description>
                      This unlinks your Patreon account. Premium granted by your subscription will
                      be removed.
                    </Dialog.Description>
                    <div className="mt-6 flex justify-end gap-3">
                      <Dialog.Close render={<Button variant="outline">Cancel</Button>} />
                      <Button variant="destructive" onClick={handleDisconnect}>
                        Yes, disconnect
                      </Button>
                    </div>
                  </Dialog.Content>
                </Dialog.Root>
              </div>
            </div>
          )}
        </Card>
      </main>
    </div>
  )
}

export default function PremiumSettingsPage() {
  return (
    <ProtectedRoute>
      <PremiumContent />
    </ProtectedRoute>
  )
}
