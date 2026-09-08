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
import { Dialog } from '@/components/ui/dialog'
import { Skeleton } from '@/components/ui/skeleton'
import { type TFunction, formatDate, useTranslations } from '@/lib/i18n'
import { cn } from '@/lib/utils'
import { archivoBlack, spaceMono } from '@/lib/fonts'
import { interpolateElements } from '@/lib/i18n/emphasise'
import { toastManager } from '@/lib/toast'
import { PATREON_JOIN_URL } from '@/lib/constants'
import {
  disconnectPatreon,
  getPaymentStatus,
  startPatreonConnect,
  type PaymentStatus,
} from '@/lib/api/payment'

function statusLabel(t: TFunction, status?: string): string {
  switch (status) {
    case 'active':
      return t('common.patreon.statusActive')
    case 'declined':
      return t('common.patreon.statusDeclined')
    case 'former':
      return t('common.patreon.statusFormer')
    case 'expired':
      return t('settings.premium.statusExpired')
    default:
      return t('common.patreon.statusNotSubscribed')
  }
}

function PremiumContent() {
  const t = useTranslations()
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
      toastManager.add({ title: t('common.patreon.connectedToast'), type: 'success' })
      router.replace('/settings/premium')
      void (async () => {
        await fetchStatus()
      })()
    } else if (result === 'error') {
      toastManager.add({
        title: t('common.patreon.connectFailedToast'),
        description: t('common.toast.tryAgain'),
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
      toastManager.add({ title: t('common.patreon.connectStartFailedToast'), type: 'error' })
    }
  }

  async function handleDisconnect() {
    try {
      await disconnectPatreon()
      toastManager.add({ title: t('common.patreon.disconnectedToast'), type: 'success' })
      fetchStatus()
    } catch {
      toastManager.add({ title: t('common.patreon.disconnectFailedToast'), type: 'error' })
    }
  }

  const isPremium = status?.is_premium === true

  return (
    <div className={cn('lanes-app min-h-screen', archivoBlack.variable, spaceMono.variable)}>
      <AppNav />
      <main id="main-content" tabIndex={-1} className="mx-auto max-w-2xl space-y-6 px-4 py-12">
        <div className="space-y-1">
          <Link href="/settings" className="text-sub text-sm transition-colors hover:text-white">
            {t('settings.premium.back')}
          </Link>
          <h1 className="text-2xl">{t('settings.premium.heading')}</h1>
        </div>

        <div className="lanes-panel p-6">
          <h2 className="mb-4 text-lg">{t('common.patreon.heading')}</h2>

          {loading ? (
            <Skeleton className="h-10 w-full rounded-none" />
          ) : !status?.connected ? (
            <div className="space-y-3">
              <div className="flex items-center justify-between gap-4">
                <p className="text-sub text-sm">{t('settings.premium.connectPitch')}</p>
                <button className="lanes-btn" onClick={handleConnect} disabled={connecting}>
                  {connecting ? t('common.patreon.connecting') : t('common.patreon.connect')}
                </button>
              </div>
              <p className="text-sub text-sm">
                {interpolateElements(t('settings.premium.notAPatronSuffix'), {
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
                <span className="text-sub text-sm">{t('settings.premium.premiumRow')}</span>
                <span className="font-medium">
                  {isPremium ? t('common.patreon.active') : t('common.patreon.inactive')}
                </span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-sub text-sm">{t('common.patreon.subscriptionRow')}</span>
                <span className="font-medium">{statusLabel(t, status.status)}</span>
              </div>
              {status.renews_at && (
                <div className="flex items-center justify-between">
                  <span className="text-sub text-sm">{t('common.patreon.renewsRow')}</span>
                  <span className="font-medium">{formatDate(new Date(status.renews_at))}</span>
                </div>
              )}

              {!isPremium && (
                <p className="text-sub text-sm">{t('settings.premium.notGranting')}</p>
              )}

              <div className="flex justify-end pt-2">
                <Dialog.Root>
                  <Dialog.Trigger
                    render={
                      <button className="lanes-btn danger">{t('common.patreon.disconnect')}</button>
                    }
                  />
                  <Dialog.Content
                    showCloseButton={false}
                    className="rounded-none border-white bg-black text-[#f4f3ef]"
                  >
                    <Dialog.Title>{t('common.patreon.disconnectTitle')}</Dialog.Title>
                    <Dialog.Description>{t('settings.premium.disconnectBody')}</Dialog.Description>
                    <div className="mt-6 flex justify-end gap-3">
                      <Dialog.Close
                        render={
                          <button className="lanes-btn ghost">
                            {t('common.patreon.disconnectCancel')}
                          </button>
                        }
                      />
                      <button className="lanes-btn danger" onClick={handleDisconnect}>
                        {t('common.patreon.disconnectConfirm')}
                      </button>
                    </div>
                  </Dialog.Content>
                </Dialog.Root>
              </div>
            </div>
          )}
        </div>
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
