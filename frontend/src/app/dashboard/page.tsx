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

import { useEffect, useState } from 'react'
import { overlaysApi } from '@/lib/api/overlays'
import { useRouter } from 'next/navigation'
import { MonitorPlay, Plus, Trash2, Puzzle } from 'lucide-react'
import { useOverlayStore } from '@/lib/stores/overlay-store'
import { toastManager } from '@/lib/toast'
import { AppNav } from '@/components/AppNav'
import { Card } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { Dialog } from '@/components/ui/dialog'
import { PlatformBadge } from '@/components/ui/badge'
import { ProtectedRoute } from '@/components/ProtectedRoute'
import { MaintenanceBanner } from '@/components/MaintenanceBanner'
import { EventSubMigrationBanner } from '@/components/EventSubMigrationBanner'
import { OnboardingChecklist } from '@/components/onboarding/OnboardingChecklist'
import { CreateOverlayDialog } from '@/components/onboarding/CreateOverlayDialog'
import { useOnboardingStore } from '@/lib/stores/onboarding-store'
import { useAuthStore } from '@/lib/stores/auth-store'
import type { ChatSource } from '@/lib/types/overlay'

// Extended overlay type that includes sources when available
interface OverlayWithSources {
  id: string
  name: string
  is_public_for_viewers: boolean
  sources?: ChatSource[]
}

// --- Platform top border helpers ---

const PLATFORM_HEX: Record<string, string> = {
  twitch: '#A37BFF',
  youtube: '#FF4444',
  kick: '#53FC18',
  tiktok: '#69C9D0',
}

function getTopBorderStyle(sources: Array<{ platform: string }>): React.CSSProperties {
  if (sources.length === 0) return { background: 'var(--color-border)' }
  if (sources.length === 1)
    return { background: PLATFORM_HEX[sources[0].platform] ?? 'var(--color-border)' }
  const colors = sources.map((s) => PLATFORM_HEX[s.platform] ?? '#888')
  const segment = 100 / colors.length
  const blend = 5
  const stops: string[] = []
  colors.forEach((color, i) => {
    const start = i * segment
    const end = (i + 1) * segment
    if (i === 0) {
      stops.push(`${color} 0%`, `${color} calc(${end}% - ${blend}%)`)
    } else if (i === colors.length - 1) {
      stops.push(`${color} calc(${start}% + ${blend}%)`, `${color} 100%`)
    } else {
      stops.push(`${color} calc(${start}% + ${blend}%)`, `${color} calc(${end}% - ${blend}%)`)
    }
  })
  return { background: `linear-gradient(90deg, ${stops.join(', ')})` }
}

// --- Skeleton loading state (3 placeholder cards) ---

function OverlayGridSkeleton() {
  return (
    <div className="grid grid-cols-1 gap-6 md:grid-cols-2 lg:grid-cols-3">
      {Array.from({ length: 3 }).map((_, i) => (
        <div key={i} className="overflow-hidden rounded-xl border border-border bg-surface">
          <div className="h-[3px] w-full bg-surface-2" />
          <div className="space-y-3 p-6">
            <Skeleton className="h-4 w-1/2" />
            <Skeleton className="h-3 w-3/4" />
            <div className="mt-2 flex gap-1.5">
              <Skeleton className="h-4 w-12 rounded-full" />
              <Skeleton className="h-4 w-12 rounded-full" />
            </div>
            <Skeleton className="mt-3 h-3 w-1/3" />
          </div>
        </div>
      ))}
    </div>
  )
}

// --- Empty state ---

function DashboardEmptyState({ onCreateClick }: { onCreateClick: () => void }) {
  return (
    <div className="flex flex-col items-center gap-4 py-24 text-center">
      <MonitorPlay className="size-16 text-text-dim" strokeWidth={1} aria-hidden="true" />
      <h2 className="text-xl font-semibold text-text">No overlays yet</h2>
      <p className="max-w-sm text-sm text-text-sub">
        Create your first overlay to see chat from all your platforms in one place.
      </p>
      <div className="mt-2 flex gap-1.5" aria-hidden="true">
        {(['twitch', 'youtube', 'kick', 'tiktok'] as const).map((p) => (
          <PlatformBadge key={p} platform={p} size="sm" />
        ))}
      </div>
      <Button variant="gradient" size="lg" onClick={onCreateClick} className="mt-4">
        Create your first overlay
      </Button>
    </div>
  )
}

// --- Delete confirmation dialog ---

function DeleteOverlayDialog({
  overlayName,
  onDelete,
  children,
}: {
  overlayName: string
  onDelete: () => void
  children: React.ReactNode
}) {
  return (
    <Dialog.Root>
      <Dialog.Trigger render={children as React.ReactElement} />
      <Dialog.Content showCloseButton={false}>
        <Dialog.Title>Delete &ldquo;{overlayName}&rdquo;?</Dialog.Title>
        <Dialog.Description>
          This action cannot be undone. All sources will be removed.
        </Dialog.Description>
        <div className="mt-6 flex justify-end gap-3">
          <Dialog.Close render={<Button variant="outline">Cancel</Button>} />
          <Button variant="destructive" onClick={onDelete}>
            Delete
          </Button>
        </div>
      </Dialog.Content>
    </Dialog.Root>
  )
}

// --- Dashboard content ---

function DashboardContent() {
  const router = useRouter()
  const { overlays, loading, fetchOverlays, deleteOverlay } = useOverlayStore()
  const [sourcesByOverlay, setSourcesByOverlay] = useState<Record<string, ChatSource[]>>({})
  const user = useAuthStore((s) => s.user)
  const onboardingStatus = useOnboardingStore((s) => s.status)
  const startOnboarding = useOnboardingStore((s) => s.start)
  const activeOverlayId = useOnboardingStore((s) => s.activeOverlayId)
  const setActiveOverlay = useOnboardingStore((s) => s.setActiveOverlay)
  const [onboardingCreateOpen, setOnboardingCreateOpen] = useState(false)
  const [overlaysFetched, setOverlaysFetched] = useState(false)

  useEffect(() => {
    fetchOverlays().then(() => setOverlaysFetched(true))
  }, [fetchOverlays])

  // First-run setup guide auto-start: only for a signed-in, non-impersonated
  // user whose server flag is explicitly null AND who has no overlays yet
  // (the zero-overlay guard protects users created between backend and
  // frontend deploys). Gated on overlaysFetched, NOT the store's loading
  // flag — that starts false before the first fetch, which would race the
  // guard into always seeing zero overlays. Settings restart bypasses this
  // via start('settings').
  useEffect(() => {
    if (!overlaysFetched || !user || user.impersonating) return
    if (user.onboarding_completed_at !== null) return
    if (overlays.length > 0) return
    startOnboarding('auto')
  }, [overlaysFetched, user, overlays.length, startOnboarding])

  // Steps 2-4 need an overlay to point at; on the dashboard bind them to the
  // first overlay when the editor hasn't set one.
  useEffect(() => {
    if (!activeOverlayId && overlays.length > 0) setActiveOverlay(overlays[0].id)
  }, [activeOverlayId, overlays, setActiveOverlay])

  // Fetch sources for all overlays after the list loads
  useEffect(() => {
    if (loading || overlays.length === 0) return
    Promise.allSettled(
      overlays.map((o) => overlaysApi.getSources(o.id).then((sources) => ({ id: o.id, sources })))
    ).then((results) => {
      const map: Record<string, ChatSource[]> = {}
      results.forEach((r) => {
        if (r.status === 'fulfilled') map[r.value.id] = r.value.sources
      })
      setSourcesByOverlay(map)
    })
  }, [overlays, loading])

  async function handleDelete(id: string) {
    try {
      await deleteOverlay(id)
      toastManager.add({ title: 'Overlay deleted', type: 'success' })
    } catch {
      toastManager.add({
        title: 'Failed to delete overlay',
        description: 'Please try again.',
        type: 'error',
      })
    }
  }

  async function handleSetPublic(id: string) {
    try {
      await overlaysApi.update(id, { is_public_for_viewers: true })
      await fetchOverlays()
      toastManager.add({ title: 'Extension overlay updated', type: 'success' })
    } catch {
      toastManager.add({ title: 'Failed to update overlay', type: 'error' })
    }
  }

  async function handleUnsetPublic(id: string) {
    try {
      await overlaysApi.update(id, { is_public_for_viewers: false })
      await fetchOverlays()
      toastManager.add({ title: 'Extension overlay deactivated', type: 'success' })
    } catch {
      toastManager.add({ title: 'Failed to update overlay', type: 'error' })
    }
  }

  const overlaysWithSources: OverlayWithSources[] = overlays.map((o) => ({
    ...o,
    sources: sourcesByOverlay[o.id],
  }))

  return (
    <div className="min-h-screen bg-bg">
      <AppNav />
      <main id="main-content" tabIndex={-1} className="mx-auto max-w-7xl px-4 py-8 sm:px-6 lg:px-8">
        <div className="mb-4 space-y-4">
          <MaintenanceBanner />
          <EventSubMigrationBanner sourcesByOverlay={sourcesByOverlay} />
        </div>
        <div className="mb-8 flex items-center justify-between">
          <h1 className="text-2xl font-bold text-text">Overlays</h1>
          <Button variant="gradient" onClick={() => router.push('/overlays/new')}>
            <Plus className="mr-2 size-4" />
            New Overlay
          </Button>
        </div>

        {loading ? (
          <OverlayGridSkeleton />
        ) : overlaysWithSources.length === 0 ? (
          <DashboardEmptyState
            onCreateClick={() =>
              // During onboarding the empty-state CTA IS step 1 — one path,
              // not two competing ones.
              onboardingStatus === 'active'
                ? setOnboardingCreateOpen(true)
                : router.push('/overlays/new')
            }
          />
        ) : (
          <div className="grid grid-cols-1 gap-6 md:grid-cols-2 lg:grid-cols-3">
            {overlaysWithSources.map((overlay) => (
              <Card
                key={overlay.id}
                interactive
                className="group cursor-pointer overflow-hidden"
                onClick={() => router.push(`/overlays/${overlay.id}`)}
              >
                <div style={{ height: '3px', ...getTopBorderStyle(overlay.sources ?? []) }} />
                <div className="p-6">
                  <div className="mb-3 flex items-start justify-between">
                    <div className="flex min-w-0 items-center gap-2">
                      <h3 className="truncate font-semibold text-text">{overlay.name}</h3>
                      {overlay.is_public_for_viewers && (
                        <span className="inline-flex shrink-0 items-center gap-1 rounded border border-twitch/30 bg-twitch/15 px-1.5 py-0.5 text-[10px] font-semibold text-twitch">
                          <Puzzle className="size-2.5" />
                          Extension
                        </span>
                      )}
                    </div>
                    <DeleteOverlayDialog
                      overlayName={overlay.name}
                      onDelete={() => handleDelete(overlay.id)}
                    >
                      <Button
                        variant="ghost"
                        size="icon"
                        onClick={(e: React.MouseEvent) => e.stopPropagation()}
                        aria-label={`Delete ${overlay.name}`}
                        className="hover:text-destructive shrink-0 text-text-dim"
                      >
                        <Trash2 className="size-4" />
                      </Button>
                    </DeleteOverlayDialog>
                  </div>
                  <div className="mb-4 flex flex-wrap gap-1.5">
                    {overlay.sources?.map((source) => (
                      <PlatformBadge
                        key={source.id}
                        platform={source.platform as 'twitch' | 'youtube' | 'kick' | 'tiktok'}
                        size="sm"
                      />
                    ))}
                  </div>
                  <div className="flex items-center justify-between">
                    <p className="text-xs text-text-dim">
                      {overlay.sources?.length ?? 0} source
                      {(overlay.sources?.length ?? 0) !== 1 ? 's' : ''}
                    </p>
                    {overlay.is_public_for_viewers ? (
                      <Button
                        variant="ghost"
                        size="sm"
                        className="hover:text-destructive -mr-2 gap-1.5 text-xs text-text-sub"
                        onClick={(e: React.MouseEvent) => {
                          e.stopPropagation()
                          handleUnsetPublic(overlay.id)
                        }}
                      >
                        Deactivate Extension
                      </Button>
                    ) : (
                      <Button
                        variant="ghost"
                        size="sm"
                        className="-mr-2 gap-1.5 text-xs text-text-sub hover:text-twitch"
                        onClick={(e: React.MouseEvent) => {
                          e.stopPropagation()
                          handleSetPublic(overlay.id)
                        }}
                      >
                        <Puzzle className="size-3" />
                        Set as Extension Overlay
                      </Button>
                    )}
                  </div>
                </div>
              </Card>
            ))}
          </div>
        )}
      </main>

      <OnboardingChecklist surface="dashboard" overlayCount={overlays.length} />
      <CreateOverlayDialog open={onboardingCreateOpen} onOpenChange={setOnboardingCreateOpen} />
    </div>
  )
}

export default function DashboardPage() {
  return (
    <ProtectedRoute>
      <DashboardContent />
    </ProtectedRoute>
  )
}
