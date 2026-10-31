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
 * Admin Chat Bots Page
 *
 * Connects and disconnects the shared All-Chat Bot account on Twitch and
 * YouTube (ADR-0081). Kick needs no page entry: the bot there is the one
 * configured on the Kick app.
 *
 * Route: /admin/chat-bots
 * Layout: inherits admin/layout.tsx (AdminSidebar, ProtectedRoute)
 */

'use client'

import { useCallback, useEffect, useState } from 'react'
import { useRouter } from 'next/navigation'
import { useAuthStore } from '@/lib/stores/auth-store'
import { authApi } from '@/lib/api/auth'
import { startChatBotConnect } from '@/lib/api/engagement'
import { AlertDialog } from '@/components/ui/alert-dialog'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { useTranslations } from '@/lib/i18n'
import { toastManager } from '@/lib/toast'
import type { ChatBotPlatform, ChatBotStatus } from '@/lib/types/engagement'

// Kick has no admin-managed row: the bot lives on the Kick app itself.
const PLATFORMS = ['twitch', 'youtube'] as const satisfies readonly ChatBotPlatform[]

export default function AdminChatBotsPage() {
  const router = useRouter()
  const t = useTranslations()
  const { user } = useAuthStore()
  const [status, setStatus] = useState<ChatBotStatus | null>(null)
  const [loading, setLoading] = useState(true)
  const [loadFailed, setLoadFailed] = useState(false)
  const [connecting, setConnecting] = useState<ChatBotPlatform | null>(null)
  const [disconnecting, setDisconnecting] = useState<ChatBotPlatform | null>(null)
  const [removing, setRemoving] = useState<ChatBotPlatform | null>(null)

  const fetchStatus = useCallback(async () => {
    try {
      const data = await authApi.getChatBotsStatus()
      setStatus(data)
      setLoadFailed(false)
    } catch {
      setLoadFailed(true)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    if (!user?.is_admin) {
      router.push('/admin')
      return
    }
    void (async () => {
      await fetchStatus()
    })()
  }, [user, router, fetchStatus])

  async function connect(platform: ChatBotPlatform) {
    setConnecting(platform)
    try {
      await startChatBotConnect(platform, 'allchat')
    } catch {
      setConnecting(null)
      toastManager.add({
        title: t('admin.chatBots.connectFailed', {
          platform: t(`common.platforms.${platform}`),
        }),
        type: 'error',
      })
    }
  }

  async function confirmDisconnect() {
    if (!removing) return
    const platform = removing
    setDisconnecting(platform)
    setRemoving(null)
    try {
      await authApi.disconnectChatBot(platform, 'allchat')
      toastManager.add({ title: t('admin.chatBots.disconnected'), type: 'success' })
      await fetchStatus()
    } catch {
      toastManager.add({
        title: t('admin.chatBots.disconnectFailed', {
          platform: t(`common.platforms.${platform}`),
        }),
        type: 'error',
      })
    } finally {
      setDisconnecting(null)
    }
  }

  return (
    <div className="mx-auto max-w-3xl px-4 py-8">
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-text">{t('admin.chatBots.heading')}</h1>
        <p className="mt-1 text-sm text-text-sub">{t('admin.chatBots.intro')}</p>
      </div>

      {loading ? (
        <Card className="space-y-3 p-6">
          {PLATFORMS.map((platform) => (
            <Skeleton key={platform} className="h-12 w-full rounded-lg" />
          ))}
        </Card>
      ) : loadFailed ? (
        <Card className="p-6">
          <p className="text-sm text-text-sub">{t('admin.chatBots.loadError')}</p>
        </Card>
      ) : status ? (
        <Card className="space-y-4 p-4">
          <h2 className="text-sm font-semibold text-text">{t('admin.chatBots.allchatHeading')}</h2>
          <ul className="divide-y divide-border">
            {PLATFORMS.map((platform) => {
              const slot = status.allchat[platform]
              const name = slot.connected ? slot.display_name : null
              return (
                <li key={platform} className="flex items-center gap-3 py-3">
                  <span className="min-w-0 flex-1 truncate text-sm text-text">
                    {t(`common.platforms.${platform}`)}
                  </span>
                  {name ? (
                    <Badge size="sm">{t('admin.chatBots.connected', { name })}</Badge>
                  ) : (
                    <Badge size="sm">{t('admin.chatBots.notConnected')}</Badge>
                  )}
                  {name ? (
                    <Button
                      size="sm"
                      variant="outline"
                      disabled={disconnecting !== null}
                      onClick={() => setRemoving(platform)}
                    >
                      {disconnecting === platform
                        ? t('admin.chatBots.disconnecting')
                        : t('admin.chatBots.disconnect')}
                    </Button>
                  ) : (
                    <Button
                      size="sm"
                      variant="outline"
                      disabled={connecting !== null}
                      onClick={() => void connect(platform)}
                    >
                      {connecting === platform
                        ? t('admin.chatBots.connecting', {
                            platform: t(`common.platforms.${platform}`),
                          })
                        : t('admin.chatBots.connect', {
                            platform: t(`common.platforms.${platform}`),
                          })}
                    </Button>
                  )}
                </li>
              )
            })}
          </ul>
          <p className="text-xs text-text-dim">{t('admin.chatBots.kickNote')}</p>
        </Card>
      ) : null}

      <AlertDialog.Root
        open={removing !== null}
        onOpenChange={(open) => !open && setRemoving(null)}
      >
        <AlertDialog.Content>
          <AlertDialog.Title>
            {t('admin.chatBots.removeTitle', {
              platform: t(`common.platforms.${removing ?? 'twitch'}`),
            })}
          </AlertDialog.Title>
          <AlertDialog.Description>{t('admin.chatBots.removeBody')}</AlertDialog.Description>
          <div className="mt-6 flex justify-end gap-2">
            <AlertDialog.Close render={<Button variant="outline" />}>
              {t('admin.chatBots.removeCancel')}
            </AlertDialog.Close>
            <Button variant="destructive" onClick={() => void confirmDisconnect()}>
              {t('admin.chatBots.removeConfirm')}
            </Button>
          </div>
        </AlertDialog.Content>
      </AlertDialog.Root>
    </div>
  )
}
