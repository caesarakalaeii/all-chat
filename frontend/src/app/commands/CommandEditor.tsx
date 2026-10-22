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

import { useEffect, useId, useRef, useState } from 'react'
import { Button } from '@/components/ui/button'
import { Dialog } from '@/components/ui/dialog'
import { Field } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Switch } from '@/components/ui/switch'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { ApiError } from '@/lib/api/client'
import { engagementApi } from '@/lib/api/engagement'
import { useTranslations } from '@/lib/i18n'
import {
  CHAT_COMMAND_ROLES,
  type ChatCommand,
  type ChatCommandInput,
  type ChatCommandKind,
  type ChatCommandPreview,
  type ChatCommandRole,
} from '@/lib/types/engagement'

/** Tokens offered as one-click inserts, with the reference row describing each. */
const VARIABLES = [
  { token: '{user}', ref: 'user' },
  { token: '{touser}', ref: 'touser' },
  { token: '{query}', ref: 'query' },
  { token: '{1}', ref: 'args' },
  { token: '{count}', ref: 'count' },
  { token: '{random 1 6}', ref: 'random' },
  { token: '{pick heads | tails}', ref: 'pick' },
  { token: '{time Europe/Berlin}', ref: 'time' },
  { token: '{platform}', ref: 'where' },
  { token: '{clip}', ref: 'clip' },
  { token: '{urlfetch https://}', ref: 'urlfetch' },
] as const

const JS_EXAMPLE = '{js user.length > 10 ? "long name!" : "hi " + user}'
const URLFETCH_EXAMPLE = '{js JSON.parse({urlfetch https://api.example.com/data}).value}'

const PREVIEW_DEBOUNCE_MS = 400

export function CommandEditor({
  initial,
  editingId,
  onClose,
  onSaved,
}: {
  initial: ChatCommandInput
  /** Set when editing an existing command; absent when creating. */
  editingId?: string
  onClose: () => void
  onSaved: (saved: ChatCommand) => void
}) {
  const t = useTranslations()
  const ids = useId()
  const responseRef = useRef<HTMLTextAreaElement>(null)
  const [form, setForm] = useState<ChatCommandInput>(initial)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [sample, setSample] = useState('')
  const [preview, setPreview] = useState<ChatCommandPreview | null>(null)
  const [previewFailed, setPreviewFailed] = useState(false)
  const empty = form.response.trim() === '' && form.image_url.trim() === ''
  // An emptied form shows the placeholder, not the last render of the old text.
  const shown = empty ? null : preview

  const set = <K extends keyof ChatCommandInput>(key: K, value: ChatCommandInput[K]) =>
    setForm((f) => ({ ...f, [key]: value }))

  // Render the reply server-side (the same engine chat uses) a moment after typing stops.
  useEffect(() => {
    if (empty) return
    let stale = false
    const timer = setTimeout(() => {
      engagementApi
        .previewChatCommand({
          response: form.response,
          image: form.image_url,
          input: sample,
          min_role: form.min_role,
          create_clip: form.create_clip,
        })
        .then((p) => {
          if (stale) return
          setPreview(p)
          setPreviewFailed(false)
        })
        .catch(() => !stale && setPreviewFailed(true))
    }, PREVIEW_DEBOUNCE_MS)
    return () => {
      stale = true
      clearTimeout(timer)
    }
  }, [empty, form.response, form.image_url, form.min_role, form.create_clip, sample])

  function insert(token: string) {
    const el = responseRef.current
    const start = el?.selectionStart ?? form.response.length
    const end = el?.selectionEnd ?? form.response.length
    set('response', form.response.slice(0, start) + token + form.response.slice(end))
    requestAnimationFrame(() => {
      el?.focus()
      el?.setSelectionRange(start + token.length, start + token.length)
    })
  }

  async function save() {
    setSaving(true)
    setError(null)
    try {
      const saved = editingId
        ? await engagementApi.updateChatCommand(editingId, form)
        : await engagementApi.createChatCommand(form)
      onSaved(saved)
    } catch (err) {
      setError(
        err instanceof ApiError && err.message ? err.message : t('commands.editor.saveFailed')
      )
    } finally {
      setSaving(false)
    }
  }

  const triggerHint =
    form.kind === 'command'
      ? t('commands.editor.triggerHintCommand')
      : t('commands.editor.triggerHintKeyword')

  return (
    <Dialog.Root open onOpenChange={(open) => !open && onClose()}>
      <Dialog.Content className="max-h-[90vh] max-w-2xl overflow-y-auto">
        <Dialog.Title>
          {editingId
            ? t('commands.editor.editTitle', { trigger: initial.trigger })
            : t('commands.editor.createTitle')}
        </Dialog.Title>

        <form
          className="mt-4 space-y-4"
          onSubmit={(e) => {
            e.preventDefault()
            void save()
          }}
        >
          <div className="grid gap-4 sm:grid-cols-2">
            <div className="flex flex-col gap-1">
              <label htmlFor={`${ids}-kind`} className="text-sm font-medium text-text">
                {t('commands.editor.kindLabel')}
              </label>
              <Select
                items={[
                  { value: 'command', label: t('commands.editor.kindCommand') },
                  { value: 'keyword', label: t('commands.editor.kindKeyword') },
                ]}
                value={form.kind}
                onValueChange={(next) => next && set('kind', next as ChatCommandKind)}
              >
                <SelectTrigger id={`${ids}-kind`} className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="command">{t('commands.editor.kindCommand')}</SelectItem>
                  <SelectItem value="keyword">{t('commands.editor.kindKeyword')}</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="flex flex-col gap-1">
              <label htmlFor={`${ids}-trigger`} className="text-sm font-medium text-text">
                {t('commands.editor.triggerLabel')}
              </label>
              <Input
                id={`${ids}-trigger`}
                className="font-mono"
                value={form.trigger}
                maxLength={100}
                required
                aria-describedby={`${ids}-trigger-hint`}
                onChange={(e) => set('trigger', e.target.value)}
              />
              <p id={`${ids}-trigger-hint`} className="text-xs text-text-sub">
                {triggerHint}
              </p>
            </div>
          </div>

          <div className="flex flex-col gap-1">
            <label htmlFor={`${ids}-response`} className="text-sm font-medium text-text">
              {t('commands.editor.responseLabel')}
            </label>
            <Textarea
              id={`${ids}-response`}
              ref={responseRef}
              rows={3}
              className="font-mono"
              maxLength={2000}
              value={form.response}
              aria-describedby={`${ids}-response-hint`}
              onChange={(e) => set('response', e.target.value)}
            />
            <p id={`${ids}-response-hint`} className="text-xs text-text-sub">
              {t('commands.editor.responseHint')}
            </p>
            <div className="flex flex-wrap gap-1">
              {VARIABLES.map((v) => (
                <Button
                  key={v.token}
                  type="button"
                  size="xs"
                  variant="outline"
                  className="font-mono"
                  aria-label={t('commands.editor.insertVariable', { token: v.token })}
                  onClick={() => insert(v.token)}
                >
                  {v.token}
                </Button>
              ))}
            </div>
          </div>

          <div className="flex flex-col gap-1">
            <label htmlFor={`${ids}-image`} className="text-sm font-medium text-text">
              {t('commands.editor.imageLabel')}
            </label>
            <Input
              id={`${ids}-image`}
              className="font-mono"
              maxLength={1000}
              placeholder="https://"
              value={form.image_url}
              aria-describedby={`${ids}-image-hint`}
              onChange={(e) => set('image_url', e.target.value)}
            />
            <p id={`${ids}-image-hint`} className="text-xs text-text-sub">
              {t('commands.editor.imageHint')}
            </p>
          </div>

          <div className="grid gap-4 sm:grid-cols-2">
            <div className="flex flex-col gap-1">
              <label htmlFor={`${ids}-role`} className="text-sm font-medium text-text">
                {t('commands.editor.roleLabel')}
              </label>
              <Select
                items={CHAT_COMMAND_ROLES.map((r) => ({
                  value: r,
                  label: t(`commands.roles.${r}`),
                }))}
                value={form.min_role}
                onValueChange={(next) => next && set('min_role', next as ChatCommandRole)}
              >
                <SelectTrigger id={`${ids}-role`} className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {CHAT_COMMAND_ROLES.map((r) => (
                    <SelectItem key={r} value={r}>
                      {t(`commands.roles.${r}`)}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="flex flex-col gap-1">
              <label htmlFor={`${ids}-cooldown`} className="text-sm font-medium text-text">
                {t('commands.editor.cooldownLabel')}
              </label>
              <Input
                id={`${ids}-cooldown`}
                type="number"
                min={0}
                max={86400}
                value={form.cooldown_seconds}
                aria-describedby={`${ids}-cooldown-hint`}
                onChange={(e) => set('cooldown_seconds', Math.max(0, Number(e.target.value) || 0))}
              />
              <p id={`${ids}-cooldown-hint`} className="text-xs text-text-sub">
                {t('commands.editor.cooldownHint')}
              </p>
            </div>
          </div>

          <div className="flex flex-col gap-3 sm:flex-row sm:gap-8">
            <SwitchRow
              label={t('commands.editor.clipLabel')}
              hint={t('commands.editor.clipHint', { token: '{clip}' })}
              checked={form.create_clip}
              onChange={(v) => set('create_clip', v)}
            />
            <SwitchRow
              label={t('commands.editor.enabledLabel')}
              checked={form.enabled}
              onChange={(v) => set('enabled', v)}
            />
          </div>

          <section
            aria-labelledby={`${ids}-preview`}
            className="space-y-2 rounded-lg border border-border bg-surface-2 p-3"
          >
            <h3 id={`${ids}-preview`} className="text-sm font-semibold text-text">
              {t('commands.editor.previewHeading')}
            </h3>
            <div className="flex flex-col gap-1">
              <label htmlFor={`${ids}-sample`} className="text-xs text-text-sub">
                {t('commands.editor.previewInputLabel')}
              </label>
              <Input
                id={`${ids}-sample`}
                size="sm"
                className="font-mono"
                maxLength={500}
                placeholder={t('commands.editor.previewInputPlaceholder')}
                value={sample}
                onChange={(e) => setSample(e.target.value)}
              />
            </div>
            <output
              htmlFor={`${ids}-response ${ids}-sample`}
              aria-live="polite"
              className="block min-h-6 font-mono text-sm break-words text-text"
            >
              {previewFailed
                ? t('commands.editor.previewFailed')
                : shown?.output || (
                    <span className="text-text-dim">{t('commands.editor.previewEmpty')}</span>
                  )}
            </output>
            {shown?.image && (
              // eslint-disable-next-line @next/next/no-img-element -- any https host the streamer picks
              <img
                src={shown.image}
                alt={t('commands.editor.imagePreviewAlt')}
                className="max-h-32 max-w-full rounded-md border border-border"
              />
            )}
            {shown && shown.errors.length > 0 && (
              <ul className="space-y-1 text-xs text-destructive">
                {shown.errors.map((e, i) => (
                  <li key={i} className="font-mono">
                    {e}
                  </li>
                ))}
              </ul>
            )}
          </section>

          <details className="rounded-lg border border-border p-3 text-sm">
            <summary className="cursor-pointer font-semibold text-text">
              {t('commands.editor.referenceHeading')}
            </summary>
            <dl className="mt-3 grid gap-x-4 gap-y-2 sm:grid-cols-[auto_1fr]">
              {VARIABLES.map((v) => (
                <div key={v.token} className="contents">
                  <dt className="font-mono text-text">{v.token}</dt>
                  <dd className="text-text-sub">{t(`commands.reference.${v.ref}`)}</dd>
                </div>
              ))}
            </dl>
            <h4 className="mt-4 font-semibold text-text">{t('commands.editor.jsHeading')}</h4>
            <p className="mt-1 text-text-sub">{t('commands.editor.jsBody')}</p>
            <p className="mt-2 flex flex-wrap gap-2 text-text-sub">
              <span>{t('commands.editor.jsExample')}</span>
              <code className="font-mono break-all text-text">{JS_EXAMPLE}</code>
            </p>
            <h4 className="mt-4 font-semibold text-text">{t('commands.editor.urlfetchHeading')}</h4>
            <p className="mt-1 text-text-sub">
              {t('commands.editor.urlfetchBody', { token: '{urlfetch https://…}' })}
            </p>
            <p className="mt-2 flex flex-wrap gap-2 text-text-sub">
              <span>{t('commands.editor.jsExample')}</span>
              <code className="font-mono break-all text-text">{URLFETCH_EXAMPLE}</code>
            </p>
          </details>

          {error && (
            <p role="alert" className="text-sm text-destructive">
              {error}
            </p>
          )}

          <div className="flex justify-end gap-2">
            <Button type="button" variant="outline" onClick={onClose} disabled={saving}>
              {t('commands.editor.cancel')}
            </Button>
            <Button type="submit" disabled={saving}>
              {saving ? t('commands.editor.saving') : t('commands.editor.save')}
            </Button>
          </div>
        </form>
      </Dialog.Content>
    </Dialog.Root>
  )
}

function SwitchRow({
  label,
  hint,
  checked,
  onChange,
}: {
  label: string
  hint?: string
  checked: boolean
  onChange: (value: boolean) => void
}) {
  return (
    <Field.Root className="flex-1 flex-row items-start gap-3">
      <Switch.Root checked={checked} onCheckedChange={(v: boolean) => onChange(v)}>
        <Switch.Thumb />
      </Switch.Root>
      <div>
        <Field.Label>{label}</Field.Label>
        {hint && <Field.Description className="text-xs">{hint}</Field.Description>}
      </div>
    </Field.Root>
  )
}
