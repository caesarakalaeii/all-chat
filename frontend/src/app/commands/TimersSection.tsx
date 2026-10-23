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
 * The timers section of /commands (ADR-0081): repeating chat messages fired by
 * interval minutes, a minimum number of chat messages, or both at once.
 */

import { useCallback, useState } from 'react'
import { Plus } from 'lucide-react'
import { AlertDialog } from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Switch } from '@/components/ui/switch'
import { engagementApi } from '@/lib/api/engagement'
import { useTranslations, type TFunction } from '@/lib/i18n'
import { toastManager } from '@/lib/toast'
import type { ChatTimer, ChatTimerInput } from '@/lib/types/engagement'
import { TimerEditor } from './TimerEditor'

const MAX_TIMERS = 20

export function TimersSection({
  timers,
  setTimers,
  onEdit,
}: {
  timers: ChatTimer[]
  setTimers: (timers: ChatTimer[]) => void
  onEdit: (initial: ChatTimerInput, id?: string) => void
}) {
  const t = useTranslations()
  const [deleting, setDeleting] = useState<ChatTimer | null>(null)

  const toggle = useCallback(
    async (timer: ChatTimer, enabled: boolean) => {
      try {
        const saved = await engagementApi.updateChatTimer(timer.id, {
          name: timer.name,
          message: timer.message,
          interval_minutes: timer.interval_minutes,
          min_messages: timer.min_messages,
        })
        setTimers(timers.map((x) => (x.id === saved.id ? { ...saved, enabled } : x)))
      } catch {
        toastManager.add({ title: t('commands.timers.toggleFailed'), type: 'error' })
      }
    },
    [t, timers, setTimers]
  )

  async function confirmDelete() {
    if (!deleting) return
    try {
      await engagementApi.deleteChatTimer(deleting.id)
      setTimers(timers.filter((x) => x.id !== deleting.id))
    } catch {
      toastManager.add({ title: t('commands.timers.deleteFailed'), type: 'error' })
    } finally {
      setDeleting(null)
    }
  }

  return (
    <Card className="space-y-3 p-4">
      <div className="flex items-center justify-between gap-4">
        <h2 className="text-sm font-semibold text-text">{t('commands.timers.heading')}</h2>
        <Button
          size="sm"
          variant="outline"
          disabled={timers.length >= MAX_TIMERS}
          onClick={() =>
            onEdit({ name: '', message: '', interval_minutes: null, min_messages: null })
          }
        >
          <Plus aria-hidden="true" />
          {t('commands.timers.new')}
        </Button>
      </div>
      <p className="text-xs text-text-sub">{t('commands.timers.intro')}</p>

      {timers.length === 0 ? (
        <p className="text-sm text-text-sub">{t('commands.timers.empty')}</p>
      ) : (
        <ul className="divide-y divide-border">
          {timers.map((timer) => (
            <li key={timer.id} className="flex flex-col gap-2 py-3 sm:flex-row sm:items-center">
              <div className="min-w-0 flex-1 space-y-1">
                <div className="flex flex-wrap items-center gap-2">
                  <span className="font-medium text-text">{timer.name}</span>
                  <span className="text-xs text-text-dim">
                    {triggerSummary(t, timer.interval_minutes, timer.min_messages)}
                  </span>
                </div>
                <p className="truncate font-mono text-xs text-text-sub">{timer.message}</p>
              </div>
              <div className="flex items-center gap-2">
                <Switch.Root
                  checked={timer.enabled}
                  aria-label={t('commands.timers.enabledLabel', { name: timer.name })}
                  onCheckedChange={(value: boolean) => void toggle(timer, value)}
                >
                  <Switch.Thumb />
                </Switch.Root>
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() =>
                    onEdit(
                      {
                        name: timer.name,
                        message: timer.message,
                        interval_minutes: timer.interval_minutes,
                        min_messages: timer.min_messages,
                      },
                      timer.id
                    )
                  }
                >
                  {t('commands.timers.edit')}
                </Button>
                <Button
                  size="sm"
                  variant="ghost"
                  aria-label={t('commands.timers.deleteLabel', { name: timer.name })}
                  onClick={() => setDeleting(timer)}
                >
                  {t('commands.timers.delete')}
                </Button>
              </div>
            </li>
          ))}
        </ul>
      )}

      <AlertDialog.Root
        open={deleting !== null}
        onOpenChange={(open) => !open && setDeleting(null)}
      >
        <AlertDialog.Content>
          <AlertDialog.Title>
            {t('commands.timers.deleteTitle', { name: deleting?.name ?? '' })}
          </AlertDialog.Title>
          <AlertDialog.Description>{t('commands.timers.deleteBody')}</AlertDialog.Description>
          <div className="mt-6 flex justify-end gap-2">
            <AlertDialog.Close render={<Button variant="outline" />}>
              {t('commands.timers.deleteCancel')}
            </AlertDialog.Close>
            <Button variant="destructive" onClick={() => void confirmDelete()}>
              {t('commands.timers.deleteConfirm')}
            </Button>
          </div>
        </AlertDialog.Content>
      </AlertDialog.Root>
    </Card>
  )
}

/** Summarizes what fires the timer, for the list row. */
export function triggerSummary(
  t: TFunction,
  intervalMinutes: number | null,
  minMessages: number | null
): string {
  if (intervalMinutes !== null && minMessages !== null) {
    return t('commands.timers.triggerBoth', { minutes: intervalMinutes, messages: minMessages })
  }
  if (intervalMinutes !== null) {
    return t('commands.timers.triggerEvery', { minutes: intervalMinutes })
  }
  if (minMessages !== null) {
    return t('commands.timers.triggerMessages', { messages: minMessages })
  }
  return ''
}
