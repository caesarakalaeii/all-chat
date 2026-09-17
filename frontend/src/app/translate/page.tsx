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
 * The localization contributor tool (ADR-0063): /translate.
 *
 * Guided translator view for users without technical experience:
 *   1. pick a language (or request a new one — open, request-based),
 *   2. pick an area of the app (namespace), progress shown per area,
 *   3. translate string by string: English source on top, one input below,
 *      placeholder hints inline, previously-submitted status visible.
 *
 * The key list comes from the local English catalog (enMessages), not the
 * backend: the catalog is the source of truth, and a key the repo no longer
 * has must not appear in the tool. Submissions batch per area on explicit
 * submit; the backend re-checks placeholder parity against source_value.
 *
 * Beta-tester gated: <ProtectedRoute requireBetaTester> here for UX, and
 * RequireEarlyAccess('localization_contribution') on the service for real.
 */

import { useEffect, useMemo, useState } from 'react'
import { AppNav } from '@/components/AppNav'
import { MaintenanceBanner } from '@/components/MaintenanceBanner'
import { ProtectedRoute } from '@/components/ProtectedRoute'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Dialog } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { toastManager } from '@/lib/toast'
import { useTranslations } from '@/lib/i18n'
import { enMessages } from '@/lib/i18n/messages/en'
import {
  localizationApi,
  type LocalizationLocale,
  type LocalizationTranslation,
  type SubmitRow,
} from '@/lib/api/localization'

// The catalog is the key source of truth; the backend never sends keys.

type MessageCatalog = { readonly [key: string]: string | MessageCatalog }

/** Flat key -> English source string, dotted paths as keys. */
function flatten(catalog: MessageCatalog, prefix = ''): Record<string, string> {
  const out: Record<string, string> = {}
  for (const [k, v] of Object.entries(catalog)) {
    const path = prefix ? `${prefix}.${k}` : k
    if (typeof v === 'string') out[path] = v
    else Object.assign(out, flatten(v, path))
  }
  return out
}

const ALL_KEYS = flatten(enMessages)

const PLACEHOLDER = /\{(\w+)\}/g

/** The translator's own namespace is not a translation surface. */
const TOOL_NAMESPACES = [
  'a11y',
  'admin',
  'auth',
  'common',
  'dashboard',
  'docs',
  'errors',
  'legal',
  'maintenanceBanner',
  'marketing',
  'metadata',
  'moderation',
  'onboarding',
  'overlayEditor',
  'settings',
  'viewerOverlay',
] as const


function TranslatePageInner() {
  const t = useTranslations()

  const [locales, setLocales] = useState<LocalizationLocale[] | null>(null)
  const [localeCode, setLocaleCode] = useState('')
  const [namespaceId, setNamespaceId] = useState('')
  const [mine, setMine] = useState<Record<string, LocalizationTranslation>>({})
  const [drafts, setDrafts] = useState<Record<string, string>>({})
  const [saving, setSaving] = useState(false)
  const [requestOpen, setRequestOpen] = useState(false)
  const [reqCode, setReqCode] = useState('')
  const [reqEnglishName, setReqEnglishName] = useState('')
  const [reqNativeName, setReqNativeName] = useState('')
  const [requesting, setRequesting] = useState(false)

  const locale = locales?.find((l) => l.code === localeCode) ?? null
  const namespace = TOOL_NAMESPACES.find((n) => n === namespaceId)
  const nsKeys = useMemo(
    () => (namespace ? Object.keys(flatten(enMessages[namespace], namespace)) : []),
    [namespace]
  )

  // Load the locale list once. A failure here includes the early-access gate
  // refusing — the page is already beta-gated, so "no locales" is the honest
  // fallback rather than an error card.
  useEffect(() => {
    localizationApi
      .listLocales()
      .then(setLocales)
      .catch(() => setLocales([]))
  }, [])

  // Load the caller's own submissions when a locale is picked.
  useEffect(() => {
    if (!localeCode) {
      setMine({})
      return
    }
    let cancelled = false
    localizationApi
      .myTranslations(localeCode)
      .then((rows) => {
        if (cancelled) return
        const byKey: Record<string, LocalizationTranslation> = {}
        for (const row of rows) byKey[row.key] = row
        setMine(byKey)
      })
      .catch(() => {
        if (!cancelled) setMine({})
      })
    return () => {
      cancelled = true
    }
  }, [localeCode])

  // Seed drafts from the caller's previous submissions so revising is not
  // retyping. Approved rows are excluded: they are already accepted, and
  // re-editing one would reset it to pending for a cosmetic change.
  useEffect(() => {
    const seeded: Record<string, string> = {}
    for (const row of Object.values(mine)) {
      if (row.status !== 'approved') seeded[row.key] = row.value
    }
    setDrafts(seeded)
  }, [mine])

  const nsProgress = useMemo(() => {
    const done = nsKeys.filter((k) => mine[k]?.status === 'approved').length
    return { done, total: nsKeys.length }
  }, [nsKeys, mine])

  const unsaved = nsKeys.filter((k) => (drafts[k] ?? '').trim() !== (mine[k]?.value ?? ''))

  function submitBatch() {
    if (!localeCode || unsaved.length === 0) return
    setSaving(true)
    const rows: SubmitRow[] = unsaved.map((key) => ({
      key,
      value: drafts[key].trim(),
      source_value: ALL_KEYS[key],
    }))
    localizationApi
      .submit(localeCode, rows)
      .then((result) => {
        for (const failure of result.failed) {
          toastManager.add({
            title: t('translate.rowError', { error: failure.error }),
            type: 'error',
          })
        }
        if (result.accepted > 0) {
          toastManager.add({
            title: t('translate.savedToast', { count: result.accepted }),
            type: 'success',
          })
        }
        return localizationApi.myTranslations(localeCode)
      })
      .then((rows) => {
        const byKey: Record<string, LocalizationTranslation> = {}
        for (const row of rows) byKey[row.key] = row
        setMine(byKey)
      })
      .catch(() => toastManager.add({ title: t('translate.saveFailedToast'), type: 'error' }))
      .finally(() => setSaving(false))
  }

  function requestLocale() {
    setRequesting(true)
    localizationApi
      .requestLocale({
        code: reqCode.trim(),
        english_name: reqEnglishName.trim(),
        native_name: reqNativeName.trim(),
      })
      .then(() => {
        toastManager.add({ title: t('translate.localeRequestedToast'), type: 'success' })
        setRequestOpen(false)
        setReqCode('')
        setReqEnglishName('')
        setReqNativeName('')
      })
      .catch(() =>
        toastManager.add({ title: t('translate.localeRequestFailedToast'), type: 'error' })
      )
      .finally(() => setRequesting(false))
  }

  return (
    <div className="min-h-screen bg-bg">
      <AppNav />
      <MaintenanceBanner />
      <main className="mx-auto max-w-3xl px-4 py-8">
        <h1 className="mb-2 text-2xl font-bold text-text">{t('translate.heading')}</h1>
        <p className="mb-6 text-text-sub">{t('translate.intro')}</p>

        {/* Step 1: language */}
        {locales === null ? (
          <Skeleton className="h-10 w-full" />
        ) : (
          <div className="mb-6 flex flex-wrap items-end gap-2">
            <div className="min-w-48">
              <Label htmlFor="translate-locale">{t('translate.localeLabel')}</Label>
              <Select
                value={localeCode}
                onValueChange={(v) => {
                  setLocaleCode(v ?? '')
                  setNamespaceId('')
                }}
              >
                <SelectTrigger id="translate-locale" className="w-full">
                  <SelectValue placeholder={t('translate.localeLabel')} />
                </SelectTrigger>
                <SelectContent>
                  {locales.length === 0 && (
                    <div className="px-2 py-1.5 text-sm text-text-sub">
                      {t('translate.localeNone')}
                    </div>
                  )}
                  {locales.map((l) => (
                    <SelectItem key={l.code} value={l.code}>
                      {l.native_name} ({l.english_name})
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <Button variant="outline" onClick={() => setRequestOpen(true)}>
              {t('translate.localeRequestButton')}
            </Button>
          </div>
        )}

        {/* Step 2: area */}
        {locale && (
          <>
            <p className="mb-3 text-sm text-text-sub">{t('translate.namespaceIntro')}</p>
            <div className="mb-6 grid grid-cols-2 gap-2 sm:grid-cols-3">
              {TOOL_NAMESPACES.map((ns) => {
                const keys = Object.keys(flatten(enMessages[ns]))
                const done = keys.filter((k) => mine[k]?.status === 'approved').length
                return (
                  <button
                    key={ns}
                    type="button"
                    onClick={() => setNamespaceId(ns)}
                    className={
                      'rounded-lg border border-border bg-surface p-3 text-left transition-colors hover:bg-surface-2 focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none' +
                      (namespaceId === ns ? ' border-twitch' : '')
                    }
                  >
                    <span className="block text-sm font-medium text-text">{ns}</span>
                    <span className="block text-xs text-text-sub">
                      {t('translate.namespaceProgress', { done, total: keys.length })}
                    </span>
                  </button>
                )
              })}
            </div>
          </>
        )}

        {/* Step 3: string by string */}
        {locale && namespace && (
          <Card className="p-6">
            <div className="mb-4 flex items-center justify-between">
              <h2 className="text-lg font-semibold text-text">{namespace}</h2>
              <span className="text-sm text-text-sub">
                {t('translate.namespaceProgress', {
                  done: nsProgress.done,
                  total: nsProgress.total,
                })}
              </span>
            </div>
            <div className="space-y-6">
              {nsKeys.map((key) => {
                const source = ALL_KEYS[key]
                const placeholders = [...source.matchAll(PLACEHOLDER)].map((m) => m[1])
                const draft = drafts[key] ?? ''
                const mineRow = mine[key]
                const missing = placeholders.filter((p) => !draft.includes(`{${p}}`))
                return (
                  <div key={key} className="rounded-lg border border-border bg-surface p-4">
                    <p className="mb-1 text-xs text-text-dim">
                      {t('translate.stringLabel', { key })}
                    </p>
                    <p className="mb-3 text-base text-text">{source}</p>

                    {mineRow && (
                      <p
                        className={
                          'mb-3 text-xs ' +
                          (mineRow.status === 'approved'
                            ? 'text-emerald-500'
                            : mineRow.status === 'rejected'
                              ? 'text-destructive'
                              : 'text-text-sub')
                        }
                      >
                        {mineRow.status === 'approved'
                          ? t('translate.statusApproved')
                          : mineRow.status === 'rejected'
                            ? t('translate.statusRejected')
                            : t('translate.statusPending')}
                        {mineRow.status === 'rejected' && mineRow.review_note && (
                          <> — {t('translate.rejectedNote', { note: mineRow.review_note })}</>
                        )}
                      </p>
                    )}

                    <Label htmlFor={`tr-${key}`} className="mb-1 block">
                      {t('translate.yourTranslationLabel')}
                    </Label>
                    <Input
                      id={`tr-${key}`}
                      value={mineRow?.status === 'approved' ? mineRow.value : draft}
                      disabled={mineRow?.status === 'approved'}
                      onChange={(e) => setDrafts((d) => ({ ...d, [key]: e.target.value }))}
                    />
                    {placeholders.length > 0 && (
                      <p className="mt-2 text-xs text-text-sub">
                        {t('translate.placeholderHint', {
                          placeholder: `{${placeholders[0]}}`,
                        })}
                      </p>
                    )}
                    {draft.trim() !== '' && missing.length > 0 && (
                      <p className="mt-2 text-xs text-destructive">
                        {t('translate.placeholderMismatch', { placeholder: `{${missing[0]}}` })}
                      </p>
                    )}
                  </div>
                )
              })}

              <Button
                onClick={submitBatch}
                disabled={saving || unsaved.length === 0}
                className="w-full"
              >
                {t('translate.saveButton', { count: unsaved.length })}
              </Button>
            </div>
          </Card>
        )}
      </main>

      {/* Request-a-language dialog */}
      <Dialog.Root open={requestOpen} onOpenChange={(open) => setRequestOpen(open)}>
        <Dialog.Content>
          <Dialog.Title>{t('translate.localeRequestTitle')}</Dialog.Title>
          <Dialog.Description>{t('translate.localeRequestBody')}</Dialog.Description>
          <div className="mt-4 space-y-3">
            <div>
              <Label htmlFor="req-code">{t('translate.localeCodeLabel')}</Label>
              <Input
                id="req-code"
                value={reqCode}
                placeholder={t('translate.localeCodePlaceholder')}
                onChange={(e) => setReqCode(e.target.value)}
              />
            </div>
            <div>
              <Label htmlFor="req-en">{t('translate.localeEnglishNameLabel')}</Label>
              <Input
                id="req-en"
                value={reqEnglishName}
                onChange={(e) => setReqEnglishName(e.target.value)}
              />
            </div>
            <div>
              <Label htmlFor="req-native">{t('translate.localeNativeNameLabel')}</Label>
              <Input
                id="req-native"
                value={reqNativeName}
                onChange={(e) => setReqNativeName(e.target.value)}
              />
            </div>
          </div>
          <div className="mt-6 flex justify-end gap-3">
            <Dialog.Close
              render={<Button variant="ghost">{t('common.betaWarning.cancelButton')}</Button>}
            />
            <Button
              onClick={requestLocale}
              disabled={
                requesting || !reqCode.trim() || !reqEnglishName.trim() || !reqNativeName.trim()
              }
            >
              {t('translate.localeRequestSubmit')}
            </Button>
          </div>
        </Dialog.Content>
      </Dialog.Root>
    </div>
  )
}

export default function TranslatePage() {
  return (
    <ProtectedRoute requireBetaTester>
      <TranslatePageInner />
    </ProtectedRoute>
  )
}
