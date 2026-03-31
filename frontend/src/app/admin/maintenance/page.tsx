'use client'

import { useEffect, useState, useCallback } from 'react'
import { useRouter } from 'next/navigation'
import { Trash2, Plus, Wrench } from 'lucide-react'
import { useAuthStore } from '@/lib/stores/auth-store'
import { maintenanceApi } from '@/lib/api/maintenance'
import { Card } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import {
  DialogRoot,
  DialogContent,
  DialogTitle,
  DialogDescription,
} from '@/components/ui/dialog'
import { toastManager } from '@/lib/toast'
import { cn } from '@/lib/utils'
import type { MaintenanceWindow, CreateMaintenanceRequest } from '@/lib/types/maintenance'

const DATE_FORMAT = new Intl.DateTimeFormat(undefined, {
  year: 'numeric',
  month: 'short',
  day: 'numeric',
  hour: '2-digit',
  minute: '2-digit',
})

function formatRange(startsAt: string, endsAt: string): string {
  return `${DATE_FORMAT.format(new Date(startsAt))} – ${DATE_FORMAT.format(new Date(endsAt))}`
}

function isActive(mw: MaintenanceWindow): boolean {
  const now = Date.now()
  return new Date(mw.starts_at).getTime() <= now && now <= new Date(mw.ends_at).getTime()
}

export default function AdminMaintenancePage() {
  const router = useRouter()
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

  const fetchMaintenances = useCallback(async () => {
    try {
      const data = await maintenanceApi.list()
      setMaintenances(data)
    } catch {
      toastManager.add({ title: 'Failed to load maintenance windows', type: 'error' })
    } finally {
      setLoading(false)
    }
  }, [])

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
      toastManager.add({ title: 'Start time must be before end time', type: 'error' })
      return
    }

    setSubmitting(true)
    try {
      await maintenanceApi.create(req)
      toastManager.add({ title: 'Maintenance scheduled', type: 'success' })
      setShowCreate(false)
      resetForm()
      await fetchMaintenances()
    } catch {
      toastManager.add({ title: 'Failed to schedule maintenance', type: 'error' })
    } finally {
      setSubmitting(false)
    }
  }

  async function handleDelete(id: string) {
    if (!window.confirm('Delete this maintenance window?')) return
    try {
      await maintenanceApi.remove(id)
      toastManager.add({ title: 'Maintenance window deleted', type: 'success' })
      await fetchMaintenances()
    } catch {
      toastManager.add({ title: 'Failed to delete maintenance window', type: 'error' })
    }
  }

  return (
    <div className="mx-auto max-w-3xl px-4 py-8">
      {/* Page header */}
      <div className="mb-6 flex items-start justify-between">
        <div>
          <h1 className="text-2xl font-bold text-text">Maintenance</h1>
          <p className="mt-1 text-sm text-text-sub">
            Schedule planned downtime windows. Users see a banner on the dashboard for upcoming and
            active maintenance.
          </p>
        </div>
        <Button variant="gradient" onClick={() => setShowCreate(true)}>
          <Plus className="mr-2 size-4" />
          Schedule
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
          <p className="text-base font-bold text-text">No maintenance windows scheduled</p>
          <p className="mt-1 text-sm text-text-sub">
            Schedule a maintenance window to notify users of upcoming downtime.
          </p>
        </Card>
      ) : (
        <Card className="overflow-hidden">
          <div className="border-b border-border px-4 py-3">
            <h2 className="text-base font-bold text-text">
              Scheduled Windows ({maintenances.length})
            </h2>
          </div>
          <div className="divide-y divide-border">
            {maintenances.map((mw) => {
              const active = isActive(mw)
              return (
                <div key={mw.id} className="flex items-start gap-4 px-4 py-4">
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2 flex-wrap">
                      <span className="font-medium text-sm text-text">{mw.title}</span>
                      <span
                        className={cn(
                          'inline-flex items-center rounded-full px-2 py-0.5 text-xs font-bold border',
                          active
                            ? 'bg-amber-500/10 text-amber-400 border-amber-500/20'
                            : 'bg-blue-500/10 text-blue-400 border-blue-500/20'
                        )}
                      >
                        {active ? 'Active' : 'Upcoming'}
                      </span>
                    </div>
                    <p className="mt-0.5 text-xs text-text-sub">{formatRange(mw.starts_at, mw.ends_at)}</p>
                    {mw.description && (
                      <p className="mt-1 text-sm text-text-sub">{mw.description}</p>
                    )}
                  </div>
                  <Button
                    variant="ghost"
                    size="icon"
                    onClick={() => handleDelete(mw.id)}
                    aria-label={`Delete ${mw.title}`}
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
          <DialogTitle>Schedule Maintenance</DialogTitle>
          <DialogDescription>
            Create a maintenance window. Users will see a banner on the dashboard until the window
            ends.
          </DialogDescription>
          <form onSubmit={handleCreate} className="mt-4 space-y-4">
            <div>
              <label className="mb-1 block text-sm font-medium text-text" htmlFor="maint-title">
                Title <span className="text-destructive">*</span>
              </label>
              <input
                id="maint-title"
                type="text"
                required
                maxLength={200}
                value={title}
                onChange={(e) => setTitle(e.target.value)}
                placeholder="e.g. Database maintenance"
                className="w-full rounded-lg border border-border bg-surface-2 px-3 py-2 text-sm text-text placeholder:text-text-dim focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-twitch"
              />
            </div>
            <div>
              <label
                className="mb-1 block text-sm font-medium text-text"
                htmlFor="maint-description"
              >
                Description
              </label>
              <textarea
                id="maint-description"
                rows={3}
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                placeholder="Optional details about the maintenance"
                className="w-full rounded-lg border border-border bg-surface-2 px-3 py-2 text-sm text-text placeholder:text-text-dim focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-twitch"
              />
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div>
                <label
                  className="mb-1 block text-sm font-medium text-text"
                  htmlFor="maint-starts-at"
                >
                  Starts at <span className="text-destructive">*</span>
                </label>
                <input
                  id="maint-starts-at"
                  type="datetime-local"
                  required
                  value={startsAt}
                  onChange={(e) => setStartsAt(e.target.value)}
                  className="w-full rounded-lg border border-border bg-surface-2 px-3 py-2 text-sm text-text focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-twitch"
                />
              </div>
              <div>
                <label
                  className="mb-1 block text-sm font-medium text-text"
                  htmlFor="maint-ends-at"
                >
                  Ends at <span className="text-destructive">*</span>
                </label>
                <input
                  id="maint-ends-at"
                  type="datetime-local"
                  required
                  value={endsAt}
                  onChange={(e) => setEndsAt(e.target.value)}
                  className="w-full rounded-lg border border-border bg-surface-2 px-3 py-2 text-sm text-text focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-twitch"
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
                Cancel
              </Button>
              <Button type="submit" variant="gradient" disabled={submitting}>
                {submitting ? 'Scheduling…' : 'Schedule'}
              </Button>
            </div>
          </form>
        </DialogContent>
      </DialogRoot>
    </div>
  )
}
