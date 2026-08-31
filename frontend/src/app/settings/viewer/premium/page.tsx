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
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Dialog } from '@/components/ui/dialog'
import { Skeleton } from '@/components/ui/skeleton'
import { useTranslations, type TFunction } from '@/lib/i18n'
import { interpolateElements } from '@/lib/i18n/emphasise'
import { toastManager } from '@/lib/toast'
import { viewerApi } from '@/lib/api/viewer'
import { PATREON_JOIN_URL } from '@/lib/constants'
import type { PaymentStatus } from '@/lib/api/payment'

function statusLabel(t: TFunction, status?: string): string {
  switch (status) {
    case 'active':
      return t('common.patreon.statusActive')
    case 'declined':
      return t('common.patreon.statusDeclined')
    case 'former':
      return t('common.patreon.statusFormer')
    case 'expired':
      return t('settings.viewerPremium.statusExpired')
    default:
      return t('common.patreon.statusNotSubscribed')
  }
}

function ViewerPremiumContent() {
  const t = useTranslations()
  const router = useRouter()
  const searchParams = useSearchParams()

  const [status, setStatus] = useState<PaymentStatus | null>(null)
  const [loading, setLoading] = useState(true)
  const [connecting, setConnecting] = useState(false)

  const fetchStatus = useCallback(async () => {
    try {
      const data = await viewerApi.getPremiumStatus()
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
      router.replace('/settings/viewer/premium')
      void (async () => {
        await fetchStatus()
      })()
    } else if (result === 'error') {
      toastManager.add({
        title: 'Could not connect Patreon',
        description: 'Please try again.',
        type: 'error',
      })
      router.replace('/settings/viewer/premium')
    }
  }, [searchParams, router, fetchStatus])

  async function handleConnect() {
    setConnecting(true)
    try {
      await viewerApi.startPatreonConnect()
    } catch {
      setConnecting(false)
      toastManager.add({ title: 'Could not start Patreon connect', type: 'error' })
    }
  }

  async function handleDisconnect() {
    try {
      await viewerApi.disconnectPatreon()
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
            href="/settings/viewer"
            className="text-sm text-text-sub transition-colors hover:text-text"
          >
            {t('settings.viewerPremium.back')}
          </Link>
          <h1 className="text-2xl font-bold text-text">{t('settings.viewerPremium.heading')}</h1>
          <p className="text-sm text-text-sub">{t('settings.viewerPremium.subheading')}</p>
        </div>

        <Card className="p-6">
          <h2 className="mb-4 text-lg font-semibold text-text">{t('common.patreon.heading')}</h2>

          {loading ? (
            <Skeleton className="h-10 w-full" />
          ) : !status?.connected ? (
            <div className="space-y-3">
              <div className="flex items-center justify-between gap-4">
                <p className="text-sm text-text-sub">{t('settings.viewerPremium.connectPitch')}</p>
                <Button onClick={handleConnect} disabled={connecting}>
                  {connecting ? t('common.patreon.connecting') : t('common.patreon.connect')}
                </Button>
              </div>
              <p className="text-sm text-text-sub">
                {interpolateElements(t('settings.viewerPremium.notAPatronSuffix'), {
                  link: (
                    <a
                      href={PATREON_JOIN_URL}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="font-medium text-twitch hover:underline"
                    >
                      {t('common.patreon.subscribe')}
                    </a>
                  ),
                })}
              </p>
            </div>
          ) : (
            <div className="space-y-4">
              <div className="flex items-center justify-between">
                <span className="text-sm text-text-sub">
                  {t('settings.viewerPremium.premiumRow')}
                </span>
                <span className="font-medium text-text">
                  {isPremium ? t('common.patreon.active') : t('common.patreon.inactive')}
                </span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-sm text-text-sub">{t('common.patreon.subscriptionRow')}</span>
                <span className="font-medium text-text">{statusLabel(t, status.status)}</span>
              </div>
              {status.renews_at && (
                <div className="flex items-center justify-between">
                  <span className="text-sm text-text-sub">{t('common.patreon.renewsRow')}</span>
                  <span className="font-medium text-text">
                    {new Date(status.renews_at).toLocaleDateString()}
                  </span>
                </div>
              )}

              {!isPremium && (
                <p className="text-sm text-text-sub">{t('settings.viewerPremium.notGranting')}</p>
              )}

              <div className="flex justify-end pt-2">
                <Dialog.Root>
                  <Dialog.Trigger
                    render={<Button variant="destructive">{t('common.patreon.disconnect')}</Button>}
                  />
                  <Dialog.Content showCloseButton={false}>
                    <Dialog.Title>{t('common.patreon.disconnectTitle')}</Dialog.Title>
                    <Dialog.Description>
                      {t('settings.viewerPremium.disconnectBody')}
                    </Dialog.Description>
                    <div className="mt-6 flex justify-end gap-3">
                      <Dialog.Close
                        render={
                          <Button variant="outline">{t('common.patreon.disconnectCancel')}</Button>
                        }
                      />
                      <Button variant="destructive" onClick={handleDisconnect}>
                        {t('common.patreon.disconnectConfirm')}
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

function ViewerPremiumUnauthenticated() {
  const t = useTranslations()
  return (
    <div className="min-h-screen bg-bg">
      <AppNav />
      <main id="main-content" tabIndex={-1} className="mx-auto max-w-2xl space-y-6 px-4 py-12">
        <h1 className="text-2xl font-bold text-text">{t('settings.viewerPremium.heading')}</h1>
        <Card className="p-6">
          <h2 className="mb-2 text-lg font-semibold text-text">
            {t('settings.viewerPremium.signInHeading')}
          </h2>
          <p className="mb-6 text-sm text-text-sub">{t('settings.viewerPremium.signInBody')}</p>
          <Link
            href="/settings/viewer"
            className="text-sm font-medium text-text transition-colors hover:text-text-sub"
          >
            {t('settings.viewerPremium.signInLink')}
          </Link>
        </Card>
      </main>
    </div>
  )
}

export default function ViewerPremiumPage() {
  // Viewer auth gate (mirrors /settings/viewer): undefined = hydrating, false = no
  // viewer token, true = signed in. Reading localStorage in an effect avoids a
  // hydration mismatch.
  const [signedIn, setSignedIn] = useState<boolean | undefined>(undefined)

  useEffect(() => {
    void (async () => {
      if (typeof window === 'undefined') return
      setSignedIn(!!localStorage.getItem('viewer_jwt_token'))
    })()
  }, [])

  if (signedIn === undefined) {
    return (
      <div className="min-h-screen bg-bg">
        <AppNav />
      </div>
    )
  }

  if (!signedIn) {
    return <ViewerPremiumUnauthenticated />
  }

  return <ViewerPremiumContent />
}
