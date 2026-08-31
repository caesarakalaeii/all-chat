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

import { useEffect, useState, useCallback } from 'react'
import { useRouter } from 'next/navigation'
import { Trash2, Plus, Wrench } from 'lucide-react'
import { useAuthStore } from '@/lib/stores/auth-store'
import { maintenanceApi } from '@/lib/api/maintenance'
import { Card } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { DialogRoot, DialogContent, DialogTitle, DialogDescription } from '@/components/ui/dialog'
import { toastManager } from '@/lib/toast'
import { formatDateTime, useTranslations } from '@/lib/i18n'
import { cn } from '@/lib/utils'
import type { MaintenanceWindow, CreateMaintenanceRequest } from '@/lib/types/maintenance'

// Not a module-level Intl.DateTimeFormat: that formats with whatever locale the
// machine has. formatDateTime pins the UI locale and caches the constructor
// itself, keyed by locale and options, so calling it per render costs nothing.
const DATE_FORMAT_OPTIONS: Intl.DateTimeFormatOptions = {
  year: 'numeric',
  month: 'short',
  day: 'numeric',
  hour: '2-digit',
  minute: '2-digit',
}

function formatRange(startsAt: string, endsAt: string): string {
  const from = formatDateTime(new Date(startsAt), DATE_FORMAT_OPTIONS)
  const to = formatDateTime(new Date(endsAt), DATE_FORMAT_OPTIONS)
  return `${from} – ${to}`
}

function isActive(mw: MaintenanceWindow): boolean {
  const now = Date.now()
  return new Date(mw.starts_at).getTime() <= now && now <= new Date(mw.ends_at).getTime()
}

export default function AdminMaintenancePage() {
  const router = useRouter()
  const t = useTranslations()
  const { user } = useAuthStore()

  const [maintenances, setMaintenances] = useState<MaintenanceWindow[]>([])
  const [loading, setLoading] = useState(true)
  const [showCreate, setShowCreate] = useState(false)
  const [submitting, setSubmitting] = useState(false)

  // Form state
  const [title, setTitle] = useState('')
  const [description, setDescription] = useState('')
  const [startsAt, setStartsAt] = useState('')
  const [endsAt, setEndsAt] = useState('')

  // Every setState lives in a promise callback rather than after an `await`:
  // `react-hooks/set-state-in-effect` follows the call from the effect below, and it
  // cannot see that a `finally` block after a `catch` only runs post-await. Returns the
  // promise so the create/delete handlers can still wait for the refreshed list.
  const fetchMaintenances = useCallback(
    () =>
      maintenanceApi
        .list()
        .then(setMaintenances)
        .catch(() => {
          toastManager.add({ title: t('admin.maintenance.loadFailedToast'), type: 'error' })
        })
        .finally(() => setLoading(false)),
    []
  )

  useEffect(() => {
    if (!user?.is_admin) {
      router.push('/dashboard')
      return
    }
    fetchMaintenances()
  }, [user, router, fetchMaintenances])

  function resetForm() {
    setTitle('')
    setDescription('')
    setStartsAt('')
    setEndsAt('')
  }

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault()
    if (!title || !startsAt || !endsAt) return

    const req: CreateMaintenanceRequest = {
      title,
      description: description || undefined,
      starts_at: new Date(startsAt).toISOString(),
      ends_at: new Date(endsAt).toISOString(),
    }

    if (new Date(req.starts_at) >= new Date(req.ends_at)) {
      toastManager.add({ title: t('admin.maintenance.invalidRangeToast'), type: 'error' })
      return
    }

    setSubmitting(true)
    try {
      await maintenanceApi.create(req)
      toastManager.add({ title: t('admin.maintenance.scheduledToast'), type: 'success' })
      setShowCreate(false)
      resetForm()
      await fetchMaintenances()
    } catch {
      toastManager.add({ title: t('admin.maintenance.scheduleFailedToast'), type: 'error' })
    } finally {
      setSubmitting(false)
    }
  }

  async function handleDelete(id: string) {
    if (!window.confirm(t('admin.maintenance.deleteConfirm'))) return
    try {
      await maintenanceApi.remove(id)
      toastManager.add({ title: t('admin.maintenance.deletedToast'), type: 'success' })
      await fetchMaintenances()
    } catch {
      toastManager.add({ title: t('admin.maintenance.deleteFailedToast'), type: 'error' })
    }
  }

  return (
    <div className="mx-auto max-w-3xl px-4 py-8">
      {/* Page header */}
      <div className="mb-6 flex items-start justify-between">
        <div>
          <h1 className="text-2xl font-bold text-text">{t('admin.maintenance.heading')}</h1>
          <p className="mt-1 text-sm text-text-sub">{t('admin.maintenance.intro')}</p>
        </div>
        <Button variant="gradient" onClick={() => setShowCreate(true)}>
          <Plus className="mr-2 size-4" />
          {t('admin.maintenance.scheduleButton')}
        </Button>
      </div>

      {/* Maintenance window list */}
      {loading ? (
        <Card className="space-y-3 p-6">
          {Array.from({ length: 3 }).map((_, i) => (
            <Skeleton key={i} className="h-16 w-full rounded-lg" />
          ))}
        </Card>
      ) : maintenances.length === 0 ? (
        <Card className="p-6 text-center">
          <Wrench className="mx-auto mb-3 size-8 text-text-dim" strokeWidth={1} />
          <p className="text-base font-bold text-text">{t('admin.maintenance.emptyTitle')}</p>
          <p className="mt-1 text-sm text-text-sub">{t('admin.maintenance.emptyBody')}</p>
        </Card>
      ) : (
        <Card className="overflow-hidden">
          <div className="border-b border-border px-4 py-3">
            <h2 className="text-base font-bold text-text">
              {t('admin.maintenance.listHeading', { count: maintenances.length })}
            </h2>
          </div>
          <div className="divide-y divide-border">
            {maintenances.map((mw) => {
              const active = isActive(mw)
              return (
                <div key={mw.id} className="flex items-start gap-4 px-4 py-4">
                  <div className="min-w-0 flex-1">
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="text-sm font-medium text-text">{mw.title}</span>
                      <span
                        className={cn(
                          'inline-flex items-center rounded-full border px-2 py-0.5 text-xs font-bold',
                          active
                            ? 'border-amber-500/20 bg-amber-500/10 text-amber-400'
                            : 'border-blue-500/20 bg-blue-500/10 text-blue-400'
                        )}
                      >
                        {active
                          ? t('admin.maintenance.statusActive')
                          : t('admin.maintenance.statusUpcoming')}
                      </span>
                    </div>
                    <p className="mt-0.5 text-xs text-text-sub">
                      {formatRange(mw.starts_at, mw.ends_at)}
                    </p>
                    {mw.description && (
                      <p className="mt-1 text-sm text-text-sub">{mw.description}</p>
                    )}
                  </div>
                  <Button
                    variant="ghost"
                    size="icon"
                    onClick={() => handleDelete(mw.id)}
                    aria-label={t('admin.maintenance.deleteLabel', { title: mw.title })}
                    className="shrink-0 text-text-dim hover:text-destructive"
                  >
                    <Trash2 className="size-4" />
                  </Button>
                </div>
              )
            })}
          </div>
        </Card>
      )}

      {/* Create maintenance dialog */}
      <DialogRoot open={showCreate} onOpenChange={setShowCreate}>
        <DialogContent>
          <DialogTitle>{t('admin.maintenance.dialogTitle')}</DialogTitle>
          <DialogDescription>{t('admin.maintenance.dialogBody')}</DialogDescription>
          <form onSubmit={handleCreate} className="mt-4 space-y-4">
            <div>
              <label className="mb-1 block text-sm font-medium text-text" htmlFor="maint-title">
                {t('admin.maintenance.titleLabel')} <span className="text-destructive">*</span>
              </label>
              <input
                id="maint-title"
                type="text"
                required
                maxLength={200}
                value={title}
                onChange={(e) => setTitle(e.target.value)}
                placeholder={t('admin.maintenance.titlePlaceholder')}
                className="w-full rounded-lg border border-border bg-surface-2 px-3 py-2 text-sm text-text placeholder:text-text-dim focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
              />
            </div>
            <div>
              <label
                className="mb-1 block text-sm font-medium text-text"
                htmlFor="maint-description"
              >
                {t('admin.maintenance.descriptionLabel')}
              </label>
              <textarea
                id="maint-description"
                rows={3}
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                placeholder={t('admin.maintenance.descriptionPlaceholder')}
                className="w-full rounded-lg border border-border bg-surface-2 px-3 py-2 text-sm text-text placeholder:text-text-dim focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
              />
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div>
                <label
                  className="mb-1 block text-sm font-medium text-text"
                  htmlFor="maint-starts-at"
                >
                  {t('admin.maintenance.startsAtLabel')} <span className="text-destructive">*</span>
                </label>
                <input
                  id="maint-starts-at"
                  type="datetime-local"
                  required
                  value={startsAt}
                  onChange={(e) => setStartsAt(e.target.value)}
                  className="w-full rounded-lg border border-border bg-surface-2 px-3 py-2 text-sm text-text focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
                />
              </div>
              <div>
                <label className="mb-1 block text-sm font-medium text-text" htmlFor="maint-ends-at">
                  {t('admin.maintenance.endsAtLabel')} <span className="text-destructive">*</span>
                </label>
                <input
                  id="maint-ends-at"
                  type="datetime-local"
                  required
                  value={endsAt}
                  onChange={(e) => setEndsAt(e.target.value)}
                  className="w-full rounded-lg border border-border bg-surface-2 px-3 py-2 text-sm text-text focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
                />
              </div>
            </div>
            <div className="mt-2 flex justify-end gap-3">
              <Button
                type="button"
                variant="outline"
                onClick={() => {
                  setShowCreate(false)
                  resetForm()
                }}
              >
                {t('admin.maintenance.cancelButton')}
              </Button>
              <Button type="submit" variant="gradient" disabled={submitting}>
                {submitting
                  ? t('admin.maintenance.submittingButton')
                  : t('admin.maintenance.scheduleButton')}
              </Button>
            </div>
          </form>
        </DialogContent>
      </DialogRoot>
    </div>
  )
}
