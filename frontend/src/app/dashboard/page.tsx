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
import { cn } from '@/lib/utils'
import { archivoBlack, spaceMono } from '@/lib/fonts'
import { Skeleton } from '@/components/ui/skeleton'
import { VisuallyHidden } from '@/components/ui/visually-hidden'
import { Dialog } from '@/components/ui/dialog'
import { PlatformBadge } from '@/components/ui/badge'
import { ProtectedRoute } from '@/components/ProtectedRoute'
import { MaintenanceBanner } from '@/components/MaintenanceBanner'
import { EventSubMigrationBanner } from '@/components/EventSubMigrationBanner'
import { DiscoveryPausedNotice } from '@/components/DiscoveryPausedNotice'
import { ModeratingElsewhereCard } from '@/components/ModeratingElsewhereCard'
import { OnboardingChecklist } from '@/components/onboarding/OnboardingChecklist'
import { CreateOverlayDialog } from '@/components/onboarding/CreateOverlayDialog'
import { useOnboardingStore } from '@/lib/stores/onboarding-store'
import { useAuthStore } from '@/lib/stores/auth-store'
import { useTranslations } from '@/lib/i18n'
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
  owncast: '#9B7FF5',
  goodgame: '#7FA3D1',
  picarto: '#27B756',
  facebook: '#3B93F5',
  rumble: '#85C742',
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
  const t = useTranslations()
  return (
    <div role="status" className="grid grid-cols-1 gap-6 md:grid-cols-2 lg:grid-cols-3">
      <VisuallyHidden>{t('dashboard.overlays.loading')}</VisuallyHidden>
      {Array.from({ length: 3 }).map((_, i) => (
        <div key={i} className="overflow-hidden border-2 border-white bg-black">
          <div className="h-[3px] w-full bg-white/20" />
          <div className="space-y-3 p-6">
            <Skeleton className="h-4 w-1/2" />
            <Skeleton className="h-3 w-3/4" />
            <div className="mt-2 flex gap-1.5">
              <Skeleton className="h-4 w-12" />
              <Skeleton className="h-4 w-12" />
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
  const t = useTranslations()
  return (
    <div className="flex flex-col items-center gap-4 py-24 text-center">
      <MonitorPlay className="text-dim size-16" strokeWidth={1} aria-hidden="true" />
      <h2 className="text-xl">{t('dashboard.empty.heading')}</h2>
      <p className="text-sub max-w-sm text-sm">{t('dashboard.empty.body')}</p>
      <div className="mt-2 flex gap-1.5" aria-hidden="true">
        {(
          [
            'twitch',
            'youtube',
            'kick',
            'tiktok',
            'owncast',
            'goodgame',
            'picarto',
            'facebook',
            'rumble',
          ] as const
        ).map((p) => (
          <PlatformBadge key={p} platform={p} size="sm" />
        ))}
      </div>
      <button onClick={onCreateClick} className="lanes-btn mt-4">
        {t('dashboard.empty.createFirst')}
      </button>
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
  const t = useTranslations()
  return (
    <Dialog.Root>
      <Dialog.Trigger render={children as React.ReactElement} />
      <Dialog.Content
        showCloseButton={false}
        className="lanes-app rounded-none border-white bg-black"
      >
        <Dialog.Title className="text-lg">
          {t('dashboard.deleteOverlay.title', { name: overlayName })}
        </Dialog.Title>
        <Dialog.Description className="text-sub mt-2 text-sm">
          {t('dashboard.deleteOverlay.description')}
        </Dialog.Description>
        <div className="mt-6 flex justify-end gap-3">
          <Dialog.Close
            render={
              <button className="lanes-btn ghost">{t('dashboard.deleteOverlay.cancel')}</button>
            }
          />
          <button className="lanes-btn danger" onClick={onDelete}>
            {t('dashboard.deleteOverlay.confirm')}
          </button>
        </div>
      </Dialog.Content>
    </Dialog.Root>
  )
}

// --- Dashboard content ---

function DashboardContent() {
  const t = useTranslations()
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
      toastManager.add({ title: t('dashboard.toasts.overlayDeleted'), type: 'success' })
    } catch {
      toastManager.add({
        title: t('dashboard.toasts.overlayDeleteFailed'),
        description: t('common.toast.tryAgain'),
        type: 'error',
      })
    }
  }

  async function handleSetPublic(id: string) {
    try {
      await overlaysApi.update(id, { is_public_for_viewers: true })
      await fetchOverlays()
      toastManager.add({ title: t('dashboard.toasts.extensionOverlayUpdated'), type: 'success' })
    } catch {
      toastManager.add({ title: t('dashboard.toasts.overlayUpdateFailed'), type: 'error' })
    }
  }

  async function handleUnsetPublic(id: string) {
    try {
      await overlaysApi.update(id, { is_public_for_viewers: false })
      await fetchOverlays()
      toastManager.add({
        title: t('dashboard.toasts.extensionOverlayDeactivated'),
        type: 'success',
      })
    } catch {
      toastManager.add({ title: t('dashboard.toasts.overlayUpdateFailed'), type: 'error' })
    }
  }

  const overlaysWithSources: OverlayWithSources[] = overlays.map((o) => ({
    ...o,
    sources: sourcesByOverlay[o.id],
  }))

  return (
    <div className={cn('lanes-app min-h-screen', archivoBlack.variable, spaceMono.variable)}>
      <AppNav />
      <main id="main-content" tabIndex={-1} className="mx-auto max-w-7xl px-4 py-8 sm:px-6 lg:px-8">
        <div className="mb-4 space-y-4">
          <MaintenanceBanner />
          <EventSubMigrationBanner sourcesByOverlay={sourcesByOverlay} />
          {/* Channels other streamers delegated to this user. Renders nothing when there
              are none, so it costs a non-moderator a single request and no pixels. */}
          <ModeratingElsewhereCard />
        </div>
        <div className="mb-8 flex items-center justify-between">
          <h1 className="text-2xl">{t('dashboard.overlays.heading')}</h1>
          <button className="lanes-btn" onClick={() => router.push('/overlays/new')}>
            <Plus className="mr-2 size-4" />
            {t('dashboard.overlays.newOverlay')}
          </button>
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
              <div
                key={overlay.id}
                role="link"
                tabIndex={0}
                aria-label={t('dashboard.overlays.openLabel', { name: overlay.name })}
                className="lanes-panel group cursor-pointer overflow-hidden focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
                onClick={() => router.push(`/overlays/${overlay.id}`)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter' || e.key === ' ') {
                    e.preventDefault()
                    router.push(`/overlays/${overlay.id}`)
                  }
                }}
              >
                <div className="p-6">
                  <div className="mb-3 flex items-start justify-between">
                    <div className="flex min-w-0 items-center gap-2">
                      <h3 className="truncate">{overlay.name}</h3>
                      {overlay.is_public_for_viewers && (
                        <span className="lanes-chip shrink-0 text-twitch">
                          <Puzzle className="size-2.5" />
                          {t('dashboard.overlays.extensionBadge')}
                        </span>
                      )}
                    </div>
                    <DeleteOverlayDialog
                      overlayName={overlay.name}
                      onDelete={() => handleDelete(overlay.id)}
                    >
                      <button
                        className="lanes-btn ghost sm shrink-0"
                        onClick={(e: React.MouseEvent) => e.stopPropagation()}
                        aria-label={t('dashboard.overlays.deleteLabel', { name: overlay.name })}
                      >
                        <Trash2 className="size-4" />
                      </button>
                    </DeleteOverlayDialog>
                  </div>
                  {/* A parked YouTube channel, which nothing on this page used to show.
                      Above the badges so it is the first thing read on the card. */}
                  <DiscoveryPausedNotice overlayId={overlay.id} sources={overlay.sources} />
                  <div className="mb-4 flex flex-wrap gap-1.5">
                    {overlay.sources?.map((source) => (
                      <PlatformBadge
                        key={source.id}
                        platform={source.platform as string}
                        size="sm"
                      />
                    ))}
                  </div>
                  <div className="flex items-center justify-between">
                    <p className="text-dim text-xs">
                      {t(
                        (overlay.sources?.length ?? 0) === 1
                          ? 'dashboard.overlays.sourceCountOne'
                          : 'dashboard.overlays.sourceCountOther',
                        { count: overlay.sources?.length ?? 0 }
                      )}
                    </p>
                    {overlay.is_public_for_viewers ? (
                      <button
                        className="lanes-btn ghost -mr-2 px-3 py-1 text-xs text-youtube"
                        onClick={(e: React.MouseEvent) => {
                          e.stopPropagation()
                          handleUnsetPublic(overlay.id)
                        }}
                      >
                        {t('dashboard.overlays.deactivateExtension')}
                      </button>
                    ) : (
                      <button
                        className="lanes-btn ghost -mr-2 px-3 py-1 text-xs text-twitch"
                        onClick={(e: React.MouseEvent) => {
                          e.stopPropagation()
                          handleSetPublic(overlay.id)
                        }}
                      >
                        <Puzzle className="size-3" />
                        {t('dashboard.overlays.setAsExtension')}
                      </button>
                    )}
                  </div>
                </div>
              </div>
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
