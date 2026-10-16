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
 * The alert profile editor of an alerts overlay (issue #954, ADR-0070/0071):
 * one profile per event type, loaded from GET /overlays/:id/alerts/profiles
 * (saved rows over the built-in defaults) and saved all-or-nothing with PUT.
 *
 * Tiering: the defaults (the built-in preset of every event type) and test
 * alerts are free; SAVING profiles is premium (the alerts_editor gate on the
 * PUT, migration 107 and ADR-0070). So without premium the form is shown
 * read-only with the standard upgrade link, and the custom-code section is
 * locked behind the same CTA.
 *
 * Custom code (premium): per-profile HTML/CSS/JS rendered by the alert box in
 * a scripts-only sandboxed iframe. The runtime the frame gets — onAlert(cb),
 * allchat.fill(template, event), data-alert-text / data-alert-image bindings —
 * and the postMessage contract { type: "alert", profile, event } are
 * documented in components/overlays/alertSandbox.ts. Textareas on purpose: no
 * editor dependency and no live network preview for v1; the test alert is
 * the preview.
 */

import { useCallback, useEffect, useId, useRef, useState } from 'react'

import { SliderControl } from '@/components/appearance/SliderControl'
import { ToggleSwitch } from '@/components/appearance/ToggleSwitch'
import { PremiumUpsellLink } from '@/components/PremiumUpsellLink'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import { ApiError } from '@/lib/api/client'
import {
  MEDIA_AUDIO_TYPES,
  MEDIA_IMAGE_TYPES,
  MediaUploadRejected,
  uploadMedia,
} from '@/lib/api/media'
import { overlaysApi } from '@/lib/api/overlays'
import { useTranslations, type TFunction } from '@/lib/i18n'
import { searchKlipyGifs, type KlipyGif } from '@/lib/klipy'
import { toastManager } from '@/lib/toast'
import { ALERT_EVENT_TYPES, type EditableAlertProfile } from '@/lib/types/alert'

/** Server limits (shared/alerts Profile.Validate), mirrored for inline errors. */
const MAX_TEMPLATE_LENGTH = 500
const MAX_CUSTOM_CODE_BYTES = 64 * 1024

const ALERT_PLACEHOLDERS = [
  { token: '{username}', key: 'placeholderUsername' },
  { token: '{amount}', key: 'placeholderAmount' },
  { token: '{count}', key: 'placeholderCount' },
  { token: '{message}', key: 'placeholderMessage' },
] as const

type ProfileField = 'text_template' | 'image_url' | 'sound_url' | 'duration_ms' | 'custom_code'
export type ProfileErrors = Partial<
  Record<ProfileField, 'errorTemplate' | 'errorUrl' | 'errorDuration' | 'errorCustomCode'>
>

const isHttpUrl = (raw: string) => {
  try {
    const u = new URL(raw)
    return (u.protocol === 'http:' || u.protocol === 'https:') && u.host !== ''
  } catch {
    return false
  }
}
const byteLength = (s: string | undefined) => new TextEncoder().encode(s ?? '').length

/** Client-side mirror of the server's checks; the server stays authoritative. */
export function validateAlertProfile(p: EditableAlertProfile): ProfileErrors {
  const errors: ProfileErrors = {}
  if (p.text_template.length > MAX_TEMPLATE_LENGTH) errors.text_template = 'errorTemplate'
  if (p.image_url && !isHttpUrl(p.image_url)) errors.image_url = 'errorUrl'
  if (p.sound_url && !isHttpUrl(p.sound_url)) errors.sound_url = 'errorUrl'
  if (!Number.isFinite(p.duration_ms) || p.duration_ms < 1 || p.duration_ms > 60000) {
    errors.duration_ms = 'errorDuration'
  }
  if (
    [p.custom_html, p.custom_css, p.custom_js].some((c) => byteLength(c) > MAX_CUSTOM_CODE_BYTES)
  ) {
    errors.custom_code = 'errorCustomCode'
  }
  return errors
}

export function AlertsEditor({ overlayId, isPremium }: { overlayId: string; isPremium: boolean }) {
  const t = useTranslations()
  const ids = useId()
  const [profiles, setProfiles] = useState<EditableAlertProfile[] | null>(null)
  const [loadFailed, setLoadFailed] = useState(false)
  const [eventType, setEventType] = useState<string>(ALERT_EVENT_TYPES[0])
  const [dirty, setDirty] = useState<Set<string>>(new Set())
  const [saving, setSaving] = useState(false)
  const [status, setStatus] = useState('')

  const load = useCallback(
    () => overlaysApi.getAlertProfiles(overlayId).then(setProfiles, () => setLoadFailed(true)),
    [overlayId]
  )

  useEffect(() => {
    void load()
  }, [load])

  if (loadFailed) {
    return (
      <div className="space-y-2">
        <p role="alert" className="text-sm text-destructive">
          {t('overlayEditor.alertsEditor.loadFailed')}
        </p>
        <Button
          type="button"
          variant="outline"
          size="sm"
          onClick={() => {
            setLoadFailed(false)
            void load()
          }}
        >
          {t('overlayEditor.alertsEditor.retry')}
        </Button>
      </div>
    )
  }
  if (!profiles) {
    return <p className="text-sm text-text-sub">{t('overlayEditor.alertsEditor.loading')}</p>
  }

  const profile = profiles.find((p) => p.event_type === eventType) ?? profiles[0]
  const errors = validateAlertProfile(profile)
  const update = (patch: Partial<EditableAlertProfile>) => {
    setProfiles((prev) =>
      prev!.map((p) => (p.event_type === profile.event_type ? { ...p, ...patch } : p))
    )
    setDirty((prev) => new Set(prev).add(profile.event_type))
    setStatus('')
  }

  async function save() {
    const changed = profiles!.filter((p) => dirty.has(p.event_type))
    const invalid = changed.find((p) => Object.keys(validateAlertProfile(p)).length > 0)
    if (invalid) {
      setEventType(invalid.event_type)
      setStatus(t('overlayEditor.alertsEditor.fixErrors'))
      return
    }
    setSaving(true)
    try {
      setProfiles(await overlaysApi.saveAlertProfiles(overlayId, changed))
      setDirty(new Set())
      setStatus(t('overlayEditor.alertsEditor.saved'))
      toastManager.add({ title: t('overlayEditor.alertsEditor.saved'), type: 'success' })
    } catch (err) {
      const reason = err instanceof ApiError || err instanceof Error ? err.message : String(err)
      const message = t('overlayEditor.alertsEditor.saveFailed', { reason })
      setStatus(message)
      toastManager.add({ title: message, type: 'error' })
    } finally {
      setSaving(false)
    }
  }

  const items = ALERT_EVENT_TYPES.map((value) => ({
    value,
    label: t(`overlayEditor.alertEventTypes.${value}`),
  }))
  const locked = !isPremium

  return (
    <div className="space-y-5">
      {locked && (
        <Card className="space-y-1 p-3">
          <p className="text-sm text-text-sub">{t('overlayEditor.alertsEditor.lockedBody')}</p>
          <PremiumUpsellLink className="text-sm" />
        </Card>
      )}

      <div className="flex flex-col gap-1">
        <label htmlFor={`${ids}-type`} className="text-xs text-text-sub">
          {t('overlayEditor.alertsEditor.eventTypeLabel')}
        </label>
        <Select
          items={items}
          value={profile.event_type}
          onValueChange={(value) => {
            if (value !== null) setEventType(value)
          }}
        >
          <SelectTrigger id={`${ids}-type`} className="w-full">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {items.map((item) => (
              <SelectItem key={item.value} value={item.value}>
                {item.label}
                {dirty.has(item.value) ? ' •' : ''}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      <fieldset disabled={locked} className="min-w-0 space-y-5">
        <ToggleSwitch
          label={t('overlayEditor.alertsEditor.enabledLabel')}
          checked={profile.enabled}
          onChange={(enabled) => update({ enabled })}
          disabled={locked}
        />

        <div className="flex flex-col gap-1">
          <label htmlFor={`${ids}-template`} className="text-xs text-text-sub">
            {t('overlayEditor.alertsEditor.templateLabel')}
          </label>
          <Input
            id={`${ids}-template`}
            value={profile.text_template}
            onChange={(e) => update({ text_template: e.target.value })}
            aria-invalid={errors.text_template ? true : undefined}
            aria-describedby={`${ids}-template-help${errors.text_template ? ` ${ids}-template-error` : ''}`}
          />
          <FieldError id={`${ids}-template-error`} error={errors.text_template} t={t} />
          <div id={`${ids}-template-help`} className="text-xs text-text-sub">
            <p>{t('overlayEditor.alertsEditor.templateHint')}</p>
            <ul className="mt-1 space-y-0.5">
              {ALERT_PLACEHOLDERS.map(({ token, key }) => (
                <li key={token}>
                  <code className="font-mono text-text">{token}</code>{' '}
                  {t(`overlayEditor.alertsEditor.${key}`)}
                </li>
              ))}
            </ul>
          </div>
        </div>

        <MediaField
          id={`${ids}-image`}
          kind="image"
          label={t('overlayEditor.alertsEditor.imageLabel')}
          value={profile.image_url}
          error={errors.image_url}
          onChange={(image_url) => update({ image_url })}
          locked={locked}
        />

        <MediaField
          id={`${ids}-sound`}
          kind="sound"
          label={t('overlayEditor.alertsEditor.soundLabel')}
          value={profile.sound_url}
          error={errors.sound_url}
          onChange={(sound_url) => update({ sound_url })}
          locked={locked}
        />

        <div className="flex flex-col gap-1">
          <label htmlFor={`${ids}-duration`} className="text-xs text-text-sub">
            {t('overlayEditor.alertsEditor.durationLabel')}
          </label>
          <Input
            id={`${ids}-duration`}
            type="number"
            min={1}
            max={60}
            step={0.5}
            value={profile.duration_ms / 1000}
            onChange={(e) => update({ duration_ms: Math.round(Number(e.target.value) * 1000) })}
            aria-invalid={errors.duration_ms ? true : undefined}
            aria-describedby={errors.duration_ms ? `${ids}-duration-error` : undefined}
            className="w-32"
          />
          <FieldError id={`${ids}-duration-error`} error={errors.duration_ms} t={t} />
        </div>

        <SliderControl
          label={t('overlayEditor.alertsEditor.volumeLabel')}
          value={profile.volume}
          min={0}
          max={100}
          step={1}
          unit="%"
          onChange={(volume) => update({ volume })}
          disabled={locked}
        />
      </fieldset>

      <section aria-labelledby={`${ids}-custom-heading`} className="space-y-3">
        <div>
          <h3 id={`${ids}-custom-heading`} className="text-sm font-semibold text-text">
            {t('overlayEditor.alertsEditor.customHeading')}
          </h3>
          <p className="text-xs text-text-sub">{t('overlayEditor.alertsEditor.customBody')}</p>
        </div>
        {locked ? (
          <Card className="space-y-1 p-3">
            <p className="text-sm text-text-sub">{t('overlayEditor.alertsEditor.customLocked')}</p>
            <PremiumUpsellLink className="text-sm" />
          </Card>
        ) : (
          <>
            {(
              [
                ['custom_html', 'customHtmlLabel'],
                ['custom_css', 'customCssLabel'],
                ['custom_js', 'customJsLabel'],
              ] as const
            ).map(([field, labelKey]) => (
              <div key={field} className="flex flex-col gap-1">
                <label htmlFor={`${ids}-${field}`} className="text-xs text-text-sub">
                  {t(`overlayEditor.alertsEditor.${labelKey}`)}
                </label>
                <Textarea
                  id={`${ids}-${field}`}
                  value={profile[field] ?? ''}
                  onChange={(e) => update({ [field]: e.target.value })}
                  spellCheck={false}
                  rows={field === 'custom_css' ? 4 : 6}
                  className="font-mono text-xs"
                  aria-invalid={errors.custom_code ? true : undefined}
                />
              </div>
            ))}
            <FieldError id={`${ids}-custom-error`} error={errors.custom_code} t={t} />
          </>
        )}
      </section>

      <div className="space-y-2">
        <Button
          type="button"
          className="w-full"
          onClick={() => void save()}
          disabled={locked || saving || dirty.size === 0}
        >
          {t(saving ? 'overlayEditor.alertsEditor.saving' : 'overlayEditor.alertsEditor.save')}
        </Button>
        {/* Always mounted so the save result announces (WCAG 4.1.3). */}
        <p role="status" className="text-center text-xs text-text-sub">
          {status || (dirty.size > 0 ? t('overlayEditor.alertsEditor.unsaved') : '')}
        </p>
      </div>
    </div>
  )
}

function FieldError({ id, error, t }: { id: string; error?: string; t: TFunction }) {
  if (!error) return null
  return (
    <p id={id} className="text-xs text-destructive">
      {t(`overlayEditor.alertsEditor.${error as 'errorUrl'}`)}
    </p>
  )
}

/** URL input + upload (+ KLIPY search for images) for one profile media field. */
function MediaField({
  id,
  kind,
  label,
  value,
  error,
  onChange,
  locked,
}: {
  id: string
  kind: 'image' | 'sound'
  label: string
  value: string
  error?: string
  onChange: (url: string) => void
  locked: boolean
}) {
  const t = useTranslations()
  const fileRef = useRef<HTMLInputElement>(null)
  const [progress, setProgress] = useState<number | null>(null)
  const [uploadError, setUploadError] = useState('')
  const accept = kind === 'image' ? MEDIA_IMAGE_TYPES : MEDIA_AUDIO_TYPES

  async function upload(file: File) {
    setUploadError('')
    setProgress(0)
    try {
      onChange(await uploadMedia(file, accept, setProgress))
    } catch (err) {
      const reason = err instanceof MediaUploadRejected ? err.reason : 'failed'
      setUploadError(
        t(
          reason === 'type'
            ? 'overlayEditor.alertsEditor.uploadType'
            : reason === 'size'
              ? 'overlayEditor.alertsEditor.uploadSize'
              : 'overlayEditor.alertsEditor.uploadFailed'
        )
      )
    } finally {
      setProgress(null)
      if (fileRef.current) fileRef.current.value = ''
    }
  }

  const describedBy = [error && `${id}-error`, uploadError && `${id}-upload-error`]
    .filter(Boolean)
    .join(' ')

  return (
    <div className="flex flex-col gap-1">
      <label htmlFor={id} className="text-xs text-text-sub">
        {label}
      </label>
      <div className="flex gap-2">
        <Input
          id={id}
          type="url"
          value={value}
          placeholder={t('overlayEditor.alertsEditor.urlPlaceholder')}
          onChange={(e) => onChange(e.target.value.trim())}
          aria-invalid={error ? true : undefined}
          aria-describedby={describedBy || undefined}
        />
        {value && (
          <Button type="button" variant="outline" onClick={() => onChange('')}>
            {t('overlayEditor.alertsEditor.clear')}
          </Button>
        )}
      </div>
      <FieldError id={`${id}-error`} error={error} t={t} />
      {kind === 'image' && value && !error && (
        // eslint-disable-next-line @next/next/no-img-element -- arbitrary streamer-chosen GIF hosts
        <img
          src={value}
          alt={t('overlayEditor.alertsEditor.imagePreviewAlt')}
          className="max-h-32 w-fit rounded-md object-contain"
        />
      )}
      <div className="flex flex-wrap items-center gap-2">
        <input
          ref={fileRef}
          type="file"
          accept={accept.join(',')}
          className="sr-only"
          tabIndex={-1}
          aria-hidden="true"
          onChange={(e) => {
            const file = e.target.files?.[0]
            if (file) void upload(file)
          }}
          disabled={locked}
        />
        <Button
          type="button"
          variant="outline"
          size="sm"
          onClick={() => fileRef.current?.click()}
          disabled={locked || progress !== null}
        >
          {progress !== null
            ? t('overlayEditor.alertsEditor.uploading', { percent: Math.round(progress * 100) })
            : t('overlayEditor.alertsEditor.upload')}
        </Button>
      </div>
      {uploadError && (
        <p id={`${id}-upload-error`} role="alert" className="text-xs text-destructive">
          {uploadError}
        </p>
      )}
      {kind === 'image' && !locked && <KlipySearch onPick={onChange} />}
    </div>
  )
}

/**
 * KLIPY GIF search ("Klippy" in issue #954): query → result grid → the picked
 * GIF's URL goes into the profile as returned (lib/klipy.ts has the usage
 * rules). Any failure, including no API key, degrades to "unavailable".
 */
function KlipySearch({ onPick }: { onPick: (url: string) => void }) {
  const t = useTranslations()
  const id = useId()
  const [query, setQuery] = useState('')
  const [state, setState] = useState<'idle' | 'searching' | 'unavailable' | 'done'>('idle')
  const [results, setResults] = useState<KlipyGif[]>([])

  async function search() {
    if (!query.trim()) return
    setState('searching')
    try {
      setResults(await searchKlipyGifs(query.trim()))
      setState('done')
    } catch {
      setResults([])
      setState('unavailable')
    }
  }

  return (
    <div className="mt-1 space-y-2">
      <form
        role="search"
        className="flex gap-2"
        onSubmit={(e) => {
          e.preventDefault()
          void search()
        }}
      >
        <label htmlFor={id} className="sr-only">
          {t('overlayEditor.alertsEditor.gifSearchLabel')}
        </label>
        <Input
          id={id}
          type="search"
          value={query}
          placeholder={t('overlayEditor.alertsEditor.gifSearchPlaceholder')}
          onChange={(e) => setQuery(e.target.value)}
        />
        <Button type="submit" variant="outline" disabled={state === 'searching' || !query.trim()}>
          {t(
            state === 'searching'
              ? 'overlayEditor.alertsEditor.gifSearching'
              : 'overlayEditor.alertsEditor.gifSearch'
          )}
        </Button>
      </form>
      {state === 'unavailable' && (
        <p role="status" className="text-xs text-text-sub">
          {t('overlayEditor.alertsEditor.gifSearchUnavailable')}
        </p>
      )}
      {state === 'done' && results.length === 0 && (
        <p role="status" className="text-xs text-text-sub">
          {t('overlayEditor.alertsEditor.gifSearchEmpty')}
        </p>
      )}
      {results.length > 0 && (
        <>
          <ul className="grid grid-cols-3 gap-2 @md:grid-cols-4">
            {results.map((gif) => (
              <li key={gif.id}>
                <button
                  type="button"
                  onClick={() => onPick(gif.url)}
                  aria-label={t('overlayEditor.alertsEditor.gifSelect', {
                    title: gif.title || t('overlayEditor.alertsEditor.gifUntitled'),
                  })}
                  className="block w-full overflow-hidden rounded-md border border-border hover:border-twitch focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
                >
                  {/* eslint-disable-next-line @next/next/no-img-element -- KLIPY media must load from the returned URL */}
                  <img
                    src={gif.previewUrl}
                    alt=""
                    loading="lazy"
                    className="aspect-square w-full object-cover"
                  />
                </button>
              </li>
            ))}
          </ul>
          <p className="text-right text-[11px] text-text-dim">
            {t('overlayEditor.alertsEditor.gifPoweredBy')}
          </p>
        </>
      )}
    </div>
  )
}
