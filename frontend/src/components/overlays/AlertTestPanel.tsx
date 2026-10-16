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

'use client'

/**
 * "Send test alert" on an alerts overlay's editor (ADR-0070): publishes a
 * synthetic alert through POST /overlays/:id/alerts/test, which reaches the
 * overlay over the same WebSocket path as a real alert. Free, unlike saving
 * profiles.
 */

import { useId, useState } from 'react'

import { Button } from '@/components/ui/button'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { overlaysApi } from '@/lib/api/overlays'
import { useTranslations } from '@/lib/i18n'
import { ALERT_EVENT_TYPES, type AlertEventType } from '@/lib/types/alert'
import { toastManager } from '@/lib/toast'

export function AlertTestPanel({ overlayId }: { overlayId: string }) {
  const t = useTranslations()
  const selectId = useId()
  const [eventType, setEventType] = useState<AlertEventType>('subscription')
  const [sending, setSending] = useState(false)

  const items = ALERT_EVENT_TYPES.map((value) => ({
    value,
    label: t(`overlayEditor.alertEventTypes.${value}`),
  }))

  async function send() {
    setSending(true)
    try {
      await overlaysApi.sendTestAlert(overlayId, eventType)
      toastManager.add({ title: t('overlayEditor.alertTest.sent'), type: 'success' })
    } catch {
      toastManager.add({ title: t('overlayEditor.alertTest.failed'), type: 'error' })
    } finally {
      setSending(false)
    }
  }

  return (
    <section className="space-y-3" aria-labelledby={`${selectId}-heading`}>
      <div>
        <h3 id={`${selectId}-heading`} className="text-sm font-semibold text-text">
          {t('overlayEditor.alertTest.heading')}
        </h3>
        <p className="text-xs text-text-sub">{t('overlayEditor.alertTest.body')}</p>
      </div>
      <div className="flex flex-col gap-1">
        <label htmlFor={selectId} className="text-xs text-text-sub">
          {t('overlayEditor.alertTest.eventTypeLabel')}
        </label>
        <Select
          items={items}
          value={eventType}
          onValueChange={(value) => {
            if (value !== null) setEventType(value as AlertEventType)
          }}
        >
          <SelectTrigger id={selectId} className="w-full">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {items.map((item) => (
              <SelectItem key={item.value} value={item.value}>
                {item.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
      <Button type="button" onClick={() => void send()} disabled={sending}>
        {t(sending ? 'overlayEditor.alertTest.sending' : 'overlayEditor.alertTest.send')}
      </Button>
    </section>
  )
}
