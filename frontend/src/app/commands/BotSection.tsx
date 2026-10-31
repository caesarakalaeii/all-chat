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

/**
 * The bot section of /commands (ADR-0081): whose account answers — the All-Chat
 * Bot (default) or the streamer's own — plus the Twitch channel:bot grant the
 * bot send needs, and the premium custom bot accounts per platform.
 */

import { useState } from 'react'
import Link from 'next/link'
import { Check, CircleAlert, Plus } from 'lucide-react'
import { AlertDialog } from '@/components/ui/alert-dialog'
import { Button, buttonVariants } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Switch } from '@/components/ui/switch'
import { engagementApi, startChatBotConnect } from '@/lib/api/engagement'
import { authApi } from '@/lib/api/auth'
import { useTranslations } from '@/lib/i18n'
import { toastManager } from '@/lib/toast'
import { cn } from '@/lib/utils'
import type {
  ChatBotIdentity,
  ChatBotPlatform,
  ChatBotSender,
  ChatBotStatus,
} from '@/lib/types/engagement'

const CUSTOM_PLATFORMS: ChatBotPlatform[] = ['twitch', 'kick', 'youtube']

export function BotSection({
  status,
  sender,
  consentOverlayId,
  onSenderChanged,
}: {
  status: ChatBotStatus
  sender: ChatBotSender
  /** The overlay whose Twitch source the re-consent attaches to; null when there is none. */
  consentOverlayId: string | null
  onSenderChanged: (sender: ChatBotSender) => void
}) {
  const t = useTranslations()
  const [savingSender, setSavingSender] = useState(false)
  const [granting, setGranting] = useState(false)
  const [connecting, setConnecting] = useState<ChatBotPlatform | null>(null)
  const [disconnecting, setDisconnecting] = useState<ChatBotPlatform | null>(null)
  const [removing, setRemoving] = useState<ChatBotPlatform | null>(null)

  async function setSender(next: ChatBotSender) {
    setSavingSender(true)
    try {
      const saved = await engagementApi.updateChatBotSettings({ sender: next })
      onSenderChanged(saved.sender)
    } catch {
      toastManager.add({ title: t('commands.bot.senderSaveFailed'), type: 'error' })
    } finally {
      setSavingSender(false)
    }
  }

  async function grant() {
    if (!consentOverlayId) return
    setGranting(true)
    try {
      window.location.href = await authApi.getChatBotGrantUrl(consentOverlayId)
    } catch {
      setGranting(false)
      toastManager.add({ title: t('commands.bot.grantFailed'), type: 'error' })
    }
  }

  async function connect(platform: ChatBotPlatform) {
    setConnecting(platform)
    try {
      await startChatBotConnect(platform, 'custom')
    } catch {
      setConnecting(null)
      toastManager.add({ title: t('commands.bot.connectFailed'), type: 'error' })
    }
  }

  async function remove(platform: ChatBotPlatform) {
    setDisconnecting(platform)
    setRemoving(null)
    try {
      await authApi.disconnectChatBot(platform, 'custom')
      toastManager.add({ title: t('commands.bot.disconnected'), type: 'success' })
      window.location.reload()
    } catch {
      toastManager.add({ title: t('commands.bot.disconnectFailed'), type: 'error' })
    } finally {
      setDisconnecting(null)
    }
  }

  return (
    <Card className="space-y-4 p-4">
      <h2 className="text-sm font-semibold text-text">{t('commands.bot.heading')}</h2>

      <div className="flex items-center justify-between gap-4">
        <p className="min-w-0 text-sm">
          <span className="font-medium text-text">
            {sender === 'bot' ? t('commands.bot.senderBot') : t('commands.bot.senderSelf')}
          </span>
          <span className="block text-xs text-text-sub">
            {sender === 'bot' ? t('commands.bot.senderBotHint') : t('commands.bot.senderSelfHint')}
          </span>
        </p>
        <Switch.Root
          checked={sender === 'bot'}
          aria-label={t('commands.bot.senderToggleLabel')}
          onCheckedChange={(value: boolean) => {
            if (!savingSender) void setSender(value ? 'bot' : 'self')
          }}
        >
          <Switch.Thumb />
        </Switch.Root>
      </div>

      {status.twitch_bot_granted ? (
        <p className="flex items-center gap-2 text-xs text-text-sub">
          <Check className="size-4 text-success" aria-hidden="true" />
          {t('commands.bot.twitchGranted')}
        </p>
      ) : (
        consentOverlayId && (
          <div className="flex items-center gap-2 text-xs">
            <CircleAlert className="size-4 text-warning" aria-hidden="true" />
            <span className="min-w-0 flex-1 text-text-sub">
              {t('commands.bot.twitchNotGranted')}
            </span>
            <Button size="sm" variant="outline" disabled={granting} onClick={() => void grant()}>
              {granting ? t('commands.bot.granting') : t('commands.bot.grant')}
            </Button>
          </div>
        )
      )}

      <div className="space-y-2">
        <h3 className="text-sm font-medium text-text">{t('commands.bot.customHeading')}</h3>
        <p className="text-xs text-text-sub">{t('commands.bot.customIntro')}</p>
        {status.premium ? (
          <ul className="divide-y divide-border">
            {CUSTOM_PLATFORMS.map((platform) => {
              const bot: ChatBotIdentity | null = status.custom[platform]
              return (
                <li key={platform} className="flex items-center gap-3 py-2">
                  <span className="min-w-0 flex-1">
                    <span className="block text-sm text-text">
                      {t(`common.platforms.${platform}`)}
                    </span>
                    <span className="block truncate text-xs text-text-sub">
                      {bot
                        ? t('commands.bot.connectedAs', {
                            name: bot.display_name,
                            login: bot.login,
                          })
                        : t('commands.bot.noneConnected')}
                    </span>
                  </span>
                  {bot ? (
                    <>
                      <Button
                        size="sm"
                        variant="ghost"
                        disabled={disconnecting !== null}
                        aria-label={t('commands.bot.disconnectLabel', {
                          platform: t(`common.platforms.${platform}`),
                        })}
                        onClick={() => setRemoving(platform)}
                      >
                        {disconnecting === platform
                          ? t('commands.bot.disconnecting')
                          : t('commands.bot.disconnect')}
                      </Button>
                    </>
                  ) : (
                    <Button
                      size="sm"
                      variant="outline"
                      disabled={connecting !== null}
                      onClick={() => void connect(platform)}
                    >
                      <Plus aria-hidden="true" />
                      {connecting === platform
                        ? t('commands.bot.connecting')
                        : t('commands.bot.connect', {
                            platform: t(`common.platforms.${platform}`),
                          })}
                    </Button>
                  )}
                </li>
              )
            })}
          </ul>
        ) : (
          <div className="space-y-2 rounded-lg border border-border p-3">
            <p className="text-xs text-text-sub">{t('commands.bot.customPremium')}</p>
            <Link href="/upgrade" className={buttonVariants({ size: 'sm', variant: 'link' })}>
              {t('commands.status.upgrade')}
            </Link>
          </div>
        )}
      </div>

      <AlertDialog.Root
        open={removing !== null}
        onOpenChange={(open) => !open && setRemoving(null)}
      >
        <AlertDialog.Content>
          <AlertDialog.Title>
            {t('commands.bot.removeTitle', {
              platform: t(`common.platforms.${removing ?? 'twitch'}`),
            })}
          </AlertDialog.Title>
          <AlertDialog.Description>{t('commands.bot.removeBody')}</AlertDialog.Description>
          <div className="mt-6 flex justify-end gap-2">
            <AlertDialog.Close render={<Button variant="outline" />}>
              {t('commands.bot.removeCancel')}
            </AlertDialog.Close>
            <Button
              variant="destructive"
              onClick={() => {
                if (removing) void remove(removing)
              }}
            >
              {t('commands.bot.removeConfirm')}
            </Button>
          </div>
        </AlertDialog.Content>
      </AlertDialog.Root>
    </Card>
  )
}
