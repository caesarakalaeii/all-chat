'use client'

import { useEffect } from 'react'
import { useRouter } from 'next/navigation'
import { MonitorPlay, Plus, Trash2 } from 'lucide-react'
import { useOverlayStore } from '@/lib/stores/overlay-store'
import { toastManager } from '@/lib/toast'
import { AppNav } from '@/components/AppNav'
import { Card } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { Dialog } from '@/components/ui/dialog'
import { PlatformBadge } from '@/components/ui/badge'
import { ProtectedRoute } from '@/components/ProtectedRoute'
import type { ChatSource } from '@/lib/types/overlay'

// Extended overlay type that includes sources when available
interface OverlayWithSources {
  id: string
  name: string
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
      stops.push(
        `${color} calc(${start}% + ${blend}%)`,
        `${color} calc(${end}% - ${blend}%)`
      )
    }
  })
  return { background: `linear-gradient(90deg, ${stops.join(', ')})` }
}

// --- Skeleton loading state (3 placeholder cards) ---

function OverlayGridSkeleton() {
  return (
    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
      {Array.from({ length: 3 }).map((_, i) => (
        <div key={i} className="rounded-xl border border-border bg-surface overflow-hidden">
          <div className="h-[3px] w-full bg-surface-2" />
          <div className="p-6 space-y-3">
            <Skeleton className="h-4 w-1/2" />
            <Skeleton className="h-3 w-3/4" />
            <div className="flex gap-1.5 mt-2">
              <Skeleton className="h-4 w-12 rounded-full" />
              <Skeleton className="h-4 w-12 rounded-full" />
            </div>
            <Skeleton className="h-3 w-1/3 mt-3" />
          </div>
        </div>
      ))}
    </div>
  )
}

// --- Empty state ---

function DashboardEmptyState({ onCreateClick }: { onCreateClick: () => void }) {
  return (
    <div className="flex flex-col items-center py-24 text-center gap-4">
      <MonitorPlay className="size-16 text-text-dim" strokeWidth={1} aria-hidden="true" />
      <h2 className="text-xl font-semibold text-text">No overlays yet</h2>
      <p className="text-text-sub text-sm max-w-sm">
        Create your first overlay to start aggregating chat across platforms.
      </p>
      <div className="flex gap-1.5 mt-2" aria-hidden="true">
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
        <div className="flex gap-3 justify-end mt-6">
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

  useEffect(() => {
    fetchOverlays()
  }, [fetchOverlays])

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

  // Cast overlays to extended type — sources may be present if API returns them
  const overlaysWithSources = overlays as unknown as OverlayWithSources[]

  return (
    <div className="min-h-screen bg-bg">
      <AppNav />
      <main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
        <div className="flex items-center justify-between mb-8">
          <h1 className="text-2xl font-bold text-text">Overlays</h1>
          <Button variant="gradient" onClick={() => router.push('/overlays/new')}>
            <Plus className="size-4 mr-2" />
            New Overlay
          </Button>
        </div>

        {loading ? (
          <OverlayGridSkeleton />
        ) : overlaysWithSources.length === 0 ? (
          <DashboardEmptyState onCreateClick={() => router.push('/overlays/new')} />
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
            {overlaysWithSources.map((overlay) => (
              <Card
                key={overlay.id}
                interactive
                className="overflow-hidden cursor-pointer group"
                onClick={() => router.push(`/overlays/${overlay.id}`)}
              >
                <div style={{ height: '3px', ...getTopBorderStyle(overlay.sources ?? []) }} />
                <div className="p-6">
                  <div className="flex items-start justify-between mb-3">
                    <h3 className="font-semibold text-text truncate pr-2">{overlay.name}</h3>
                    <DeleteOverlayDialog
                      overlayName={overlay.name}
                      onDelete={() => handleDelete(overlay.id)}
                    >
                      <Button
                        variant="ghost"
                        size="icon"
                        onClick={(e: React.MouseEvent) => e.stopPropagation()}
                        aria-label={`Delete ${overlay.name}`}
                        className="text-text-dim hover:text-destructive shrink-0"
                      >
                        <Trash2 className="size-4" />
                      </Button>
                    </DeleteOverlayDialog>
                  </div>
                  <div className="flex flex-wrap gap-1.5 mb-4">
                    {overlay.sources?.map((source) => (
                      <PlatformBadge
                        key={source.id}
                        platform={
                          source.platform as 'twitch' | 'youtube' | 'kick' | 'tiktok'
                        }
                        size="sm"
                      />
                    ))}
                  </div>
                  <p className="text-xs text-text-dim">
                    {overlay.sources?.length ?? 0} source
                    {(overlay.sources?.length ?? 0) !== 1 ? 's' : ''}
                  </p>
                </div>
              </Card>
            ))}
          </div>
        )}
      </main>
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
