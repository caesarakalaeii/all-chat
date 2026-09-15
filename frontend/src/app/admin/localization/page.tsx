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
 * Admin Localization Review Page (ADR-0063)
 *
 * The review half of the beta-tester translation tool:
 *  - language requests: approve (visible to all contributors) or reject,
 *  - review queue: pending submissions with the English source read from the
 *    local catalog (never from the DB — the repo is the source of truth),
 *  - export: download approved translations as JSON, feed
 *    scripts/generate-locale-catalog.mjs to produce catalog files for a PR.
 *
 * Route: /admin/localization
 * Layout: inherits admin/layout.tsx (AdminNav, ToastProvider, ProtectedRoute)
 */

'use client'

import { useEffect, useState } from 'react'
import { useAuthStore } from '@/lib/stores/auth-store'
import { Card } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { Textarea } from '@/components/ui/textarea'
import {
  DialogRoot,
  DialogContent,
  DialogDescription,
  DialogTitle,
} from '@/components/ui/dialog'
import { toastManager } from '@/lib/toast'
import { useTranslations } from '@/lib/i18n'
import { enMessages } from '@/lib/i18n/messages/en'
import {
  localizationAdminApi,
  type AdminTranslation,
  type LocaleProgress,
  type LocaleRequest,
} from '@/lib/api/localization'

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

/** The submission under review, plus the note being drafted for a rejection. */
interface ReviewTarget {
  translation: AdminTranslation
}

export default function AdminLocalizationPage() {
  const { user } = useAuthStore()
  const t = useTranslations()

  const [requests, setRequests] = useState<LocaleRequest[] | null>(null)
  const [queue, setQueue] = useState<AdminTranslation[] | null>(null)
  const [progress, setProgress] = useState<LocaleProgress[] | null>(null)
  const [exporting, setExporting] = useState<string | null>(null)
  const [reviewTarget, setReviewTarget] = useState<ReviewTarget | null>(null)
  const [reviewNote, setReviewNote] = useState('')
  const [reviewing, setReviewing] = useState(false)

  useEffect(() => {
    if (!user?.is_admin) return
    Promise.all([
      localizationAdminApi.localeRequests(),
      localizationAdminApi.reviewQueue(),
      localizationAdminApi.progress(),
    ])
      .then(([req, q, p]) => {
        setRequests(req)
        setQueue(q)
        setProgress(p)
      })
      .catch(() => {
        setRequests([])
        setQueue([])
        setProgress([])
        toastManager.add({ title: t('admin.localization.loadError'), type: 'error' })
      })
  }, [user, t])

  function reviewLocale(code: string, approved: boolean) {
    localizationAdminApi
      .reviewLocaleRequest(code, approved)
      .then(() => {
        toastManager.add({
          title: approved
            ? t('admin.localization.localeApprovedToast', { code })
            : t('admin.localization.localeRejectedToast', { code }),
          type: 'success',
        })
        return localizationAdminApi.localeRequests()
      })
      .then(setRequests)
      .catch(() =>
        toastManager.add({ title: t('admin.localization.localeReviewFailedToast'), type: 'error' })
      )
  }

  function reviewSubmission(approved: boolean) {
    if (!reviewTarget) return
    setReviewing(true)
    localizationAdminApi
      .reviewSubmission(
        reviewTarget.translation.locale,
        reviewTarget.translation.key,
        approved,
        approved ? undefined : reviewNote
      )
      .then(() => {
        toastManager.add({
          title: approved
            ? t('admin.localization.submissionApprovedToast')
            : t('admin.localization.submissionRejectedToast'),
          type: 'success',
        })
        setReviewTarget(null)
        setReviewNote('')
        return Promise.all([
          localizationAdminApi.reviewQueue(),
          localizationAdminApi.progress(),
        ])
      })
      .then(([q, p]) => {
        setQueue(q)
        setProgress(p)
      })
      .catch(() =>
        toastManager.add({
          title: t('admin.localization.submissionReviewFailedToast'),
          type: 'error',
        })
      )
      .finally(() => setReviewing(false))
  }

  function downloadExport(code: string) {
    setExporting(code)
    localizationAdminApi
      .exportLocale(code)
      .then((data) => {
        const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' })
        const url = URL.createObjectURL(blob)
        const a = document.createElement('a')
        a.href = url
        a.download = `localization-export-${code}.json`
        a.click()
        URL.revokeObjectURL(url)
      })
      .catch(() =>
        toastManager.add({ title: t('admin.localization.exportFailedToast'), type: 'error' })
      )
      .finally(() => setExporting(null))
  }

  return (
    <div className="mx-auto max-w-3xl space-y-6 px-4 py-8">
      {/* Page header */}
      <div>
        <h1 className="text-2xl font-bold text-text">{t('admin.localization.heading')}</h1>
        <p className="mt-1 text-sm text-text-sub">{t('admin.localization.intro')}</p>
      </div>

      {/* Language requests */}
      <Card className="overflow-hidden">
        <div className="border-b border-border px-4 py-3">
          <h2 className="text-base font-bold text-text">
            {t('admin.localization.requestsHeading')}
          </h2>
        </div>
        {requests === null ? (
          <div className="space-y-2 p-4">
            <Skeleton className="h-10 w-full rounded-lg" />
          </div>
        ) : requests.length === 0 ? (
          <p className="px-4 py-3 text-sm text-text-sub">
            {t('admin.localization.requestsEmpty')}
          </p>
        ) : (
          <div className="divide-y divide-border">
            {requests.map((req) => (
              <div key={req.code} className="flex items-center gap-3 px-4 py-3">
                <div className="min-w-0 flex-1">
                  <span className="block text-sm font-medium text-text">
                    {req.native_name} ({req.english_name}, {req.code})
                  </span>
                </div>
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => reviewLocale(req.code, true)}
                >
                  {t('admin.localization.approveLocale', { code: req.code })}
                </Button>
                <Button size="sm" variant="ghost" onClick={() => reviewLocale(req.code, false)}>
                  {t('admin.localization.rejectLocale', { code: req.code })}
                </Button>
              </div>
            ))}
          </div>
        )}
      </Card>

      {/* Review queue */}
      <Card className="overflow-hidden">
        <div className="border-b border-border px-4 py-3">
          <h2 className="text-base font-bold text-text">
            {t('admin.localization.queueHeading')}
          </h2>
        </div>
        {queue === null ? (
          <div className="space-y-2 p-4">
            <Skeleton className="h-16 w-full rounded-lg" />
            <Skeleton className="h-16 w-full rounded-lg" />
          </div>
        ) : queue.length === 0 ? (
          <p className="px-4 py-3 text-sm text-text-sub">{t('admin.localization.queueEmpty')}</p>
        ) : (
          <div className="divide-y divide-border">
            {queue.map((tr) => (
              <div key={`${tr.locale}:${tr.key}`} className="px-4 py-3">
                <div className="mb-2 flex items-center gap-2">
                  <span className="rounded-full border border-violet-500/20 bg-violet-500/10 px-2 py-0.5 text-xs font-bold text-violet-400">
                    {tr.locale}
                  </span>
                  <span className="font-mono text-xs text-text-sub">{tr.key}</span>
                </div>
                <div className="mb-1 text-xs text-text-sub">
                  {t('admin.localization.sourceLabel')}
                </div>
                <p className="mb-2 text-sm text-text">{ALL_KEYS[tr.key] ?? '—'}</p>
                <div className="mb-1 text-xs text-text-sub">
                  {t('admin.localization.translationLabel', { locale: tr.locale })}
                </div>
                <p className="mb-3 text-sm text-text">{tr.value}</p>
                <div className="flex gap-2">
                  <Button size="sm" onClick={() => setReviewTarget({ translation: tr })}>
                    {t('admin.localization.approveButton')}
                  </Button>
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => {
                      setReviewTarget({ translation: tr })
                      setReviewNote('')
                    }}
                  >
                    {t('admin.localization.rejectButton')}
                  </Button>
                </div>
              </div>
            ))}
          </div>
        )}
      </Card>

      {/* Export + progress */}
      <Card className="overflow-hidden">
        <div className="border-b border-border px-4 py-3">
          <h2 className="text-base font-bold text-text">{t('admin.localization.exportHeading')}</h2>
        </div>
        <div className="space-y-4 p-4">
          <p className="text-sm text-text-sub">{t('admin.localization.exportBody')}</p>
          {progress === null ? (
            <Skeleton className="h-10 w-full rounded-lg" />
          ) : progress.length === 0 ? (
            <p className="text-sm text-text-sub">{t('admin.localization.requestsEmpty')}</p>
          ) : (
            <div className="space-y-2">
              {progress.map((p) => (
                <div
                  key={p.code}
                  className="flex min-h-[44px] flex-wrap items-center gap-3 rounded-lg border border-border bg-surface px-3 py-2"
                >
                  <span className="font-mono text-sm text-text">{p.code}</span>
                  <span className="text-xs text-text-sub">
                    {t('admin.localization.progressPending', { count: p.pending })}
                  </span>
                  <span className="text-xs text-text-sub">
                    {t('admin.localization.progressApproved', { count: p.approved })}
                  </span>
                  <Button
                    size="sm"
                    variant="outline"
                    className="ml-auto"
                    disabled={exporting === p.code || p.approved === 0}
                    onClick={() => downloadExport(p.code)}
                  >
                    {t('admin.localization.exportButton', { code: p.code })}
                  </Button>
                </div>
              ))}
            </div>
          )}
        </div>
      </Card>

      {/* Review dialog: approve or reject with a note. The note field is shown
          for both decisions — an approving admin may leave it empty. */}
      <DialogRoot open={!!reviewTarget} onOpenChange={() => setReviewTarget(null)}>
        <DialogContent>
          {reviewTarget && (
            <>
              <DialogTitle>{reviewTarget.translation.key}</DialogTitle>
              <DialogDescription>
                {t('admin.localization.translationLabel', {
                  locale: reviewTarget.translation.locale,
                })}
                : {reviewTarget.translation.value}
              </DialogDescription>
              <div className="mt-4">
                <label className="mb-1 block text-sm text-text-sub" htmlFor="review-note">
                  {t('admin.localization.reviewNoteLabel')}
                </label>
                <Textarea
                  id="review-note"
                  value={reviewNote}
                  placeholder={t('admin.localization.reviewNotePlaceholder')}
                  onChange={(e) => setReviewNote(e.target.value)}
                />
              </div>
              <div className="mt-6 flex justify-end gap-3">
                <Button variant="outline" disabled={reviewing} onClick={() => reviewSubmission(false)}>
                  {t('admin.localization.rejectButton')}
                </Button>
                <Button disabled={reviewing} onClick={() => reviewSubmission(true)}>
                  {t('admin.localization.approveButton')}
                </Button>
              </div>
            </>
          )}
        </DialogContent>
      </DialogRoot>
    </div>
  )
}
