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
 * - Shared overlay sources (v1.3) with revocation support
 * - WebSocket listener for real-time share_revoked notifications
 * - OAuth callback handling (source_added / error query params)
 */

'use client'

import { use, useEffect, useState } from 'react'
import { useRouter, useSearchParams } from 'next/navigation'
import { ChevronLeft, X } from 'lucide-react'
import { useAuthStore } from '@/lib/stores/auth-store'
import { overlaysApi } from '@/lib/api/overlays'
import { sharesApi } from '@/lib/api/shares'
import type { Overlay, ChatSource } from '@/lib/types/overlay'
import type { AcceptedShare } from '@/lib/types/share'
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
import { StatusBadge } from '@/app/dashboard/shares/components/StatusBadge'
import { RevocationConfirmModal } from '@/app/dashboard/shares/components/RevocationConfirmModal'
import { cn } from '@/lib/utils'

// Static platform border mapping — full literal class strings for Tailwind JIT safety
const PLATFORM_BORDER: Record<string, string> = {
  twitch:         'border-l-twitch',
  youtube:        'border-l-youtube',
  kick:           'border-l-kick',
  tiktok:         'border-l-tiktok',
  shared_overlay: 'border-l-twitch',
}

// ---- Sub-components --------------------------------------------------------

function SourceCard({
  source,
  onRemove,
  onRevoke,
}: {
  source: ChatSource
  onRemove: (id: string) => void
  onRevoke?: (source: ChatSource) => void
}) {
  const [confirmOpen, setConfirmOpen] = useState(false)
  const isShared = source.platform === 'shared_overlay'
  const isInactiveShared = isShared && !source.is_active

  return (
    <Card
      className={cn(
        'flex items-center justify-between p-4 border-l-2',
        PLATFORM_BORDER[source.platform] ?? 'border-l-border',
        isInactiveShared && 'opacity-50',
      )}
    >
      <div className="flex items-center gap-3 min-w-0">
        <PlatformBadge platform={source.platform as 'twitch' | 'youtube' | 'kick' | 'tiktok'} size="sm" />
        <span className="text-sm font-medium text-text truncate">
          {source.channel_name || source.channel_id}
        </span>
        {isInactiveShared && source.share_status && (
          <StatusBadge status={source.share_status} size="sm" />
        )}
      </div>
      <div className="flex items-center gap-2 shrink-0 ml-3">
        {isShared && source.is_active && onRevoke && (
          <Button
            variant="outline"
            size="sm"
            className="text-xs text-destructive border-destructive/40 hover:bg-destructive/10"
            onClick={() => onRevoke(source)}
          >
            Revoke
          </Button>
        )}
        <Dialog.Root open={confirmOpen} onOpenChange={setConfirmOpen}>
          <Dialog.Trigger
            render={
              <button
                className="text-text-sub hover:text-destructive transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-twitch rounded"
                aria-label={`Remove ${source.channel_name || source.channel_id}`}
              >
                <X className="size-4" />
              </button>
            }
          />
          <Dialog.Content>
            <Dialog.Title>Remove source?</Dialog.Title>
            <Dialog.Description>
              Remove <strong>{source.channel_name || source.channel_id}</strong> from this overlay.
              Chat messages from this source will stop appearing.
            </Dialog.Description>
            <div className="flex justify-end gap-3 pt-2">
              <Dialog.Close render={<Button variant="outline">Cancel</Button>} />
              <Button
                variant="destructive"
                onClick={() => { setConfirmOpen(false); onRemove(source.id) }}
              >
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
      {[0, 1].map(i => <Skeleton key={i} className="h-[60px] w-full rounded-xl" />)}
    </div>
  )
}

function AddSourceForm({ onAdd }: { onAdd: (platform: string, channelId: string) => void }) {
  const [platform, setPlatform] = useState('twitch')
  const [channelId, setChannelId] = useState('')
  const [isAdding, setIsAdding] = useState(false)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!channelId.trim()) return
    setIsAdding(true)
    try {
      await onAdd(platform, channelId.trim())
      setChannelId('')
    } finally {
      setIsAdding(false)
    }
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-3">
      <div className="flex gap-2">
        <select
          value={platform}
          onChange={e => setPlatform(e.target.value)}
          className="rounded-lg border border-border bg-surface px-3 py-2 text-sm text-text focus:outline-none focus:ring-2 focus:ring-twitch"
          aria-label="Platform"
        >
          <option value="twitch">Twitch</option>
          <option value="youtube">YouTube</option>
          <option value="kick">Kick</option>
          <option value="tiktok">TikTok</option>
        </select>
        <Input
          value={channelId}
          onChange={e => setChannelId(e.target.value)}
          placeholder="Channel ID or username"
          className="flex-1"
        />
      </div>
      <Button
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
  const searchParams = useSearchParams()
  const { token } = useAuthStore()

  const [overlay, setOverlay] = useState<Overlay | null>(null)
  const [sources, setSources] = useState<ChatSource[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [showBetaWarning, setShowBetaWarning] = useState<'youtube' | null>(null)
  const [acceptedShares, setAcceptedShares] = useState<AcceptedShare[]>([])
  const [revokeTarget, setRevokeTarget] = useState<ChatSource | null>(null)

  // Load overlay, sources, and accepted shares
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
          setSources([])
        }

        // Load accepted shares for shared overlay sources — non-critical
        try {
          const sharesData = await sharesApi.getAcceptedShares()
          setAcceptedShares(sharesData)
        } catch {
          // Non-critical — overlay editor works without shared overlays
        }
      } catch {
        setOverlay(null)
      } finally {
        setIsLoading(false)
      }
    }

    loadData()
  }, [id, token, router])

  // WebSocket listener for share_revoked notifications (real-time update)
  useEffect(() => {
    if (!token || !id) return

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const wsUrl = `${protocol}//${window.location.host}/ws/overlay/${id}?token=${token}`
    const ws = new WebSocket(wsUrl)

    ws.onmessage = (event) => {
      try {
        const envelope = JSON.parse(event.data)
        if (envelope.type === 'share_revoked') {
          const revoker = envelope.data?.revoked_by_username || 'someone'
          toastManager.add({
            title: 'Share revoked',
            description: `Your share with ${revoker} was revoked`,
            type: 'error',
          })
          overlaysApi.getSources(id).then(setSources).catch(console.error)
        }
      } catch {
        // Ignore parse errors
      }
    }

    ws.onerror = () => {
      console.warn('[OverlayEditor] Notification WS error')
    }

    return () => {
      if (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING) {
        ws.close()
      }
    }
  }, [id, token])

  // Handle OAuth callback query params (source_added / error)
  useEffect(() => {
    const sourceAdded = searchParams.get('source_added')
    const error = searchParams.get('error')

    if (sourceAdded) {
      toastManager.add({
        title: 'Source added',
        description: `Successfully added ${sourceAdded} source!`,
        type: 'success',
      })
      overlaysApi.getSources(id).then(setSources).catch(console.error)
      window.history.replaceState({}, '', `/overlays/${id}`)
    } else if (error === 'failed_to_add_source') {
      toastManager.add({
        title: 'Failed to add source',
        description: 'Please try again.',
        type: 'error',
      })
    }
  }, [id, searchParams])

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

  async function handleAddSharedOverlay(share: AcceptedShare) {
    try {
      await overlaysApi.addSource(id, {
        platform: 'shared_overlay',
        channel_id: share.sender_overlay_id,
        channel_name: `${share.sender_display_name}'s overlay`,
      })
      const updated = await overlaysApi.getSources(id)
      setSources(updated)
      toastManager.add({
        title: 'Shared overlay added',
        description: `Added ${share.sender_display_name}'s overlay`,
        type: 'success',
      })
    } catch {
      toastManager.add({
        title: 'Failed to add shared overlay',
        description: 'Please try again.',
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
            >
              Event Settings
            </Button>
            <Button
              variant="outline"
              size="sm"
              onClick={() => router.push(`/overlays/${id}/credits`)}
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
                  <SourceCard
                    key={source.id}
                    source={source}
                    onRemove={handleRemoveSource}
                    onRevoke={setRevokeTarget}
                  />
                ))}
                {sources.length === 0 && (
                  <p className="text-text-sub text-sm py-4">
                    No sources added yet. Add a platform below.
                  </p>
                )}
              </div>
            )}

            {/* Accepted shared overlays — add as source */}
            {acceptedShares.length > 0 && (
              <div className="mb-6 pt-4 border-t border-border">
                <h3 className="text-sm font-medium text-text mb-3">Shared Overlays</h3>
                <div className="space-y-2">
                  {acceptedShares.map(share => (
                    <button
                      key={share.share_id}
                      onClick={() => handleAddSharedOverlay(share)}
                      className="w-full flex items-center justify-between px-3 py-2 text-sm rounded-lg border border-border bg-surface hover:bg-surface-2 transition-colors text-text"
                    >
                      <span>{share.sender_display_name}&apos;s overlay</span>
                      <span className="text-xs text-twitch">+ Add</span>
                    </button>
                  ))}
                </div>
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
            window.location.href = `/api/v1/auth/youtube/add-source/${id}`
          }}
        />
      )}

      {/* Revocation Confirm Modal */}
      {revokeTarget && (
        <RevocationConfirmModal
          partnerName={revokeTarget.channel_name || 'shared overlay'}
          shareId={revokeTarget.channel_id}
          onClose={() => setRevokeTarget(null)}
          onRevoked={() => {
            setRevokeTarget(null)
            overlaysApi.getSources(id).then(setSources).catch(console.error)
          }}
        />
      )}
    </div>
  )
}
