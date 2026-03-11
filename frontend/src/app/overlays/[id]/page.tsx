/**
 * Overlay Editor Page
 *
 * Edit an existing overlay and manage its chat sources.
 * Displays a split-view layout: config panel left, live preview iframe right.
 *
 * Features:
 * - Platform-colored source cards with PlatformBadge
 * - Draggable split-view with live overlay preview
 * - Dialog confirmation for source removal
 * - Toast feedback for all actions
 * - Skeleton loading states
 */

'use client'

import { use, useEffect, useState } from 'react'
import { useRouter } from 'next/navigation'
import { ChevronLeft, X } from 'lucide-react'
import { useAuthStore } from '@/lib/stores/auth-store'
import { overlaysApi } from '@/lib/api/overlays'
import type { Overlay, ChatSource } from '@/lib/types/overlay'
import { toastManager } from '@/lib/toast'
import { AppNav } from '@/components/AppNav'
import { BetaWarning } from '@/components/BetaWarning'
import { SplitView } from '@/components/SplitView'
import { Card } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { Dialog } from '@/components/ui/dialog'
import { PlatformBadge } from '@/components/ui/badge'
import { cn } from '@/lib/utils'

// Static platform border mapping — full literal class strings for Tailwind JIT safety
const PLATFORM_BORDER: Record<string, string> = {
  twitch:  'border-l-twitch',
  youtube: 'border-l-youtube',
  kick:    'border-l-kick',
  tiktok:  'border-l-tiktok',
}

// ---- Sub-components --------------------------------------------------------

function SourceCard({
  source,
  onRemove,
}: {
  source: ChatSource
  onRemove: (id: string) => void
}) {
  const borderClass = PLATFORM_BORDER[source.platform] ?? 'border-l-border'
  return (
    <Card className={cn('p-4 border-l-2', borderClass)}>
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <PlatformBadge platform={source.platform} />
          <div>
            <p className="text-sm font-medium text-text">
              {source.channel_name ?? source.channel_id}
            </p>
            <p className="text-xs text-text-sub capitalize">{source.platform}</p>
          </div>
        </div>
        <Dialog.Root>
          <Dialog.Trigger
            render={
              <Button
                variant="ghost"
                size="icon"
                aria-label={`Remove ${source.channel_name ?? source.channel_id}`}
                className="text-text-dim hover:text-destructive"
              >
                <X className="size-4" />
              </Button>
            }
          />
          <Dialog.Content showCloseButton={false}>
            <Dialog.Title>Remove source?</Dialog.Title>
            <Dialog.Description>
              Remove {source.channel_name ?? source.channel_id} ({source.platform}) from this
              overlay? This cannot be undone.
            </Dialog.Description>
            <div className="flex gap-3 justify-end mt-6">
              <Dialog.Close render={<Button variant="outline">Cancel</Button>} />
              <Button variant="destructive" onClick={() => onRemove(source.id)}>
                Remove
              </Button>
            </div>
          </Dialog.Content>
        </Dialog.Root>
      </div>
    </Card>
  )
}

function SourceListSkeleton() {
  return (
    <div className="space-y-3">
      {Array.from({ length: 2 }).map((_, i) => (
        <div
          key={i}
          className="rounded-xl border border-border bg-surface p-4 flex items-center gap-3"
        >
          <Skeleton className="size-8 rounded-full" />
          <div className="space-y-2 flex-1">
            <Skeleton className="h-4 w-1/3" />
            <Skeleton className="h-3 w-1/4" />
          </div>
        </div>
      ))}
    </div>
  )
}

function AddSourceForm({
  onAdd,
}: {
  onAdd: (platform: string, channelId: string) => Promise<void>
}) {
  const [platform, setPlatform] = useState('twitch')
  const [channelId, setChannelId] = useState('')
  const [isAdding, setIsAdding] = useState(false)

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!channelId.trim()) return
    setIsAdding(true)
    await onAdd(platform, channelId.trim())
    setChannelId('')
    setIsAdding(false)
  }

  return (
    <form onSubmit={handleSubmit} className="border border-border rounded-xl p-4 space-y-3">
      <h3 className="text-sm font-semibold text-text">Add Source</h3>
      <div className="flex gap-2">
        <select
          value={platform}
          onChange={e => setPlatform(e.target.value)}
          className="flex-shrink-0 rounded-lg border border-border bg-surface text-text text-sm px-3 py-2 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-twitch"
          aria-label="Platform"
        >
          <option value="twitch">Twitch</option>
          <option value="youtube">YouTube</option>
          <option value="kick">Kick</option>
          <option value="tiktok">TikTok</option>
        </select>
        <Input
          placeholder="Channel ID or username"
          value={channelId}
          onChange={e => setChannelId(e.target.value)}
          className="flex-1"
          aria-label="Channel ID or username"
        />
      </div>
      <Button
        variant="gradient"
        type="submit"
        disabled={isAdding || !channelId.trim()}
        className="w-full"
      >
        {isAdding ? <Skeleton className="h-4 w-20" /> : 'Add Source'}
      </Button>
    </form>
  )
}

// ---- Page ------------------------------------------------------------------

export default function OverlayEditorPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params)
  const router = useRouter()
  const { token } = useAuthStore()

  const [overlay, setOverlay] = useState<Overlay | null>(null)
  const [sources, setSources] = useState<ChatSource[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [showBetaWarning, setShowBetaWarning] = useState<'youtube' | null>(null)

  // Load overlay and sources
  useEffect(() => {
    if (!token) {
      router.push('/')
      return
    }

    const loadData = async () => {
      try {
        const overlayData = await overlaysApi.get(id)
        setOverlay(overlayData)

        try {
          const sourcesData = await overlaysApi.getSources(id)
          setSources(sourcesData)
        } catch {
          console.warn('Sources endpoint not available yet, starting with empty sources')
          setSources([])
        }
      } catch (error) {
        console.error('Failed to load overlay:', error)
        setOverlay(null)
      } finally {
        setIsLoading(false)
      }
    }

    loadData()
  }, [id, token, router])

  async function handleRemoveSource(sourceId: string) {
    try {
      await overlaysApi.removeSource(id, sourceId)
      setSources(prev => prev.filter(s => s.id !== sourceId))
      toastManager.add({ title: 'Source removed', type: 'success' })
    } catch {
      toastManager.add({
        title: 'Failed to remove source',
        description: 'Please try again.',
        type: 'error',
      })
    }
  }

  async function handleAddSource(platform: string, channelId: string) {
    // Show beta warning for YouTube
    if (platform === 'youtube') {
      setShowBetaWarning('youtube')
      return
    }

    await doAddSource(platform, channelId)
  }

  async function doAddSource(platform: string, channelId: string) {
    try {
      const source = await overlaysApi.addSource(id, {
        platform: platform as ChatSource['platform'],
        channel_id: channelId,
      })
      setSources(prev => [...prev, source])
      toastManager.add({ title: 'Source added', type: 'success' })
    } catch {
      toastManager.add({
        title: 'Failed to add source',
        description: 'Verify the channel ID and try again.',
        type: 'error',
      })
    }
  }

  if (isLoading) {
    return (
      <div className="min-h-screen bg-bg">
        <AppNav />
        <div className="flex h-[calc(100vh-60px)] items-center justify-center">
          <div className="space-y-3 w-64">
            <Skeleton className="h-6 w-40" />
            <Skeleton className="h-4 w-full" />
            <Skeleton className="h-4 w-2/3" />
          </div>
        </div>
      </div>
    )
  }

  if (!overlay) {
    return (
      <div className="min-h-screen bg-bg">
        <AppNav />
        <div className="flex h-[calc(100vh-60px)] items-center justify-center">
          <div className="text-center">
            <p className="text-destructive text-lg mb-4">Overlay not found</p>
            <Button variant="outline" onClick={() => router.push('/dashboard')}>
              Return to Dashboard
            </Button>
          </div>
        </div>
      </div>
    )
  }

  return (
    <div className="min-h-screen bg-bg">
      <AppNav />
      <SplitView overlayId={id}>
        {/* Config panel content */}
        <div className="p-6 max-w-none">
          {/* Header */}
          <div className="flex items-center justify-between mb-6">
            <div>
              <button
                onClick={() => router.push('/dashboard')}
                className="text-text-sub hover:text-text text-sm flex items-center gap-1 mb-1 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-twitch rounded"
              >
                <ChevronLeft className="size-4" />
                Back
              </button>
              <h1 className="text-xl font-bold text-text">{overlay.name}</h1>
              {overlay.description && (
                <p className="text-sm text-text-sub mt-0.5">{overlay.description}</p>
              )}
            </div>
          </div>

          {/* Action buttons */}
          <div className="flex flex-wrap gap-2 mb-6">
            <Button
              variant="outline"
              size="sm"
              onClick={() => router.push(`/overlays/${id}/events`)}
              aria-label="Event settings"
            >
              Event Settings
            </Button>
            <Button
              variant="outline"
              size="sm"
              onClick={() => router.push(`/overlays/${id}/credits`)}
              aria-label="Credits"
            >
              Credits
            </Button>
          </div>

          {/* Sources section */}
          <section aria-label="Chat sources">
            <div className="flex items-center justify-between mb-4">
              <h2 className="text-base font-semibold text-text">Sources</h2>
            </div>

            {isLoading ? (
              <SourceListSkeleton />
            ) : (
              <div className="space-y-3 mb-6">
                {sources.map(source => (
                  <SourceCard key={source.id} source={source} onRemove={handleRemoveSource} />
                ))}
                {sources.length === 0 && (
                  <p className="text-text-sub text-sm py-4">
                    No sources added yet. Add a platform below.
                  </p>
                )}
              </div>
            )}

            {/* Add source form */}
            <AddSourceForm onAdd={handleAddSource} />
          </section>
        </div>
      </SplitView>

      {/* Beta Warning Modal */}
      {showBetaWarning && (
        <BetaWarning
          platform={showBetaWarning}
          onCancel={() => setShowBetaWarning(null)}
          onContinue={() => {
            setShowBetaWarning(null)
            // YouTube beta: user acknowledged, but we don't have channel ID here
            // Redirect to OAuth flow
            window.location.href = `/api/v1/auth/youtube/add-source/${id}`
          }}
        />
      )}
    </div>
  )
}
