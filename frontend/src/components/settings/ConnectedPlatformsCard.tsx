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
 * Connected platforms on /settings: every platform whose OAuth tokens the account
 * holds (GET /api/v1/auth/me/connections), each linked one with a confirmed
 * Disconnect that deletes its tokens. The sign-in platform is listed but cannot be
 * disconnected here; its tokens go with the account. Patreon and Discord have their
 * own disconnects, on /settings/premium and in the Discord card.
 */

'use client'

import { useEffect, useState } from 'react'
import { AlertDialog } from '@/components/ui/alert-dialog'
import { Skeleton } from '@/components/ui/skeleton'
import { VisuallyHidden } from '@/components/ui/visually-hidden'
import { authApi } from '@/lib/api/auth'
import { useTranslations } from '@/lib/i18n'
import { toastManager } from '@/lib/toast'
import type { ConnectablePlatform, PlatformConnection } from '@/lib/types/auth'

// What a disconnect does differs per platform; the copy mirrors
// DisconnectPlatform in services/auth-service/repository/platform_connections.go.
const EFFECT_KEYS = {
  twitch: 'settings.connections.effectTwitch',
  youtube: 'settings.connections.effectYoutube',
  kick: 'settings.connections.effectKick',
  facebook: 'settings.connections.effectFacebook',
  instagram: 'settings.connections.effectInstagram',
} as const satisfies Record<ConnectablePlatform, string>

export function ConnectedPlatformsCard() {
  const t = useTranslations()
  const [connections, setConnections] = useState<PlatformConnection[] | null>(null)
  const [loadFailed, setLoadFailed] = useState(false)
  const [target, setTarget] = useState<ConnectablePlatform | null>(null)
  const [disconnecting, setDisconnecting] = useState(false)

  useEffect(() => {
    let cancelled = false
    authApi
      .getConnections()
      .then((list) => {
        if (!cancelled) setConnections(list)
      })
      .catch(() => {
        if (!cancelled) setLoadFailed(true)
      })
    return () => {
      cancelled = true
    }
  }, [])

  const platformName = (platform: ConnectablePlatform) => t(`common.platforms.${platform}`)

  async function handleConfirm() {
    if (!target) return
    const platform = target
    setDisconnecting(true)
    try {
      await authApi.disconnectPlatform(platform)
      setConnections((previous) => previous?.filter((c) => c.platform !== platform) ?? null)
      toastManager.add({
        title: t('settings.connections.disconnectedToast', { platform: platformName(platform) }),
        type: 'success',
      })
      setTarget(null)
    } catch {
      toastManager.add({
        title: t('settings.connections.disconnectFailedToast', {
          platform: platformName(platform),
        }),
        type: 'error',
      })
    } finally {
      setDisconnecting(false)
    }
  }

  return (
    <div id="connections" className="lanes-panel scroll-mt-24 p-6">
      <h2 className="mb-4 text-lg">{t('settings.connections.heading')}</h2>
      <p className="text-sub mb-4 text-sm">{t('settings.connections.body')}</p>

      {loadFailed ? (
        <p role="alert" className="text-sm text-destructive">
          {t('settings.connections.loadFailed')}
        </p>
      ) : connections === null ? (
        <div role="status">
          <VisuallyHidden>{t('settings.connections.loading')}</VisuallyHidden>
          <Skeleton className="h-10 w-full rounded-none" />
        </div>
      ) : connections.length === 0 ? (
        <p className="text-sub text-sm">{t('settings.connections.empty')}</p>
      ) : (
        <ul className="space-y-4">
          {connections.map((connection) => (
            <li key={connection.platform} className="flex items-center justify-between gap-3">
              <div className="min-w-0 flex-1">
                <p className="flex items-center gap-2 font-medium">
                  {platformName(connection.platform)}
                  {connection.is_login && (
                    <span className="lanes-chip">{t('settings.connections.signInBadge')}</span>
                  )}
                </p>
                <p className="text-sub text-sm break-all">{connection.accounts.join(', ')}</p>
                {connection.is_login && (
                  <p className="text-sub mt-1 text-xs">
                    {t('settings.connections.signInNote', {
                      platform: platformName(connection.platform),
                    })}
                  </p>
                )}
              </div>
              {!connection.is_login && (
                <button
                  className="lanes-btn danger shrink-0"
                  aria-label={t('settings.connections.disconnectLabel', {
                    platform: platformName(connection.platform),
                  })}
                  onClick={() => setTarget(connection.platform)}
                >
                  {t('settings.connections.disconnect')}
                </button>
              )}
            </li>
          ))}
        </ul>
      )}

      <AlertDialog.Root
        open={target !== null}
        onOpenChange={(open: boolean) => {
          if (!open && !disconnecting) setTarget(null)
        }}
      >
        <AlertDialog.Content
          size="sm"
          className="rounded-none border-white bg-black text-[#f4f3ef]"
        >
          <AlertDialog.Title>
            {target
              ? t('settings.connections.disconnectTitle', { platform: platformName(target) })
              : ''}
          </AlertDialog.Title>
          <AlertDialog.Description>{target ? t(EFFECT_KEYS[target]) : ''}</AlertDialog.Description>
          <div className="mt-6 flex justify-end gap-2">
            <button
              className="lanes-btn ghost"
              disabled={disconnecting}
              onClick={() => setTarget(null)}
            >
              {t('settings.connections.disconnectCancel')}
            </button>
            <button
              className="lanes-btn danger"
              disabled={disconnecting}
              onClick={() => void handleConfirm()}
            >
              {disconnecting
                ? t('settings.connections.disconnecting')
                : t('settings.connections.disconnectConfirm')}
            </button>
          </div>
        </AlertDialog.Content>
      </AlertDialog.Root>
    </div>
  )
}
