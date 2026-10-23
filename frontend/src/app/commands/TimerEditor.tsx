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
 * The create/edit timer dialog (ADR-0081). Name and message are required; the
 * two triggers are optional individually but at least one must be set — both
 * set means fire every X minutes but only if at least Y messages arrived.
 */

import { useId, useState } from 'react'
import { Dialog } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Button } from '@/components/ui/button'
import { ApiError } from '@/lib/api/client'
import { engagementApi } from '@/lib/api/engagement'
import { useTranslations } from '@/lib/i18n'
import type { ChatTimer, ChatTimerInput } from '@/lib/types/engagement'

const MIN_INTERVAL = 5
const MAX_INTERVAL = 1440
const MIN_MESSAGES = 1
const MAX_MESSAGES = 10000

function coerce(input: string, lo: number, hi: number): number | null | undefined {
  if (input.trim() === '') return null
  const n = Number(input)
  if (!Number.isInteger(n) || n < lo || n > hi) return undefined
  return n
}

export function TimerEditor({
  initial,
  editingId,
  onClose,
  onSaved,
}: {
  initial: ChatTimerInput
  /** Set when editing an existing timer; absent when creating. */
  editingId?: string
  onClose: () => void
  onSaved: (saved: ChatTimer) => void
}) {
  const t = useTranslations()
  const ids = useId()
  const [form, setForm] = useState(initial)
  // '' empty (no trigger), an integer (trigger set), or undefined (invalid input).
  const [intervalValue, setIntervalValue] = useState(
    initial.interval_minutes === null ? '' : String(initial.interval_minutes)
  )
  const [messagesValue, setMessagesValue] = useState(
    initial.min_messages === null ? '' : String(initial.min_messages)
  )
  const [manualError, setManualError] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)

  function validate(): { body: ChatTimerInput; error: string | null } {
    const name = form.name.trim()
    const message = form.message.trim()
    if (name === '') return { body: form, error: t('commands.timers.nameRequired') }
    if (message === '') return { body: form, error: t('commands.timers.messageRequired') }
    const interval = coerce(intervalValue, MIN_INTERVAL, MAX_INTERVAL)
    if (interval === undefined) return { body: form, error: t('commands.timers.intervalRange') }
    const messages = coerce(messagesValue, MIN_MESSAGES, MAX_MESSAGES)
    if (messages === undefined) return { body: form, error: t('commands.timers.messagesRange') }
    if (interval === null && messages === null)
      return { body: form, error: t('commands.timers.triggerRequired') }
    return {
      body: { name, message, interval_minutes: interval, min_messages: messages },
      error: null,
    }
  }

  async function save() {
    const plan = validate()
    if (plan.error) {
      setManualError(plan.error)
      return
    }
    setSaving(true)
    setManualError(null)
    try {
      const saved = editingId
        ? await engagementApi.updateChatTimer(editingId, plan.body)
        : await engagementApi.createChatTimer(plan.body)
      onSaved(saved)
    } catch (err) {
      setManualError(
        err instanceof ApiError && err.message ? err.message : t('commands.timers.saveFailed')
      )
    } finally {
      setSaving(false)
    }
  }

  const error = manualError

  return (
    <Dialog.Root open onOpenChange={(open) => !open && onClose()}>
      <Dialog.Content className="max-h-[90vh] max-w-xl overflow-y-auto">
        <Dialog.Title>
          {editingId
            ? t('commands.timers.editTitle', { name: initial.name })
            : t('commands.timers.createTitle')}
        </Dialog.Title>
        <Dialog.Description>{t('commands.timers.dialogIntro')}</Dialog.Description>
        <form
          className="mt-4 space-y-4"
          onSubmit={(e) => {
            e.preventDefault()
            void save()
          }}
        >
          <div className="flex flex-col gap-1">
            <label htmlFor={`${ids}-name`} className="text-sm font-medium text-text">
              {t('commands.timers.nameLabel')}
            </label>
            <Input
              id={`${ids}-name`}
              value={form.name}
              maxLength={50}
              required
              onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
            />
          </div>

          <div className="flex flex-col gap-1">
            <label htmlFor={`${ids}-message`} className="text-sm font-medium text-text">
              {t('commands.timers.messageLabel')}
            </label>
            <Textarea
              id={`${ids}-message`}
              rows={3}
              className="font-mono"
              maxLength={2000}
              required
              value={form.message}
              onChange={(e) => setForm((f) => ({ ...f, message: e.target.value }))}
            />
            <p className="text-xs text-text-sub">{t('commands.timers.messageHint')}</p>
          </div>

          <div className="grid gap-4 sm:grid-cols-2">
            <div className="flex flex-col gap-1">
              <label htmlFor={`${ids}-interval`} className="text-sm font-medium text-text">
                {t('commands.timers.intervalLabel')}
              </label>
              <Input
                id={`${ids}-interval`}
                type="number"
                min={MIN_INTERVAL}
                max={MAX_INTERVAL}
                value={intervalValue}
                aria-describedby={`${ids}-interval-hint`}
                onChange={(e) => {
                  setManualError(null)
                  setIntervalValue(e.target.value)
                }}
              />
              <p id={`${ids}-interval-hint`} className="text-xs text-text-sub">
                {t('commands.timers.intervalHint', { min: MIN_INTERVAL, max: MAX_INTERVAL })}
              </p>
            </div>
            <div className="flex flex-col gap-1">
              <label htmlFor={`${ids}-messages`} className="text-sm font-medium text-text">
                {t('commands.timers.messagesLabel')}
              </label>
              <Input
                id={`${ids}-messages`}
                type="number"
                min={MIN_MESSAGES}
                max={MAX_MESSAGES}
                value={messagesValue}
                aria-describedby={`${ids}-messages-hint`}
                onChange={(e) => {
                  setManualError(null)
                  setMessagesValue(e.target.value)
                }}
              />
              <p id={`${ids}-messages-hint`} className="text-xs text-text-sub">
                {t('commands.timers.messagesHint', { min: MIN_MESSAGES, max: MAX_MESSAGES })}
              </p>
            </div>
          </div>

          <p className="text-xs text-text-sub">{t('commands.timers.combinedRule')}</p>

          {error && (
            <p role="alert" className="text-sm text-destructive">
              {error}
            </p>
          )}

          <div className="flex justify-end gap-2">
            <Button type="button" variant="outline" onClick={onClose} disabled={saving}>
              {t('commands.timers.cancel')}
            </Button>
            <Button type="submit" disabled={saving}>
              {saving ? t('commands.timers.saving') : t('commands.timers.save')}
            </Button>
          </div>
        </form>
      </Dialog.Content>
    </Dialog.Root>
  )
}
