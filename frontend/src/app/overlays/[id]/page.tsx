/**
 * Overlay Editor Page
 *
 * Edit an existing overlay and manage its chat sources.
 * Displays a split-view layout: full config panel left, live embed preview iframe right.
 *
 * Features:
 * - Platform-colored source cards with PlatformBadge
 * - Draggable split-view with live overlay embed preview
 * - Dialog confirmation for source removal
 * - Toast feedback for all actions
 * - Skeleton loading states
 * - Shared overlay sources (v1.3) with revocation support
 * - WebSocket listener for real-time share_revoked notifications
 * - OAuth callback handling (source_added / error query params)
 * - Customization controls (font size, max messages, duration, badges, emotes)
 * - Mock message injection
 * - Custom CSS editor with theme marketplace
 * - Copy OBS URL button
 */

'use client'

import { use, useEffect, useRef, useState } from 'react'
import { useRouter, useSearchParams } from 'next/navigation'
import { ChevronLeft, X, Clipboard, Share2 } from 'lucide-react'
import { useAuthStore } from '@/lib/stores/auth-store'
import { overlaysApi } from '@/lib/api/overlays'
import { sharesApi } from '@/lib/api/shares'
import type { Overlay, ChatSource } from '@/lib/types/overlay'
import type { ChatMessage } from '@/lib/types/message'
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
import dynamic from 'next/dynamic'

// Dynamically import Monaco Editor to avoid SSR issues
const MonacoCSSEditor = dynamic(() => import('@/components/MonacoCSSEditor'), {
  ssr: false,
  loading: () => (
    <div className="h-[300px] bg-surface-2 border border-border rounded-lg flex items-center justify-center">
      <div className="text-text-sub text-sm">Loading editor...</div>
    </div>
  ),
})

// Dynamically import Theme Marketplace Modal
const ThemeMarketplaceModal = dynamic(
  () => import('@/components/theme-marketplace/ThemeMarketplaceModal'),
  { ssr: false },
)

// Static platform border mapping — full literal class strings for Tailwind JIT safety
const PLATFORM_BORDER: Record<string, string> = {
  twitch:         'border-l-twitch',
  youtube:        'border-l-youtube',
  kick:           'border-l-kick',
  tiktok:         'border-l-tiktok',
  shared_overlay: 'border-l-twitch',
}

// ---- Types -----------------------------------------------------------------

type MockMessageFormState = {
  platform: ChatMessage['platform']
  displayName: string
  username: string
  avatarUrl: string
  message: string
  color: string
}

const DEFAULT_MOCK_FORM: MockMessageFormState = {
  platform: 'twitch',
  displayName: 'Overlay Fan',
  username: 'overlayfan',
  avatarUrl: '',
  message: 'This overlay looks great! PogChamp',
  color: '#9146ff',
}

const SAMPLE_MOCK_MESSAGES: Array<Omit<ChatMessage, 'id' | 'timestamp' | 'overlay_id'>> = [
  {
    platform: 'twitch',
    channel_id: 'sample-twitch',
    channel_name: 'Sample Twitch',
    user: {
      id: 'sample-user-1',
      username: 'retro_mod',
      display_name: 'RetroMod',
      avatar_url: 'https://i.pravatar.cc/100?img=13',
      badges: [],
      color: '#fbbf24',
    },
    message: { text: 'Welcome to the overlay preview! PogChamp', emotes: [] },
    metadata: { mock: true },
  },
  {
    platform: 'youtube',
    channel_id: 'sample-youtube',
    channel_name: 'Sample YouTube',
    user: {
      id: 'sample-user-2',
      username: 'cybercritic',
      display_name: 'CyberCritic',
      avatar_url: 'https://i.pravatar.cc/100?img=32',
      badges: [],
      color: '#f87171',
    },
    message: { text: 'Picked up the neon CSS preset and it SLAPS 🔥', emotes: [] },
    metadata: { mock: true },
  },
  {
    platform: 'kick',
    channel_id: 'sample-kick',
    channel_name: 'Sample Kick',
    user: {
      id: 'sample-user-3',
      username: 'emote_master',
      display_name: 'EmoteMaster',
      avatar_url: 'https://i.pravatar.cc/100?img=56',
      badges: [],
      color: '#4ade80',
    },
    message: { text: 'Drop your favorite emotes in chat 😎', emotes: [] },
    metadata: { mock: true },
  },
]

const SAMPLE_EVENT_MESSAGES: Array<Omit<ChatMessage, 'id' | 'timestamp' | 'overlay_id'>> = [
  {
    platform: 'twitch',
    channel_id: 'sample-twitch',
    channel_name: 'Sample Twitch',
    user: {
      id: 'event-user-1',
      username: 'generousviewer',
      display_name: 'GenerousViewer',
      avatar_url: 'https://i.pravatar.cc/100?img=45',
      badges: [],
      color: '#ff6b6b',
    },
    message: { text: 'Love the stream! Keep it up!', emotes: [] },
    event: {
      type: 'subscription',
      tier: 'high',
      duration: 30,
      is_update: false,
      metadata: { sub_tier: '1000', months: 1, streak: 1 },
    },
    metadata: { mock: true, event: true },
  },
  {
    platform: 'youtube',
    channel_id: 'sample-youtube',
    channel_name: 'Sample YouTube',
    user: {
      id: 'event-user-2',
      username: 'superfan',
      display_name: 'SuperFan',
      avatar_url: 'https://i.pravatar.cc/100?img=67',
      badges: [],
      color: '#e91e63',
    },
    message: { text: 'Amazing content! Thanks for all you do!', emotes: [] },
    event: {
      type: 'super_chat',
      tier: 'high',
      value: { amount: 50, currency: 'USD', display_text: '$50.00' },
      duration: 60,
      is_update: false,
      metadata: {},
    },
    metadata: { mock: true, event: true },
  },
  {
    platform: 'twitch',
    channel_id: 'sample-twitch',
    channel_name: 'Sample Twitch',
    user: {
      id: 'event-user-3',
      username: 'bigstreamer',
      display_name: 'BigStreamer',
      avatar_url: 'https://i.pravatar.cc/100?img=23',
      badges: [],
      color: '#9146ff',
    },
    message: { text: 'is raiding with 2,500 viewers!', emotes: [] },
    event: {
      type: 'raid',
      tier: 'high',
      duration: 40,
      is_update: false,
      metadata: { viewer_count: 2500 },
    },
    metadata: { mock: true, event: true },
  },
]

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

// Platform source buttons — OAuth redirect for Twitch/YouTube/Kick,
// text input for TikTok (no OAuth required).
// Admins also get a manual channel ID form for any platform.
function AddSourceForm({
  overlayId,
  token,
  onAddTikTok,
  onYouTubeClick,
  onAddManual,
  isAdmin = false,
}: {
  overlayId: string
  token: string
  onAddTikTok: (username: string) => void
  onYouTubeClick: () => void
  onAddManual?: (platform: string, channelId: string) => void
  isAdmin?: boolean
}) {
  const [tiktokUsername, setTiktokUsername] = useState('')
  const [isAdding, setIsAdding] = useState(false)
  const [adminPlatform, setAdminPlatform] = useState('twitch')

  // Fetch the OAuth auth_url from the backend (with Authorization header),
  // then redirect the browser to it — same pattern as the login flow.
  const startOAuth = async (endpoint: string) => {
    try {
      const res = await fetch(endpoint, {
        headers: { Authorization: `Bearer ${token}` },
      })
      const data = await res.json()
      if (data.auth_url) {
        window.location.href = data.auth_url
      } else {
        console.error('No auth_url returned', data)
      }
    } catch (err) {
      console.error('Failed to initiate OAuth', err)
    }
  }
  const [adminChannelId, setAdminChannelId] = useState('')
  const [isAdminAdding, setIsAdminAdding] = useState(false)

  const handleTikTokSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    const username = tiktokUsername.trim().replace(/^@/, '')
    if (!username) return
    setIsAdding(true)
    try {
      await onAddTikTok(username)
      setTiktokUsername('')
    } finally {
      setIsAdding(false)
    }
  }

  return (
    <div className="space-y-3">
      <p className="text-xs text-text-sub">Add a platform via OAuth or enter a TikTok username directly.</p>

      {/* OAuth buttons — fetch auth_url then redirect, same pattern as login */}
      <div className="grid grid-cols-1 gap-2">
        <button
          onClick={() => startOAuth(`/api/v1/auth/twitch/add-source/${overlayId}`)}
          className="flex items-center gap-2.5 px-4 py-2.5 rounded-lg bg-twitch text-white text-sm font-semibold hover:opacity-90 transition-opacity focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-twitch focus-visible:ring-offset-2 focus-visible:ring-offset-bg"
        >
          <svg className="w-4 h-4 shrink-0" viewBox="0 0 24 24" aria-hidden="true">
            <path fill="#FFFFFF" d="M11.571 4.714h1.715v5.143H11.57zm4.715 0H18v5.143h-1.714zM6 0L1.714 4.286v15.428h5.143V24l4.286-4.286h3.428L22.286 12V0zm14.571 11.143l-3.428 3.428h-3.429l-3 3v-3H6.857V1.714h13.714z" />
          </svg>
          Connect Twitch
        </button>

        <button
          onClick={onYouTubeClick}
          className="flex items-center gap-2.5 px-4 py-2.5 rounded-lg text-white text-sm font-semibold hover:opacity-90 transition-opacity focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:ring-offset-bg"
          style={{ backgroundColor: '#FF0000', '--tw-ring-color': '#FF0000' } as React.CSSProperties}
        >
          <svg className="w-4 h-4 shrink-0" viewBox="0 0 24 24" aria-hidden="true">
            <path fill="#FFFFFF" d="M23.498 6.186a3.016 3.016 0 0 0-2.122-2.136C19.505 3.545 12 3.545 12 3.545s-7.505 0-9.377.505A3.017 3.017 0 0 0 .502 6.186C0 8.07 0 12 0 12s0 3.93.502 5.814a3.016 3.016 0 0 0 2.122 2.136c1.871.505 9.376.505 9.376.505s7.505 0 9.377-.505a3.015 3.015 0 0 0 2.122-2.136C24 15.93 24 12 24 12s0-3.93-.502-5.814zM9.545 15.568V8.432L15.818 12l-6.273 3.568z" />
          </svg>
          Connect YouTube
        </button>

        <button
          onClick={() => startOAuth(`/api/v1/auth/kick/add-source/${overlayId}`)}
          className="flex items-center gap-2.5 px-4 py-2.5 rounded-lg text-bg text-sm font-semibold hover:opacity-90 transition-opacity focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-kick focus-visible:ring-offset-2 focus-visible:ring-offset-bg"
          style={{ backgroundColor: 'var(--color-kick)' }}
        >
          <svg className="w-4 h-4 shrink-0" viewBox="0 0 512 512" aria-hidden="true">
            <path fill="currentColor" d="M37 .036h164.448v113.621h54.71v-56.82h54.731V.036h164.448v170.777h-54.73v56.82h-54.711v56.8h54.71v56.82h54.73V512.03H310.89v-56.82h-54.73v-56.8h-54.711v113.62H37V.036z" />
          </svg>
          Connect Kick
        </button>
      </div>

      {/* TikTok — username only, no OAuth */}
      <form onSubmit={handleTikTokSubmit} className="pt-1 border-t border-border">
        <p className="text-xs text-text-sub mb-2">TikTok (enter username)</p>
        <div className="flex gap-2">
          <Input
            value={tiktokUsername}
            onChange={e => setTiktokUsername(e.target.value)}
            placeholder="@username"
            className="flex-1"
          />
          <Button type="submit" disabled={isAdding || !tiktokUsername.trim()} size="sm">
            Add
          </Button>
        </div>
      </form>

      {/* Admin manual entry — any platform, any channel ID */}
      {isAdmin && onAddManual && (
        <details className="pt-1 border-t border-border">
          <summary className="text-xs text-text-sub cursor-pointer select-none hover:text-text py-1">
            Admin: manual channel ID
          </summary>
          <form
            className="mt-2 space-y-2"
            onSubmit={async e => {
              e.preventDefault()
              if (!adminChannelId.trim()) return
              setIsAdminAdding(true)
              try {
                await onAddManual(adminPlatform, adminChannelId.trim())
                setAdminChannelId('')
              } finally {
                setIsAdminAdding(false)
              }
            }}
          >
            <div className="flex gap-2">
              <select
                value={adminPlatform}
                onChange={e => setAdminPlatform(e.target.value)}
                className="rounded-lg border border-border bg-surface px-2 py-1.5 text-xs text-text focus:outline-none focus:ring-2 focus:ring-twitch"
              >
                <option value="twitch">Twitch</option>
                <option value="youtube">YouTube</option>
                <option value="kick">Kick</option>
                <option value="tiktok">TikTok</option>
              </select>
              <Input
                value={adminChannelId}
                onChange={e => setAdminChannelId(e.target.value)}
                placeholder="Channel ID or username"
                className="flex-1 text-xs"
              />
            </div>
            <Button type="submit" disabled={isAdminAdding || !adminChannelId.trim()} size="sm" variant="outline" className="w-full">
              {isAdminAdding ? 'Adding…' : 'Add manually'}
            </Button>
          </form>
        </details>
      )}
    </div>
  )
}

// ---- Page ------------------------------------------------------------------

export default function OverlayEditorPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params)
  const router = useRouter()
  const searchParams = useSearchParams()
  const { token, user } = useAuthStore()

  // --- Overlay / sources state ---
  const [overlay, setOverlay] = useState<Overlay | null>(null)
  const [sources, setSources] = useState<ChatSource[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [showBetaWarning, setShowBetaWarning] = useState<'youtube' | null>(null)
  const [acceptedShares, setAcceptedShares] = useState<AcceptedShare[]>([])
  const [revokeTarget, setRevokeTarget] = useState<ChatSource | null>(null)

  // --- Customization state ---
  const [fontSize, setFontSize] = useState(16)
  const [maxMessages, setMaxMessages] = useState(50)
  const [messageDuration, setMessageDuration] = useState(15)
  const [disableMessageFade, setDisableMessageFade] = useState(false)
  const [platformBadgePosition, setPlatformBadgePosition] = useState<'before' | 'after'>('before')
  const [platformBadgeStyle, setPlatformBadgeStyle] = useState<'text' | 'icon'>('text')
  const [showPlatformBadge, setShowPlatformBadge] = useState(true)
  const [layout, setLayout] = useState<'vertical' | 'ticker'>('vertical')
  const [tickerSpeed, setTickerSpeed] = useState(150)
  const [configLoaded, setConfigLoaded] = useState(false)
  const [isSavingConfig, setIsSavingConfig] = useState(false)
  const [configAlert, setConfigAlert] = useState<{ type: 'success' | 'error'; message: string } | null>(null)

  // --- Mock messages state ---
  const [mockForm, setMockForm] = useState<MockMessageFormState>(DEFAULT_MOCK_FORM)

  // --- Custom CSS state ---
  const [customCss, setCustomCss] = useState('')
  const [useCustomCss, setUseCustomCss] = useState(false)
  const [showThemeMarketplace, setShowThemeMarketplace] = useState(false)

  // --- OBS URL copy state ---
  const [copiedObs, setCopiedObs] = useState(false)

  // --- Share overlay state ---
  const [showShareModal, setShowShareModal] = useState(false)
  const [showPremiumRequired, setShowPremiumRequired] = useState(false)
  const [shareRecipient, setShareRecipient] = useState('')
  const [shareLoading, setShareLoading] = useState(false)
  const shareInputRef = useRef<HTMLInputElement>(null)

  // Load overlay, sources, accepted shares and config
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

        // Load accepted shares — non-critical
        try {
          const sharesData = await sharesApi.getAcceptedShares()
          setAcceptedShares(sharesData)
        } catch {
          // Non-critical
        }

        // Load overlay config for customization defaults
        try {
          const config = await overlaysApi.getConfig(id)
          const display = config.display_settings || {}

          if (typeof display.max_messages === 'number') setMaxMessages(display.max_messages)
          if (typeof display.font_size === 'number') setFontSize(display.font_size)
          if (typeof display.message_duration === 'number') setMessageDuration(display.message_duration)
          if (typeof display.disable_message_fade === 'boolean') setDisableMessageFade(display.disable_message_fade)
          if (display.platform_badge_position === 'before' || display.platform_badge_position === 'after') {
            setPlatformBadgePosition(display.platform_badge_position)
          }
          if (display.platform_badge_style === 'text' || display.platform_badge_style === 'icon') {
            setPlatformBadgeStyle(display.platform_badge_style)
          }
          if (typeof display.show_platform_badge === 'boolean') setShowPlatformBadge(display.show_platform_badge)
          if (display.layout === 'vertical' || display.layout === 'ticker') setLayout(display.layout)
          if (typeof display.ticker_speed === 'number' && display.ticker_speed > 0) setTickerSpeed(display.ticker_speed)

          const css = config.custom_css || ''
          setCustomCss(css)
          setUseCustomCss(Boolean(css.trim().length))
        } catch (err) {
          console.warn('[OverlayEditor] Failed to load config', err)
        } finally {
          setConfigLoaded(true)
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

  // --- Source handlers ---

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

  async function handleAddTikTokSource(username: string) {
    try {
      const source = await overlaysApi.addSource(id, {
        platform: 'tiktok',
        channel_id: username,
      })
      setSources(prev => [...prev, source])
      toastManager.add({ title: 'TikTok source added', type: 'success' })
    } catch {
      toastManager.add({
        title: 'Failed to add TikTok source',
        description: 'Check the username and try again.',
        type: 'error',
      })
    }
  }

  async function handleAddManual(platform: string, channelId: string) {
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

  // --- OBS URL ---

  function handleCopyObsUrl() {
    const url = `${window.location.origin}/overlay/${id}`
    navigator.clipboard.writeText(url).then(() => {
      setCopiedObs(true)
      setTimeout(() => setCopiedObs(false), 2000)
    })
  }

  function handleShareClick() {
    if (!user?.is_premium) {
      setShowPremiumRequired(true)
    } else {
      setShareRecipient('')
      setShowShareModal(true)
      setTimeout(() => shareInputRef.current?.focus(), 50)
    }
  }

  async function handleSendShareRequest() {
    const username = shareRecipient.trim()
    if (!username) return
    setShareLoading(true)
    try {
      await sharesApi.createRequest(username, id)
      toastManager.add({ title: `Share request sent to ${username}`, type: 'success' })
      setShowShareModal(false)
      setShareRecipient('')
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to send share request'
      toastManager.add({ title: msg, type: 'error' })
    } finally {
      setShareLoading(false)
    }
  }

  // --- Mock message handlers ---

  function handleMockInputChange<K extends keyof MockMessageFormState>(
    field: K,
    value: MockMessageFormState[K],
  ) {
    setMockForm(prev => ({ ...prev, [field]: value }))
  }

  const resolveMockTarget = (requestedPlatform?: ChatMessage['platform']) => {
    const preferred = sources.find(source =>
      requestedPlatform ? source.platform === requestedPlatform : true,
    )
    if (!preferred) {
      return {
        platform: requestedPlatform || 'twitch',
        channel_id: undefined,
        channel_name: undefined,
      }
    }
    return {
      platform: requestedPlatform || (preferred.platform as ChatMessage['platform']),
      channel_id: preferred.channel_id,
      channel_name: preferred.channel_name || preferred.channel_id,
    }
  }

  async function handleAddMockMessage() {
    if (!mockForm.message.trim()) return
    const target = resolveMockTarget(mockForm.platform)
    try {
      await overlaysApi.sendMockMessage(id, {
        platform: target.platform,
        channel_id: target.channel_id,
        channel_name: target.channel_name,
        text: mockForm.message,
        username:
          mockForm.username ||
          mockForm.displayName.toLowerCase().replace(/\s+/g, '') ||
          'mockuser',
        display_name: mockForm.displayName || mockForm.username || 'Mock Viewer',
        avatar_url: mockForm.avatarUrl || undefined,
        color: mockForm.color || undefined,
        metadata: { mock: true, source: 'editor-form' },
      })
      setMockForm(prev => ({ ...prev, message: '' }))
    } catch (error) {
      console.error('[Editor] Failed to send mock message:', error)
      toastManager.add({ title: 'Failed to send mock message', type: 'error' })
    }
  }

  async function handleAddSampleTranscript() {
    for (const [index, sample] of SAMPLE_MOCK_MESSAGES.entries()) {
      const target = resolveMockTarget(sample.platform)
      try {
        await overlaysApi.sendMockMessage(id, {
          platform: target.platform,
          channel_id: target.channel_id,
          channel_name: target.channel_name,
          text: sample.message.text,
          username: sample.user.username,
          display_name: sample.user.display_name,
          avatar_url: sample.user.avatar_url,
          color: sample.user.color,
          badges: sample.user.badges,
          metadata: { ...(sample.metadata || {}), mock: true, preset: true, order: index },
        })
      } catch (error) {
        console.error('[Editor] Failed to send sample message:', error)
        break
      }
    }
  }

  async function handleAddSampleEvents() {
    for (const [index, sample] of SAMPLE_EVENT_MESSAGES.entries()) {
      const target = resolveMockTarget(sample.platform)
      try {
        await overlaysApi.sendMockMessage(id, {
          platform: target.platform,
          channel_id: target.channel_id,
          channel_name: target.channel_name,
          text: sample.message.text,
          username: sample.user.username,
          display_name: sample.user.display_name,
          avatar_url: sample.user.avatar_url,
          color: sample.user.color,
          badges: sample.user.badges,
          event: sample.event,
          metadata: { ...(sample.metadata || {}), mock: true, preset: true, order: index },
        })
        await new Promise(resolve => setTimeout(resolve, 800))
      } catch (error) {
        console.error('[Editor] Failed to send sample event:', error)
        break
      }
    }
  }

  // --- Save configuration ---

  async function handleSaveConfiguration() {
    setIsSavingConfig(true)
    setConfigAlert(null)
    try {
      await overlaysApi.updateConfig(id, {
        display_settings: {
          font_size: fontSize,
          message_duration: messageDuration,
          max_messages: maxMessages,
          disable_message_fade: disableMessageFade,
          platform_badge_position: platformBadgePosition,
          platform_badge_style: platformBadgeStyle,
          show_platform_badge: showPlatformBadge,
          layout,
          ticker_speed: tickerSpeed,
        },
        custom_css: useCustomCss ? customCss : '',
      })
      setConfigAlert({ type: 'success', message: 'Configuration saved!' })
    } catch (error) {
      console.error('[Editor] Failed to save config', error)
      setConfigAlert({ type: 'error', message: 'Failed to save configuration' })
    } finally {
      setIsSavingConfig(false)
      setTimeout(() => setConfigAlert(null), 5000)
    }
  }

  // --- Loading / error states ---

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
        <div className="p-6 max-w-none space-y-6">

          {/* 1. Header */}
          <div className="flex items-start justify-between">
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
            <div className="flex flex-wrap gap-2 shrink-0 ml-4">
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
          </div>

          {/* 2. Copy OBS URL */}
          <Button
            variant="outline"
            className="w-full flex items-center gap-2 justify-center"
            onClick={handleCopyObsUrl}
          >
            <Clipboard className="size-4" />
            {copiedObs ? 'Copied!' : 'Copy OBS URL'}
          </Button>

          {/* 2b. Share overlay */}
          <Button
            variant="outline"
            className="w-full flex items-center gap-2 justify-center"
            onClick={handleShareClick}
          >
            <Share2 className="size-4" />
            Share Overlay
          </Button>

          {/* Premium required dialog */}
          {showPremiumRequired && (
            <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50" onClick={() => setShowPremiumRequired(false)}>
              <div className="bg-surface border border-border rounded-xl p-6 max-w-sm w-full mx-4 shadow-xl" onClick={e => e.stopPropagation()}>
                <div className="flex items-start justify-between mb-3">
                  <h2 className="text-lg font-semibold text-text">Premium Feature</h2>
                  <button onClick={() => setShowPremiumRequired(false)} className="text-text-sub hover:text-text"><X className="size-4" /></button>
                </div>
                <p className="text-sm text-text-sub mb-4">
                  Sharing your overlay is a premium feature. Upgrade your account to share your chat with other streamers.
                </p>
                <p className="text-sm text-text-sub mb-5">
                  For more information and to get access, join our Discord community.
                </p>
                <div className="flex gap-2">
                  <Button variant="outline" className="flex-1" onClick={() => setShowPremiumRequired(false)}>Close</Button>
                  <a
                    href="https://discord.gg/xCGBSuz39P"
                    target="_blank"
                    rel="noopener noreferrer"
                    className="flex-1"
                  >
                    <Button className="w-full">Join Discord</Button>
                  </a>
                </div>
              </div>
            </div>
          )}

          {/* Share overlay modal */}
          {showShareModal && (
            <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50" onClick={() => setShowShareModal(false)}>
              <div className="bg-surface border border-border rounded-xl p-6 max-w-sm w-full mx-4 shadow-xl" onClick={e => e.stopPropagation()}>
                <div className="flex items-start justify-between mb-3">
                  <h2 className="text-lg font-semibold text-text">Share Overlay</h2>
                  <button onClick={() => setShowShareModal(false)} className="text-text-sub hover:text-text"><X className="size-4" /></button>
                </div>
                <p className="text-sm text-text-sub mb-4">
                  Enter the Twitch username of the person you want to share <strong>{overlay?.name}</strong> with. They&apos;ll receive a request they can accept or decline.
                </p>
                <div className="mb-4">
                  <label className="text-xs text-text-sub mb-1 block">Twitch username</label>
                  <input
                    ref={shareInputRef}
                    type="text"
                    value={shareRecipient}
                    onChange={e => setShareRecipient(e.target.value)}
                    onKeyDown={e => e.key === 'Enter' && handleSendShareRequest()}
                    placeholder="e.g. somestreamer"
                    className="w-full px-3 py-2 text-sm rounded-lg border border-border bg-surface-2 text-text placeholder:text-text-sub focus:outline-none focus:ring-2 focus:ring-twitch/50"
                    disabled={shareLoading}
                  />
                </div>
                <div className="flex gap-2">
                  <Button variant="outline" className="flex-1" onClick={() => setShowShareModal(false)} disabled={shareLoading}>Cancel</Button>
                  <Button
                    className="flex-1"
                    onClick={handleSendShareRequest}
                    disabled={shareLoading || !shareRecipient.trim()}
                  >
                    {shareLoading ? 'Sending...' : 'Send Request'}
                  </Button>
                </div>
              </div>
            </div>
          )}

          {/* 3. Sources section */}
          <section aria-label="Chat sources">
            <h2 className="text-sm font-semibold text-text mb-3">Sources</h2>

            {isLoading ? (
              <SourceListSkeleton />
            ) : (
              <div className="space-y-3 mb-4">
                {sources.map(source => (
                  <SourceCard
                    key={source.id}
                    source={source}
                    onRemove={handleRemoveSource}
                    onRevoke={setRevokeTarget}
                  />
                ))}
                {sources.length === 0 && (
                  <p className="text-text-sub text-sm py-2">
                    No sources added yet. Add a platform below.
                  </p>
                )}
              </div>
            )}

            {/* Accepted shared overlays — add as source */}
            {acceptedShares.length > 0 && (
              <div className="mb-4 pt-4 border-t border-border">
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

            {/* Add source */}
            <AddSourceForm
              overlayId={id}
              token={token ?? ''}
              onAddTikTok={handleAddTikTokSource}
              onYouTubeClick={() => setShowBetaWarning('youtube')}
              onAddManual={handleAddManual}
              isAdmin={user?.is_admin === true}
            />
          </section>

          {/* 4. Customization section */}
          <Card className="p-4">
            <h2 className="text-sm font-semibold text-text mb-4">Customization</h2>
            <div className="space-y-5">

              {/* Font Size */}
              <div>
                <label className="block text-xs text-text-sub mb-1">
                  Font Size: <span className="text-twitch">{fontSize}px</span>
                </label>
                <input
                  type="range"
                  min="12"
                  max="32"
                  value={fontSize}
                  onChange={e => setFontSize(parseInt(e.target.value))}
                  className="w-full accent-twitch"
                />
              </div>

              {/* Max Messages */}
              <div>
                <label className="block text-xs text-text-sub mb-1">
                  Max Messages: <span className="text-twitch">{maxMessages}</span>
                </label>
                <input
                  type="range"
                  min="10"
                  max="100"
                  value={maxMessages}
                  onChange={e => setMaxMessages(parseInt(e.target.value))}
                  className="w-full accent-twitch"
                />
              </div>

              {/* Message Duration */}
              <div>
                <label className="block text-xs text-text-sub mb-1">
                  Message Duration: <span className="text-twitch">{messageDuration}s</span>
                </label>
                <input
                  type="range"
                  min="5"
                  max="60"
                  value={messageDuration}
                  onChange={e => setMessageDuration(parseInt(e.target.value))}
                  className="w-full accent-twitch"
                  disabled={disableMessageFade}
                />
              </div>

              {/* Disable fade */}
              <div>
                <label className="flex items-center gap-2 text-xs text-text-sub">
                  <input
                    type="checkbox"
                    checked={disableMessageFade}
                    onChange={e => setDisableMessageFade(e.target.checked)}
                    className="accent-twitch"
                  />
                  Disable Message Fade Out
                </label>
                <p className="text-xs text-text-sub mt-1 ml-5">
                  Messages stay visible until max is reached
                </p>
              </div>

              {/* Layout */}
              <div>
                <p className="text-xs text-text-sub mb-2">Layout</p>
                <div className="flex gap-4">
                  <label className="flex items-center gap-1.5 text-xs text-text-sub cursor-pointer">
                    <input
                      type="radio"
                      name="layout"
                      value="vertical"
                      checked={layout === 'vertical'}
                      onChange={() => setLayout('vertical')}
                      className="accent-twitch"
                    />
                    Vertical (default)
                  </label>
                  <label className="flex items-center gap-1.5 text-xs text-text-sub cursor-pointer">
                    <input
                      type="radio"
                      name="layout"
                      value="ticker"
                      checked={layout === 'ticker'}
                      onChange={() => setLayout('ticker')}
                      className="accent-twitch"
                    />
                    Ticker (horizontal)
                  </label>
                </div>
              </div>

              {/* Ticker Speed — only shown when ticker layout is active */}
              {layout === 'ticker' && (
                <div>
                  <label className="block text-xs text-text-sub mb-1">
                    Ticker Speed: <span className="text-twitch">{tickerSpeed}px/s</span>
                  </label>
                  <input
                    type="range"
                    min="50"
                    max="400"
                    step="10"
                    value={tickerSpeed}
                    onChange={e => setTickerSpeed(parseInt(e.target.value))}
                    className="w-full accent-twitch"
                  />
                  <p className="text-xs text-text-sub mt-1">
                    How fast messages scroll across the screen
                  </p>
                </div>
              )}

              {/* Platform Badge */}
              <div>
                <p className="text-xs text-text-sub mb-2">Platform Badge</p>
                <div className="space-y-2">
                  <label className="flex items-center gap-2 text-xs text-text-sub">
                    <input
                      type="checkbox"
                      checked={showPlatformBadge}
                      onChange={e => setShowPlatformBadge(e.target.checked)}
                      className="accent-twitch"
                    />
                    Show Platform Badge
                  </label>
                  <div className={cn('space-y-2', !showPlatformBadge && 'opacity-50 pointer-events-none')}>
                    <div>
                      <p className="text-xs text-text-sub mb-1">Position</p>
                      <div className="flex gap-4">
                        <label className="flex items-center gap-1.5 text-xs text-text-sub cursor-pointer">
                          <input
                            type="radio"
                            name="platformBadgePosition"
                            value="before"
                            checked={platformBadgePosition === 'before'}
                            onChange={() => setPlatformBadgePosition('before')}
                            className="accent-twitch"
                            disabled={!showPlatformBadge}
                          />
                          Before
                        </label>
                        <label className="flex items-center gap-1.5 text-xs text-text-sub cursor-pointer">
                          <input
                            type="radio"
                            name="platformBadgePosition"
                            value="after"
                            checked={platformBadgePosition === 'after'}
                            onChange={() => setPlatformBadgePosition('after')}
                            className="accent-twitch"
                            disabled={!showPlatformBadge}
                          />
                          After
                        </label>
                      </div>
                    </div>
                    <div>
                      <p className="text-xs text-text-sub mb-1">Style</p>
                      <div className="flex gap-4">
                        <label className="flex items-center gap-1.5 text-xs text-text-sub cursor-pointer">
                          <input
                            type="radio"
                            name="platformBadgeStyle"
                            value="text"
                            checked={platformBadgeStyle === 'text'}
                            onChange={() => setPlatformBadgeStyle('text')}
                            className="accent-twitch"
                            disabled={!showPlatformBadge}
                          />
                          Text
                        </label>
                        <label className="flex items-center gap-1.5 text-xs text-text-sub cursor-pointer">
                          <input
                            type="radio"
                            name="platformBadgeStyle"
                            value="icon"
                            checked={platformBadgeStyle === 'icon'}
                            onChange={() => setPlatformBadgeStyle('icon')}
                            className="accent-twitch"
                            disabled={!showPlatformBadge}
                          />
                          Icon
                        </label>
                      </div>
                    </div>
                  </div>
                </div>
              </div>

              {/* Emote Providers */}
              <div>
                <p className="text-xs text-text-sub mb-2">Emote Providers</p>
                <div className="space-y-1.5">
                  <label className="flex items-center gap-2 text-xs text-text-sub">
                    <input type="checkbox" defaultChecked className="accent-twitch" />
                    7TV
                  </label>
                  <label className="flex items-center gap-2 text-xs text-text-sub">
                    <input type="checkbox" defaultChecked className="accent-twitch" />
                    BetterTTV
                  </label>
                  <label className="flex items-center gap-2 text-xs text-text-sub">
                    <input type="checkbox" defaultChecked className="accent-twitch" />
                    FrankerFaceZ
                  </label>
                </div>
              </div>
            </div>
          </Card>

          {/* 5. Mock Messages section */}
          <Card className="p-4">
            <div className="flex items-center justify-between mb-3">
              <h2 className="text-sm font-semibold text-text">Mock Messages</h2>
            </div>
            <div className="space-y-3">
              <div>
                <label className="block text-xs text-text-sub mb-1">Platform</label>
                <select
                  value={mockForm.platform}
                  onChange={e => handleMockInputChange('platform', e.target.value as MockMessageFormState['platform'])}
                  className="w-full rounded-lg border border-border bg-surface px-2 py-2 text-sm text-text focus:outline-none focus:ring-2 focus:ring-twitch"
                >
                  <option value="twitch">Twitch</option>
                  <option value="youtube">YouTube</option>
                  <option value="kick">Kick</option>
                  <option value="tiktok">TikTok</option>
                </select>
              </div>
              <div className="grid grid-cols-2 gap-2">
                <div>
                  <label className="block text-xs text-text-sub mb-1">Display Name</label>
                  <input
                    type="text"
                    value={mockForm.displayName}
                    onChange={e => handleMockInputChange('displayName', e.target.value)}
                    className="w-full rounded-lg border border-border bg-surface px-2 py-2 text-sm text-text focus:outline-none focus:ring-2 focus:ring-twitch"
                  />
                </div>
                <div>
                  <label className="block text-xs text-text-sub mb-1">Username</label>
                  <input
                    type="text"
                    value={mockForm.username}
                    onChange={e => handleMockInputChange('username', e.target.value)}
                    className="w-full rounded-lg border border-border bg-surface px-2 py-2 text-sm text-text focus:outline-none focus:ring-2 focus:ring-twitch"
                  />
                </div>
              </div>
              <div className="grid grid-cols-2 gap-2">
                <div>
                  <label className="block text-xs text-text-sub mb-1">Avatar URL</label>
                  <input
                    type="text"
                    value={mockForm.avatarUrl}
                    onChange={e => handleMockInputChange('avatarUrl', e.target.value)}
                    placeholder="https://..."
                    className="w-full rounded-lg border border-border bg-surface px-2 py-2 text-sm text-text focus:outline-none focus:ring-2 focus:ring-twitch"
                  />
                </div>
                <div>
                  <label className="block text-xs text-text-sub mb-1">Name Color</label>
                  <input
                    type="color"
                    value={mockForm.color}
                    onChange={e => handleMockInputChange('color', e.target.value)}
                    className="w-full rounded-lg border border-border bg-surface px-2 py-1.5 h-9"
                  />
                </div>
              </div>
              <div>
                <label className="block text-xs text-text-sub mb-1">Message</label>
                <textarea
                  value={mockForm.message}
                  onChange={e => handleMockInputChange('message', e.target.value)}
                  className="w-full rounded-lg border border-border bg-surface px-2 py-2 text-sm text-text h-16 focus:outline-none focus:ring-2 focus:ring-twitch resize-none"
                  placeholder="Type something fun..."
                />
              </div>
              <Button
                type="button"
                onClick={() => void handleAddMockMessage()}
                disabled={!mockForm.message.trim()}
                className="w-full"
              >
                Inject Message
              </Button>
              <div className="flex gap-2">
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  className="flex-1 text-xs"
                  onClick={() => void handleAddSampleTranscript()}
                >
                  💬 Sample Chat
                </Button>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  className="flex-1 text-xs border-yellow-600/40 text-yellow-400 hover:bg-yellow-900/20"
                  onClick={() => void handleAddSampleEvents()}
                >
                  ⭐ Sample Events
                </Button>
              </div>
            </div>
          </Card>

          {/* 6. Custom CSS section */}
          <Card className="p-4">
            <div className="flex items-center justify-between mb-3">
              <div className="flex items-center gap-3">
                <h2 className="text-sm font-semibold text-text">Custom CSS</h2>
                <label className="flex items-center gap-2 text-xs text-text-sub cursor-pointer">
                  <input
                    type="checkbox"
                    checked={useCustomCss}
                    onChange={e => setUseCustomCss(e.target.checked)}
                    className="accent-twitch"
                  />
                  Enable
                </label>
              </div>
              <div className="flex gap-2">
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  className="text-xs"
                  onClick={() => setShowThemeMarketplace(true)}
                >
                  Browse Themes
                </Button>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  className="text-xs"
                  onClick={() => { setCustomCss(''); setUseCustomCss(false) }}
                >
                  Reset
                </Button>
              </div>
            </div>

            <MonacoCSSEditor
              value={customCss}
              onChange={setCustomCss}
              height="300px"
              placeholder="/* Enter your custom CSS here */"
            />

            <p className="text-xs text-text-sub mt-3">
              Need inspiration? Explore{' '}
              <a
                href="https://github.com/caesarakalaeii/all-chat/tree/main/docs/overlay-themes"
                target="_blank"
                rel="noreferrer"
                className="text-twitch hover:underline"
              >
                theme docs
              </a>
              .
            </p>
          </Card>

          {/* Save Configuration */}
          <div className="space-y-2 pb-6">
            <Button
              onClick={() => void handleSaveConfiguration()}
              disabled={!configLoaded || isSavingConfig}
              className="w-full"
            >
              {isSavingConfig ? 'Saving...' : 'Save Configuration'}
            </Button>
            {configAlert && (
              <p className={cn('text-sm text-center', configAlert.type === 'success' ? 'text-green-400' : 'text-destructive')}>
                {configAlert.message}
              </p>
            )}
          </div>

        </div>
      </SplitView>

      {/* Beta Warning Modal */}
      {showBetaWarning && (
        <BetaWarning
          platform={showBetaWarning}
          onCancel={() => setShowBetaWarning(null)}
          onContinue={async () => {
            setShowBetaWarning(null)
            try {
              const res = await fetch(`/api/v1/auth/youtube/add-source/${id}`, {
                headers: { Authorization: `Bearer ${token}` },
              })
              const data = await res.json()
              if (data.auth_url) window.location.href = data.auth_url
            } catch (err) {
              toastManager.add({ title: 'Failed to start YouTube OAuth', type: 'error' })
            }
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

      {/* Theme Marketplace Modal */}
      <ThemeMarketplaceModal
        isOpen={showThemeMarketplace}
        onClose={() => setShowThemeMarketplace(false)}
        onApplyTheme={(css) => {
          setCustomCss(css)
          setUseCustomCss(true)
          setShowThemeMarketplace(false)
        }}
      />
    </div>
  )
}
