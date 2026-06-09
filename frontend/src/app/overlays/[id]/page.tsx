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

import { use, useCallback, useEffect, useRef, useState } from 'react'
import { useRouter, useSearchParams } from 'next/navigation'
import Link from 'next/link'
import { ChevronLeft, ChevronRight, X, Clipboard, Share2, Puzzle } from 'lucide-react'
import { useAuthStore } from '@/lib/stores/auth-store'
import { overlaysApi } from '@/lib/api/overlays'
import type { TTSConfigMetadata, ElevenLabsVoice, TestKeyResult } from '@/lib/api/overlays'
import { sharesApi } from '@/lib/api/shares'
import { getGuilds, getGuildChannels, updateSourceConfig } from '@/lib/api/discord'
import type { DiscordGuild, ChannelCategory } from '@/lib/api/discord'
import type { Overlay, ChatSource, DiscordSourceConfig, FilterSettings, DisplaySettings } from '@/lib/types/overlay'
import type { ChatMessage } from '@/lib/types/message'
import type { AcceptedShare } from '@/lib/types/share'
import type { VisualSettings } from '@/lib/types/visual-settings'
import { visualSettingsToCss } from '@/lib/utils/visual-settings-to-css'
import { parseCssToVisualSettings } from '@/lib/utils/theme-css-parser'
import { toastManager } from '@/lib/toast'
import { trackEvent } from '@/lib/analytics'
import { AppNav } from '@/components/AppNav'
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
import { AppearancePanel } from '@/components/appearance/AppearancePanel'
import { CollapsibleSection } from '@/components/appearance/CollapsibleSection'
import dynamic from 'next/dynamic'

// Dynamically import Monaco Editor to avoid SSR issues
const MonacoCSSEditor = dynamic(() => import('@/components/MonacoCSSEditor'), {
  ssr: false,
  loading: () => (
    <div className="flex h-[300px] items-center justify-center rounded-lg border border-border bg-surface-2">
      <div className="text-sm text-text-sub">Loading editor...</div>
    </div>
  ),
})

// Dynamically import ThemeContent for inline Theme section (SSR unsafe — uses hooks)
const ThemeContent = dynamic(
  () => import('@/components/theme-marketplace/ThemeContent').then((m) => ({ default: m.ThemeContent })),
  { ssr: false }
)

// Static platform border mapping — full literal class strings for Tailwind JIT safety
const PLATFORM_BORDER: Record<string, string> = {
  twitch: 'border-l-twitch',
  youtube: 'border-l-youtube',
  kick: 'border-l-kick',
  tiktok: 'border-l-tiktok',
  shared_overlay: 'border-l-twitch',
  discord: 'border-l-discord',
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
  onConfigureRelay,
  onConfigureStreamSelect,
  isOwnChannel,
  onReconnectChat,
}: {
  source: ChatSource
  onRemove: (id: string) => void
  onRevoke?: (source: ChatSource) => void
  onConfigureRelay?: (source: ChatSource) => void
  onConfigureStreamSelect?: (source: ChatSource) => void
  // isOwnChannel is true when this Twitch source belongs to the logged-in user's own
  // channel (only then can they grant the chat scopes); onReconnectChat starts the
  // add-source OAuth reflow that requests them.
  isOwnChannel?: boolean
  onReconnectChat?: () => void
}) {
  const [confirmOpen, setConfirmOpen] = useState(false)
  const isTwitch = source.platform === 'twitch'
  const isShared = source.platform === 'shared_overlay'
  const isInactiveShared = isShared && !source.is_active
  const isDiscord = source.platform === 'discord'
  const isYoutube = source.platform === 'youtube'

  const discordConfig = isDiscord ? (source.config as DiscordSourceConfig) : null
  const discordLabel =
    isDiscord && discordConfig
      ? `${(source.config as DiscordSourceConfig & { guild_name?: string }).guild_name ?? ''} › #${source.channel_name ?? source.channel_id}`
      : null

  return (
    <Card
      className={cn(
        'border-l-2 p-4',
        PLATFORM_BORDER[source.platform] ?? 'border-l-border',
        isInactiveShared && 'opacity-50'
      )}
    >
      <div className="flex items-center justify-between">
        <div className="flex min-w-0 items-center gap-3">
          <PlatformBadge
            platform={source.platform as 'twitch' | 'youtube' | 'kick' | 'tiktok'}
            size="sm"
          />
          <div className="min-w-0">
            <span className="truncate text-sm font-medium text-text">
              {discordLabel ?? source.channel_name ?? source.channel_id}
            </span>
            {isDiscord && (
              <div className="mt-1 flex flex-wrap items-center gap-2">
                {/* Connection status badge */}
                <span
                  className={cn(
                    'inline-flex items-center gap-1 rounded-full border px-2 py-0.5 text-xs font-medium',
                    source.is_active
                      ? 'border-green-500/20 bg-green-500/10 text-green-400'
                      : 'border-border bg-surface-2/40 text-text-sub'
                  )}
                >
                  <span
                    className={cn(
                      'size-1.5 rounded-full',
                      source.is_active ? 'bg-green-400' : 'bg-text-sub'
                    )}
                  />
                  {source.is_active ? 'Connected' : 'Disconnected'}
                </span>
                {/* Relay ON/OFF badge */}
                {discordConfig && (
                  <span
                    className={cn(
                      'inline-flex items-center rounded-full border px-2 py-0.5 text-xs font-medium',
                      discordConfig.relay_enabled
                        ? 'border-green-600/30 bg-green-600/20 text-green-300'
                        : 'border-border bg-surface-2 text-text-sub'
                    )}
                  >
                    {discordConfig.relay_enabled ? 'Relay ON' : 'Relay OFF'}
                  </span>
                )}
              </div>
            )}
            {isInactiveShared && source.share_status && (
              <StatusBadge status={source.share_status} size="sm" />
            )}
            {/* Twitch: surface whether chat is read via EventSub, and offer a reconnect
                CTA on the owner's own channel when it is not yet enabled. */}
            {isTwitch && source.chat_via_eventsub && (
              <div className="mt-1">
                <span className="inline-flex items-center gap-1 rounded-full border border-green-500/20 bg-green-500/10 px-2 py-0.5 text-xs font-medium text-green-400">
                  <span className="size-1.5 rounded-full bg-green-400" />
                  Chat via EventSub
                </span>
              </div>
            )}
            {isTwitch && !source.chat_via_eventsub && isOwnChannel && onReconnectChat && (
              <button
                onClick={onReconnectChat}
                className="mt-1 inline-flex items-center gap-1 rounded text-xs text-text-sub underline-offset-2 transition-colors hover:text-text hover:underline focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
              >
                Reconnect to enable chat
              </button>
            )}
          </div>
        </div>
        <div className="ml-3 flex shrink-0 items-center gap-2">
          {isShared && source.is_active && onRevoke && (
            <Button
              variant="outline"
              size="sm"
              className="text-destructive border-destructive/40 hover:bg-destructive/10 text-xs"
              onClick={() => onRevoke(source)}
            >
              Revoke
            </Button>
          )}
          <Dialog.Root open={confirmOpen} onOpenChange={setConfirmOpen}>
            <Dialog.Trigger
              render={
                <button
                  className="hover:text-destructive rounded text-text-sub transition-colors focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
                  aria-label={`Remove ${source.channel_name || source.channel_id}`}
                >
                  <X className="size-4" />
                </button>
              }
            />
            <Dialog.Content>
              <Dialog.Title>Remove source?</Dialog.Title>
              <Dialog.Description>
                Remove <strong>{source.channel_name || source.channel_id}</strong> from this
                overlay. Chat messages from this source will stop appearing.
              </Dialog.Description>
              <div className="flex justify-end gap-3 pt-2">
                <Dialog.Close render={<Button variant="outline">Cancel</Button>} />
                <Button
                  variant="destructive"
                  onClick={() => {
                    setConfirmOpen(false)
                    onRemove(source.id)
                  }}
                >
                  Remove
                </Button>
              </div>
            </Dialog.Content>
          </Dialog.Root>
        </div>
      </div>
      {/* Configure relay button — Discord only */}
      {isDiscord && onConfigureRelay && (
        <button
          className="mt-2 flex items-center gap-1 rounded text-xs text-text-sub transition-colors hover:text-text focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
          onClick={() => onConfigureRelay(source)}
        >
          <ChevronRight className="size-3" />
          Configure relay
        </button>
      )}
      {/* Stream selection button — YouTube only */}
      {isYoutube && onConfigureStreamSelect && (
        <button
          className="mt-2 flex items-center gap-1 rounded text-xs text-text-sub transition-colors hover:text-text focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
          onClick={() => onConfigureStreamSelect(source)}
        >
          <ChevronRight className="size-3" />
          Stream selection
        </button>
      )}
    </Card>
  )
}

// ---- StreamSelectionPanel ---------------------------------------------------

const STREAM_STRATEGIES = [
  { value: 'first_found', label: 'First found', description: 'Picks the first live stream (default)' },
  { value: 'most_viewers', label: 'Most viewers', description: 'Picks the stream with the highest viewer count' },
  { value: 'fewest_viewers', label: 'Fewest viewers', description: 'Picks the stream with the lowest viewer count' },
  { value: 'title_match', label: 'Title match', description: 'Picks the first stream whose title contains a keyword' },
  { value: 'title_match_all', label: 'Title match (all)', description: 'Monitors all streams whose title contains a keyword' },
  { value: 'all', label: 'All streams', description: 'Monitors all concurrent live streams simultaneously' },
] as const

function StreamSelectionPanel({
  source,
  overlayId,
  isPremium,
  onSaved,
}: {
  source: ChatSource
  overlayId: string
  isPremium: boolean
  onSaved: () => void
}) {
  const ytConfig = (source.config ?? {}) as import('@/lib/types/overlay').YouTubeSourceConfig
  const [strategy, setStrategy] = useState(ytConfig.stream_select ?? 'first_found')
  const [matchTerm, setMatchTerm] = useState(ytConfig.stream_match ?? '')
  const [saving, setSaving] = useState(false)

  const hasChanges =
    strategy !== (ytConfig.stream_select ?? 'first_found') ||
    matchTerm !== (ytConfig.stream_match ?? '')

  async function handleSave() {
    setSaving(true)
    try {
      const needsMatch = strategy === 'title_match' || strategy === 'title_match_all'
      const config: Record<string, unknown> = {
        ...source.config,
        stream_select: strategy === 'first_found' ? undefined : strategy,
        stream_match: needsMatch ? matchTerm : undefined,
      }
      // Clean undefined keys
      Object.keys(config).forEach((k) => config[k] === undefined && delete config[k])
      await overlaysApi.updateSourceConfig(overlayId, source.id, config)
      trackEvent('yt_stream_strategy_set', { strategy })
      toastManager.add({ title: 'Stream selection saved', type: 'success' })
      onSaved()
    } catch {
      toastManager.add({ title: 'Failed to save stream selection', type: 'error' })
    } finally {
      setSaving(false)
    }
  }

  const isNonDefault = strategy !== 'first_found'
  const locked = !isPremium && isNonDefault

  return (
    <Card className="mt-1 rounded-t-none border-t-0 bg-surface-2/50 p-4">
      <div className="space-y-3">
        <div>
          <label className="mb-1 block text-xs font-medium text-text-sub">
            Stream selection strategy
          </label>
          <p className="mb-2 text-xs text-text-sub/70">
            When this channel has multiple concurrent live streams, choose which one to monitor.
          </p>
          <select
            value={strategy}
            onChange={(e) => setStrategy(e.target.value as typeof strategy)}
            className="w-full rounded-lg border border-border bg-surface px-3 py-1.5 text-xs text-text focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
          >
            {STREAM_STRATEGIES.map((s) => (
              <option
                key={s.value}
                value={s.value}
                disabled={!isPremium && s.value !== 'first_found'}
              >
                {s.label}
                {!isPremium && s.value !== 'first_found' ? ' (Premium)' : ''}
              </option>
            ))}
          </select>
          {STREAM_STRATEGIES.find((s) => s.value === strategy) && (
            <p className="mt-1 text-xs text-text-sub/60">
              {STREAM_STRATEGIES.find((s) => s.value === strategy)?.description}
            </p>
          )}
        </div>

        {(strategy === 'title_match' || strategy === 'title_match_all') && (
          <div>
            <label className="mb-1 block text-xs font-medium text-text-sub">
              Title keyword
            </label>
            <Input
              value={matchTerm}
              onChange={(e) => setMatchTerm(e.target.value)}
              placeholder="e.g. synthwave, lofi, jazz"
              className="text-xs"
            />
          </div>
        )}

        {locked && (
          <p className="text-xs text-yellow-400/80">
            Non-default strategies require a premium subscription.
          </p>
        )}

        <Button
          size="sm"
          variant="outline"
          className="w-full"
          disabled={!hasChanges || saving || locked || ((strategy === 'title_match' || strategy === 'title_match_all') && !matchTerm.trim())}
          onClick={handleSave}
        >
          {saving ? 'Saving…' : 'Save'}
        </Button>
      </div>
    </Card>
  )
}

// ---- RelayPanel ------------------------------------------------------------

function RelayPanel({
  source,
  overlayId,
  onSaved,
}: {
  source: ChatSource
  overlayId: string
  onSaved: (updated: ChatSource) => void
}) {
  const discordConfig = source.config as DiscordSourceConfig
  const [relayEnabled, setRelayEnabled] = useState(discordConfig.relay_enabled ?? false)
  const [relayChannelId, setRelayChannelId] = useState<string>(
    discordConfig.relay_channel_id ?? ''
  )
  const [channels, setChannels] = useState<ChannelCategory[]>([])
  const [channelsLoading, setChannelsLoading] = useState(false)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    setChannelsLoading(true)
    getGuildChannels(discordConfig.guild_id)
      .then((res) => setChannels(res.categories))
      .catch(() => setChannels([]))
      .finally(() => setChannelsLoading(false))
  }, [discordConfig.guild_id])

  const handleSave = async () => {
    setSaving(true)
    const newConfig: DiscordSourceConfig = {
      ...discordConfig,
      relay_enabled: relayEnabled,
      relay_channel_id: relayEnabled ? relayChannelId : null,
    }
    // Optimistic update
    onSaved({ ...source, config: newConfig })
    try {
      await updateSourceConfig(overlayId, source.id, newConfig)
      toastManager.add({ title: 'Relay settings saved', type: 'success' })
    } catch {
      onSaved(source) // rollback
      toastManager.add({ title: 'Failed to save relay settings', type: 'error' })
    } finally {
      setSaving(false)
    }
  }

  const isSaveDisabled = saving || (relayEnabled && !relayChannelId)

  return (
    <div className="ml-4 mt-1 rounded-lg border border-border bg-surface-2 p-4 space-y-3">
      {/* Loop filter info — static */}
      <p className="text-xs text-text-sub">
        Loop filter: active — Discord messages are never relayed back to Discord.
      </p>

      {/* Relay toggle */}
      <label className="flex cursor-pointer items-center gap-2 text-sm text-text">
        <input
          type="checkbox"
          checked={relayEnabled}
          onChange={(e) => setRelayEnabled(e.target.checked)}
          className="accent-discord size-4"
        />
        Enable relay
      </label>

      {/* Outbound channel picker — visible only when relay enabled */}
      {relayEnabled && (
        <div>
          <label className="mb-1 block text-xs text-text-sub">Outbound channel</label>
          {channelsLoading ? (
            <Skeleton className="h-9 w-full rounded-lg" />
          ) : (
            <select
              value={relayChannelId}
              onChange={(e) => setRelayChannelId(e.target.value)}
              className="w-full rounded-lg border border-border bg-surface px-2 py-2 text-sm text-text focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
            >
              <option value="">Select a channel...</option>
              {channels.map((cat) => (
                <optgroup key={cat.id || 'uncategorized'} label={cat.name}>
                  {cat.channels.map((ch) => (
                    <option key={ch.id} value={ch.id}>
                      #{ch.name}
                    </option>
                  ))}
                </optgroup>
              ))}
            </select>
          )}
        </div>
      )}

      <Button
        size="sm"
        disabled={isSaveDisabled}
        onClick={() => void handleSave()}
        className="w-full"
      >
        {saving ? 'Saving...' : 'Save'}
      </Button>
    </div>
  )
}

function SourceListSkeleton() {
  return (
    <div className="space-y-3">
      {[0, 1].map((i) => (
        <Skeleton key={i} className="h-[60px] w-full rounded-xl" />
      ))}
    </div>
  )
}

// Platform source buttons — OAuth redirect for Twitch/YouTube/Kick,
// text input for TikTok (no OAuth required), dialog for Discord.
// Admins also get a manual channel ID form for any platform.
function AddSourceForm({
  overlayId,
  token,
  onAddTikTok,
  onAddManual,
  onSourceAdded,
  isAdmin = false,
}: {
  overlayId: string
  token: string
  onAddTikTok: (username: string) => void
  onAddManual?: (platform: string, channelId: string) => void
  onSourceAdded?: () => void
  isAdmin?: boolean
}) {
  const [tiktokUsername, setTiktokUsername] = useState('')
  const [isAdding, setIsAdding] = useState(false)
  const [adminPlatform, setAdminPlatform] = useState('twitch')

  // Discord dialog state
  const [guilds, setGuilds] = useState<DiscordGuild[]>([])
  const [guildsLoaded, setGuildsLoaded] = useState(false)
  const [discordDialogOpen, setDiscordDialogOpen] = useState(false)
  const [discordStep, setDiscordStep] = useState<1 | 2>(1)
  const [selectedGuild, setSelectedGuild] = useState<DiscordGuild | null>(null)
  const [selectedChannelId, setSelectedChannelId] = useState<string>('')
  const [selectedChannelName, setSelectedChannelName] = useState<string>('')
  const [guildChannels, setGuildChannels] = useState<ChannelCategory[]>([])
  const [channelsLoading, setChannelsLoading] = useState(false)
  const [isAddingDiscord, setIsAddingDiscord] = useState(false)

  useEffect(() => {
    getGuilds()
      .then((data) => setGuilds(data))
      .catch(() => setGuilds([]))
      .finally(() => setGuildsLoaded(true))
  }, [])

  const handleDiscordButtonClick = () => {
    if (guilds.length === 0) return
    setDiscordStep(1)
    setSelectedGuild(null)
    setSelectedChannelId('')
    setSelectedChannelName('')
    setGuildChannels([])
    setDiscordDialogOpen(true)
  }

  const fetchChannelsForGuild = async (guildId: string) => {
    setChannelsLoading(true)
    try {
      const res = await getGuildChannels(guildId)
      setGuildChannels(res.categories)
    } catch {
      setGuildChannels([])
    } finally {
      setChannelsLoading(false)
    }
  }

  const handleSelectGuild = (guild: DiscordGuild) => {
    setSelectedGuild(guild)
    setSelectedChannelId('')
    setSelectedChannelName('')
    setDiscordStep(2)
    void fetchChannelsForGuild(guild.guild_id)
  }

  const handleAddDiscordSource = async () => {
    if (!selectedGuild || !selectedChannelId) return
    setIsAddingDiscord(true)
    try {
      const config: DiscordSourceConfig & { guild_name: string } = {
        guild_id: selectedGuild.guild_id,
        guild_name: selectedGuild.guild_name,
        inbound_channel_id: selectedChannelId,
        relay_enabled: false,
        relay_channel_id: null,
      }
      await overlaysApi.addSource(overlayId, {
        platform: 'discord',
        channel_id: selectedChannelId,
        channel_name: selectedChannelName,
        config,
      })
      setDiscordDialogOpen(false)
      setDiscordStep(1)
      setSelectedGuild(null)
      setSelectedChannelId('')
      setSelectedChannelName('')
      onSourceAdded?.()
      toastManager.add({ title: 'Discord source added', type: 'success' })
    } catch {
      toastManager.add({ title: 'Failed to add Discord source', type: 'error' })
    } finally {
      setIsAddingDiscord(false)
    }
  }

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
  const [youtubeResolved, setYoutubeResolved] = useState<{
    channel_id: string
    title?: string
    custom_url?: string
    thumbnail?: string
  } | null>(null)
  const [youtubeResolveError, setYoutubeResolveError] = useState<string | null>(null)

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
      <p className="text-xs text-text-sub">
        Add a platform via OAuth or enter a TikTok username directly.
      </p>

      {/* OAuth buttons — fetch auth_url then redirect, same pattern as login */}
      <div className="grid grid-cols-1 gap-2">
        <button
          onClick={() => startOAuth(`/api/v1/auth/twitch/add-source/${overlayId}`)}
          className="flex items-center gap-2.5 rounded-lg bg-twitch px-4 py-2.5 text-sm font-semibold text-white transition-opacity hover:opacity-90 focus-visible:ring-2 focus-visible:ring-twitch focus-visible:ring-offset-2 focus-visible:ring-offset-bg focus-visible:outline-none"
        >
          <svg className="h-4 w-4 shrink-0" viewBox="0 0 24 24" aria-hidden="true">
            <path
              fill="#FFFFFF"
              d="M11.571 4.714h1.715v5.143H11.57zm4.715 0H18v5.143h-1.714zM6 0L1.714 4.286v15.428h5.143V24l4.286-4.286h3.428L22.286 12V0zm14.571 11.143l-3.428 3.428h-3.429l-3 3v-3H6.857V1.714h13.714z"
            />
          </svg>
          Connect Twitch
        </button>

        <button
          onClick={() => startOAuth(`/api/v1/auth/youtube/add-source/${overlayId}`)}
          className="flex items-center gap-2.5 rounded-lg px-4 py-2.5 text-sm font-semibold text-white transition-opacity hover:opacity-90 focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:ring-offset-bg focus-visible:outline-none"
          style={
            { backgroundColor: '#FF0000', '--tw-ring-color': '#FF0000' } as React.CSSProperties
          }
        >
          <svg className="h-4 w-4 shrink-0" viewBox="0 0 24 24" aria-hidden="true">
            <path
              fill="#FFFFFF"
              d="M23.498 6.186a3.016 3.016 0 0 0-2.122-2.136C19.505 3.545 12 3.545 12 3.545s-7.505 0-9.377.505A3.017 3.017 0 0 0 .502 6.186C0 8.07 0 12 0 12s0 3.93.502 5.814a3.016 3.016 0 0 0 2.122 2.136c1.871.505 9.376.505 9.376.505s7.505 0 9.377-.505a3.015 3.015 0 0 0 2.122-2.136C24 15.93 24 12 24 12s0-3.93-.502-5.814zM9.545 15.568V8.432L15.818 12l-6.273 3.568z"
            />
          </svg>
          Connect YouTube
        </button>

        <button
          onClick={() => startOAuth(`/api/v1/auth/kick/add-source/${overlayId}`)}
          className="flex items-center gap-2.5 rounded-lg px-4 py-2.5 text-sm font-semibold text-bg transition-opacity hover:opacity-90 focus-visible:ring-2 focus-visible:ring-kick focus-visible:ring-offset-2 focus-visible:ring-offset-bg focus-visible:outline-none"
          style={{ backgroundColor: 'var(--color-kick)' }}
        >
          <svg className="h-4 w-4 shrink-0" viewBox="0 0 512 512" aria-hidden="true">
            <path
              fill="currentColor"
              d="M37 .036h164.448v113.621h54.71v-56.82h54.731V.036h164.448v170.777h-54.73v56.82h-54.711v56.8h54.71v56.82h54.73V512.03H310.89v-56.82h-54.73v-56.8h-54.711v113.62H37V.036z"
            />
          </svg>
          Connect Kick
        </button>

        {/* Discord — guild dialog or settings prompt */}
        {guildsLoaded && guilds.length === 0 ? (
          <p className="rounded-lg border border-border bg-surface px-4 py-2.5 text-sm text-text-sub">
            Connect a Discord server in{' '}
            <Link href="/settings" className="text-discord underline hover:opacity-80">
              Settings
            </Link>{' '}
            first to add Discord sources.
          </p>
        ) : (
          <button
            onClick={handleDiscordButtonClick}
            className="flex items-center gap-2.5 rounded-lg px-4 py-2.5 text-sm font-semibold text-white transition-opacity hover:opacity-90 focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:ring-offset-bg focus-visible:outline-none"
            style={{ backgroundColor: '#5865F2', '--tw-ring-color': '#5865F2' } as React.CSSProperties}
          >
            {/* Discord logo mark */}
            <svg className="h-4 w-4 shrink-0" viewBox="0 0 24 24" aria-hidden="true">
              <path
                fill="#FFFFFF"
                d="M20.317 4.37a19.791 19.791 0 0 0-4.885-1.515.074.074 0 0 0-.079.037c-.21.375-.444.864-.608 1.25a18.27 18.27 0 0 0-5.487 0 12.64 12.64 0 0 0-.617-1.25.077.077 0 0 0-.079-.037A19.736 19.736 0 0 0 3.677 4.37a.07.07 0 0 0-.032.027C.533 9.046-.32 13.58.099 18.057c.001.022.015.043.03.056a19.9 19.9 0 0 0 5.993 3.03.078.078 0 0 0 .084-.028 14.09 14.09 0 0 0 1.226-1.994.076.076 0 0 0-.041-.106 13.107 13.107 0 0 1-1.872-.892.077.077 0 0 1-.008-.128 10.2 10.2 0 0 0 .372-.292.074.074 0 0 1 .077-.01c3.928 1.793 8.18 1.793 12.062 0a.074.074 0 0 1 .078.01c.12.098.246.198.373.292a.077.077 0 0 1-.006.127 12.299 12.299 0 0 1-1.873.892.077.077 0 0 0-.041.107c.36.698.772 1.362 1.225 1.993a.076.076 0 0 0 .084.028 19.839 19.839 0 0 0 6.002-3.03.077.077 0 0 0 .032-.054c.5-5.177-.838-9.674-3.549-13.66a.061.061 0 0 0-.031-.03zM8.02 15.33c-1.183 0-2.157-1.085-2.157-2.419 0-1.333.956-2.419 2.157-2.419 1.21 0 2.176 1.096 2.157 2.42 0 1.333-.956 2.418-2.157 2.418zm7.975 0c-1.183 0-2.157-1.085-2.157-2.419 0-1.333.955-2.419 2.157-2.419 1.21 0 2.176 1.096 2.157 2.42 0 1.333-.946 2.418-2.157 2.418z"
              />
            </svg>
            Connect Discord
          </button>
        )}
      </div>

      {/* Discord 2-step dialog */}
      <Dialog.Root
        open={discordDialogOpen}
        onOpenChange={(open) => {
          setDiscordDialogOpen(open)
          if (!open) {
            setDiscordStep(1)
            setSelectedGuild(null)
            setSelectedChannelId('')
            setSelectedChannelName('')
          }
        }}
      >
        <Dialog.Content>
          <Dialog.Title>
            {discordStep === 1 ? 'Select a Discord Server' : 'Select a Channel'}
          </Dialog.Title>
          <Dialog.Description>
            {discordStep === 1
              ? 'Choose which Discord server to add as a source.'
              : `Picking a channel from ${selectedGuild?.guild_name ?? ''}`}
          </Dialog.Description>

          {discordStep === 1 && (
            <div className="mt-3 space-y-2">
              {guilds.map((guild) => (
                <button
                  key={guild.guild_id}
                  onClick={() => handleSelectGuild(guild)}
                  className="flex w-full items-center gap-3 rounded-lg border border-border bg-surface px-3 py-2.5 text-sm text-text transition-colors hover:bg-surface-2 focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
                >
                  {guild.guild_icon ? (
                    // eslint-disable-next-line @next/next/no-img-element
                    <img
                      src={`https://cdn.discordapp.com/icons/${guild.guild_id}/${guild.guild_icon}.png?size=32`}
                      alt=""
                      className="size-8 rounded-full object-cover"
                    />
                  ) : (
                    <div className="flex size-8 shrink-0 items-center justify-center rounded-full bg-discord/20 text-xs font-bold text-discord">
                      {guild.guild_name.charAt(0).toUpperCase()}
                    </div>
                  )}
                  <span className="truncate font-medium">{guild.guild_name}</span>
                  <ChevronRight className="ml-auto size-4 shrink-0 text-text-sub" />
                </button>
              ))}
            </div>
          )}

          {discordStep === 2 && (
            <div className="mt-3 space-y-3">
              {channelsLoading ? (
                <Skeleton className="h-9 w-full rounded-lg" />
              ) : (
                <div>
                  <label className="mb-1 block text-xs text-text-sub">Channel</label>
                  <select
                    value={selectedChannelId}
                    onChange={(e) => {
                      setSelectedChannelId(e.target.value)
                      const ch = guildChannels
                        .flatMap((cat) => cat.channels)
                        .find((c) => c.id === e.target.value)
                      setSelectedChannelName(ch?.name ?? '')
                    }}
                    className="w-full rounded-lg border border-border bg-surface px-2 py-2 text-sm text-text focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
                  >
                    <option value="">Select a channel...</option>
                    {guildChannels.map((cat) => (
                      <optgroup key={cat.id || 'uncategorized'} label={cat.name}>
                        {cat.channels.map((ch) => (
                          <option key={ch.id} value={ch.id}>
                            #{ch.name}
                          </option>
                        ))}
                      </optgroup>
                    ))}
                  </select>
                </div>
              )}
            </div>
          )}

          <div className="mt-4 flex justify-end gap-2">
            {discordStep === 2 && (
              <Button
                variant="outline"
                onClick={() => {
                  setDiscordStep(1)
                  setSelectedChannelId('')
                  setSelectedChannelName('')
                }}
              >
                Back
              </Button>
            )}
            <Dialog.Close render={<Button variant="outline">Cancel</Button>} />
            {discordStep === 2 && (
              <Button
                disabled={!selectedChannelId || isAddingDiscord}
                onClick={() => void handleAddDiscordSource()}
              >
                {isAddingDiscord ? 'Adding...' : 'Add'}
              </Button>
            )}
          </div>
        </Dialog.Content>
      </Dialog.Root>

      {/* TikTok — username only, no OAuth */}
      <form onSubmit={handleTikTokSubmit} className="border-t border-border pt-1">
        <p className="mb-2 text-xs text-text-sub">TikTok (enter username)</p>
        <div className="flex gap-2">
          <Input
            value={tiktokUsername}
            onChange={(e) => setTiktokUsername(e.target.value)}
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
        <details className="border-t border-border pt-1">
          <summary className="cursor-pointer py-1 text-xs text-text-sub select-none hover:text-text">
            Admin: manual channel ID
          </summary>
          <form
            className="mt-2 space-y-2"
            onSubmit={async (e) => {
              e.preventDefault()
              if (!adminChannelId.trim()) return
              setIsAdminAdding(true)
              setYoutubeResolveError(null)
              try {
                if (adminPlatform === 'youtube') {
                  const resolved = await overlaysApi.resolveYouTubeChannel(adminChannelId.trim())
                  setYoutubeResolved(resolved)
                  await onAddManual(adminPlatform, resolved.channel_id)
                  setAdminChannelId('')
                  setYoutubeResolved(null)
                } else {
                  await onAddManual(adminPlatform, adminChannelId.trim())
                  setAdminChannelId('')
                }
              } catch (err) {
                if (adminPlatform === 'youtube') {
                  setYoutubeResolveError(err instanceof Error ? err.message : 'Failed to resolve YouTube channel')
                  setYoutubeResolved(null)
                }
              } finally {
                setIsAdminAdding(false)
              }
            }}
          >
            <div className="flex gap-2">
              <select
                value={adminPlatform}
                onChange={(e) => {
                  setAdminPlatform(e.target.value)
                  setYoutubeResolved(null)
                  setYoutubeResolveError(null)
                }}
                className="rounded-lg border border-border bg-surface px-2 py-1.5 text-xs text-text focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
              >
                <option value="twitch">Twitch</option>
                <option value="youtube">YouTube</option>
                <option value="kick">Kick</option>
                <option value="tiktok">TikTok</option>
              </select>
              <Input
                value={adminChannelId}
                onChange={(e) => {
                  setAdminChannelId(e.target.value)
                  setYoutubeResolved(null)
                  setYoutubeResolveError(null)
                }}
                placeholder={adminPlatform === 'youtube' ? '@handle, channel URL, or UC…' : 'Channel ID or username'}
                className="flex-1 text-xs"
              />
            </div>
            {adminPlatform === 'youtube' && youtubeResolved && (
              <div className="flex items-center gap-2 rounded-lg bg-surface px-2 py-1.5 text-xs text-text-sub">
                {youtubeResolved.thumbnail && (
                  // eslint-disable-next-line @next/next/no-img-element
                  <img src={youtubeResolved.thumbnail} alt="" className="h-6 w-6 rounded-full object-cover" />
                )}
                <span className="font-medium text-text">{youtubeResolved.title ?? youtubeResolved.channel_id}</span>
                {youtubeResolved.custom_url && (
                  <span className="text-text-sub">{youtubeResolved.custom_url}</span>
                )}
                <span className="ml-auto font-mono text-[10px] text-text-sub">{youtubeResolved.channel_id}</span>
              </div>
            )}
            {adminPlatform === 'youtube' && youtubeResolveError && (
              <p className="text-xs text-red-500">{youtubeResolveError}</p>
            )}
            <Button
              type="submit"
              disabled={isAdminAdding || !adminChannelId.trim()}
              size="sm"
              variant="outline"
              className="w-full"
            >
              {isAdminAdding
                ? adminPlatform === 'youtube'
                  ? 'Resolving…'
                  : 'Adding…'
                : 'Add manually'}
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
  const [acceptedShares, setAcceptedShares] = useState<AcceptedShare[]>([])
  const [revokeTarget, setRevokeTarget] = useState<ChatSource | null>(null)

  // --- Customization state ---
  const [maxMessages, setMaxMessages] = useState(50)
  const [messageDuration, setMessageDuration] = useState(15)
  const [disableMessageFade, setDisableMessageFade] = useState(false)
  const [invertMessageOrder, setInvertMessageOrder] = useState(false)
  const [enable7tv, setEnable7tv] = useState(true)
  const [enableBttv, setEnableBttv] = useState(true)
  const [enableFfz, setEnableFfz] = useState(true)
  // Per-overlay 7TV override input. The user can paste an emote-set ID
  // (24-char hex or 26-char ULID), a 7tv.app emote-set URL, or a 7tv.app user
  // profile URL; the backend resolves and stores the canonical set ID.
  const [seventvOverrideInput, setSeventvOverrideInput] = useState('')
  const [seventvOverrideSavedID, setSeventvOverrideSavedID] = useState('')
  // Descriptor for the currently-saved set, populated by an auto-resolve on
  // config load so the editor shows the human name + emote count instead of
  // an opaque ULID. Cleared whenever the saved ID changes to "".
  const [seventvSavedDescriptor, setSeventvSavedDescriptor] = useState<
    | { name?: string; emoteCount?: number }
    | null
  >(null)
  const [seventvResolveState, setSeventvResolveState] = useState<
    | { status: 'idle' }
    | { status: 'resolving' }
    | { status: 'resolved'; setID: string; name?: string; emoteCount?: number }
    | { status: 'error'; message: string }
  >({ status: 'idle' })
  const [seventvRemoving, setSeventvRemoving] = useState(false)
  const [isPublicForViewers, setIsPublicForViewers] = useState(false)
  const [configLoaded, setConfigLoaded] = useState(false)
  const [isSavingConfig, setIsSavingConfig] = useState(false)
  const [configAlert, setConfigAlert] = useState<{
    type: 'success' | 'error'
    message: string
  } | null>(null)

  // --- Mock messages state ---
  const [mockForm, setMockForm] = useState<MockMessageFormState>(DEFAULT_MOCK_FORM)

  // --- Custom CSS state ---
  const [customCss, setCustomCss] = useState('')
  const [useCustomCss, setUseCustomCss] = useState(false)

  // --- Visual appearance settings state ---
  const [visualSettings, setVisualSettings] = useState<Partial<VisualSettings>>({})
  const [iframeVisibilityDefaults, setIframeVisibilityDefaults] = useState<Partial<VisualSettings>>({})
  const [parsedThemeSettings, setParsedThemeSettings] = useState<Partial<VisualSettings>>({})
  const [showThemeConfirm, setShowThemeConfirm] = useState(false)
  const [pendingTheme, setPendingTheme] = useState<{ css: string; parsed: Partial<VisualSettings> } | null>(null)

  // --- Iframe ref for live preview communication ---
  const iframeRef = useRef<HTMLIFrameElement | null>(null)

  // --- Latest-value ref for visualSettings (avoids stale closure in handleIframeReady) ---
  const visualSettingsRef = useRef<Partial<VisualSettings>>({})
  visualSettingsRef.current = visualSettings

  // --- Filter settings state (Phase 11) ---
  const [filterSettings, setFilterSettings] = useState<FilterSettings>({})
  const filterSettingsRef = useRef<FilterSettings>({})
  filterSettingsRef.current = filterSettings

  // --- Sound settings state (Phase 12) ---
  const [soundSettings, setSoundSettings] = useState<Partial<DisplaySettings>>({})
  const soundSettingsRef = useRef<Partial<DisplaySettings>>({})
  soundSettingsRef.current = soundSettings

  // --- TTS settings state (Phase 13 — Plans 01 & 03) ---
  // `ttsSettings` holds the tts_* fields that live in display_settings (persisted
  // via the existing updateConfig flow). `hasElevenLabsConfig` + `obsUrl` come
  // from the separate GET /tts-config endpoint (Plan 02) — the ElevenLabs key
  // and voice_id are NEVER in display_settings.
  const [ttsSettings, setTtsSettings] = useState<Partial<DisplaySettings>>({})
  const ttsSettingsRef = useRef<Partial<DisplaySettings>>({})
  ttsSettingsRef.current = ttsSettings
  const [hasElevenLabsConfig, setHasElevenLabsConfig] = useState(false)
  const [obsUrl, setObsUrl] = useState<string | undefined>(undefined)
  // Persisted ElevenLabs voice_id (Issue #276). Lives outside display_settings
  // because the voice_id column is on overlay_tts_configs, not the overlay
  // configs blob. Drives TTSGroup's picker initial value and the visibility of
  // the "Save voice" button.
  const [elevenLabsVoiceId, setElevenLabsVoiceId] = useState<string | undefined>(undefined)

  // --- OBS URL copy state ---
  const [copiedObs, setCopiedObs] = useState(false)

  // --- Discord relay state ---
  const [relayExpandedSourceId, setRelayExpandedSourceId] = useState<string | null>(null)
  const [streamSelectExpandedSourceId, setStreamSelectExpandedSourceId] = useState<string | null>(null)

  // --- Share overlay state ---
  const [showShareModal, setShowShareModal] = useState(false)
  const [showPremiumRequired, setShowPremiumRequired] = useState(false)
  const [shareRecipient, setShareRecipient] = useState('')
  const [shareLoading, setShareLoading] = useState(false)
  const shareInputRef = useRef<HTMLInputElement>(null)

  // --- Clone / Reset Overlay ID state ---
  const [isCloning, setIsCloning] = useState(false)
  const [showResetConfirm, setShowResetConfirm] = useState(false)
  const [isResetting, setIsResetting] = useState(false)

  // --- sendFilterSettingsToIframe: post filter settings to the embed iframe (Phase 11) ---
  const sendFilterSettingsToIframe = useCallback((settings: FilterSettings) => {
    iframeRef.current?.contentWindow?.postMessage(
      { type: 'FILTER_SETTINGS_UPDATE', filterSettings: settings },
      '*'
    )
  }, [])

  // --- sendSoundSettingsToIframe: post sound settings to the embed iframe (Phase 12) ---
  const sendSoundSettingsToIframe = useCallback((settings: Partial<DisplaySettings>) => {
    iframeRef.current?.contentWindow?.postMessage(
      { type: 'SOUND_SETTINGS_UPDATE', soundSettings: settings },
      '*'
    )
  }, [])

  // --- sendTtsSettingsToIframe: post TTS settings to the embed iframe (Phase 13 D-22) ---
  // No debounce — fires on every change so the editor preview reflects the tuning
  // immediately. Only the 20 tts_* display_settings fields travel here; the
  // ElevenLabs runtime (endpoint/token/voiceId) is loaded by the iframe itself
  // via GET /tts-config and is preserved across these postMessages.
  const sendTtsSettingsToIframe = useCallback((settings: Partial<DisplaySettings>) => {
    iframeRef.current?.contentWindow?.postMessage(
      {
        type: 'TTS_SETTINGS_UPDATE',
        ttsSettings: {
          tts_enabled: settings.tts_enabled,
          tts_provider: settings.tts_provider,
          tts_volume: settings.tts_volume,
          tts_voice_uri: settings.tts_voice_uri,
          tts_rate: settings.tts_rate,
          tts_pitch: settings.tts_pitch,
          tts_filter_mode: settings.tts_filter_mode,
          tts_sample_rate: settings.tts_sample_rate,
          tts_max_queue: settings.tts_max_queue,
          tts_messages_per_minute: settings.tts_messages_per_minute,
          tts_user_cooldown_seconds: settings.tts_user_cooldown_seconds,
          tts_staleness_seconds: settings.tts_staleness_seconds,
          tts_priority_events: settings.tts_priority_events,
          tts_priority_bits_min: settings.tts_priority_bits_min,
          tts_read_username: settings.tts_read_username,
          tts_read_platform: settings.tts_read_platform,
          tts_max_message_chars: settings.tts_max_message_chars,
          tts_skip_emote_only: settings.tts_skip_emote_only,
          tts_skip_links: settings.tts_skip_links,
          tts_enabled_platforms: settings.tts_enabled_platforms,
        },
      },
      '*'
    )
  }, [])

  // --- sendCssToIframe: post CSS generated from visualSettings to the iframe ---
  const sendCssToIframe = useCallback((settings: Partial<VisualSettings>) => {
    const css = visualSettingsToCss(settings)
    iframeRef.current?.contentWindow?.postMessage(
      { type: 'VISUAL_CSS_UPDATE', css },
      '*'
    )
    // Send non-CSS visual settings (platform badge position/style, indicators toggle)
    iframeRef.current?.contentWindow?.postMessage(
      {
        type: 'VISUAL_SETTINGS_UPDATE',
        settings: {
          platformBadgePosition: settings.platformBadgePosition,
          platformBadgeStyle: settings.platformBadgeStyle,
          showPlatformBadge: settings.showPlatformBadge,
          showPlatformIndicators: settings.showPlatformIndicators,
        },
      },
      '*'
    )
  }, [])

  // --- handleVisualSettingsChange: merge patch, update state, send CSS ---
  const handleVisualSettingsChange = useCallback((patch: Partial<VisualSettings>) => {
    setVisualSettings((prev) => {
      const next = { ...prev, ...patch }
      sendCssToIframe(next)
      return next
    })
  }, [sendCssToIframe])

  // --- handleFilterSettingsChange: merge patch, update state, send to iframe immediately (D-07 WYSIWYG) ---
  const handleFilterSettingsChange = useCallback((patch: Partial<FilterSettings>) => {
    setFilterSettings((prev) => {
      const next = { ...prev, ...patch }
      sendFilterSettingsToIframe(next)
      return next
    })
  }, [sendFilterSettingsToIframe])

  // --- handleSoundSettingsChange: merge patch, update state, send to iframe (Phase 12) ---
  const handleSoundSettingsChange = useCallback((patch: Partial<DisplaySettings>) => {
    setSoundSettings((prev) => {
      const next = { ...prev, ...patch }
      sendSoundSettingsToIframe(next)
      return next
    })
  }, [sendSoundSettingsToIframe])

  // --- handleTTSSettingsChange: merge patch, update state, send to embed iframe (Phase 13 D-22) ---
  const handleTTSSettingsChange = useCallback((patch: Partial<DisplaySettings>) => {
    setTtsSettings((prev) => {
      const next = { ...prev, ...patch }
      sendTtsSettingsToIframe(next)
      return next
    })
  }, [sendTtsSettingsToIframe])

  // Phase 13 Plan 03 — ElevenLabs flow handlers (wired via AppearancePanel -> TTSGroup props)
  const handleSaveTTSKey = useCallback(async (apiKey: string, voiceId: string): Promise<void> => {
    await overlaysApi.saveTTSKey(id, apiKey, voiceId)
    // Refresh metadata so the OBS URL + Test button render.
    const meta = await overlaysApi.getTTSConfig(id)
    setHasElevenLabsConfig(meta.has_elevenlabs_config)
    setObsUrl(meta.obs_url)
    setElevenLabsVoiceId(meta.voice_id)
  }, [id])

  // Issue #276 — voice-only update path. Persists to PATCH /tts-config/voice
  // and refreshes local state so the picker no longer shows the "Save voice"
  // button (pickedVoiceId === savedVoiceId).
  const handleSaveTTSVoice = useCallback(async (voiceId: string): Promise<void> => {
    await overlaysApi.saveTTSVoice(id, voiceId)
    setElevenLabsVoiceId(voiceId)
  }, [id])

  const handleRemoveTTSKey = useCallback(async (): Promise<void> => {
    await overlaysApi.removeTTSKey(id)
    setHasElevenLabsConfig(false)
    setObsUrl(undefined)
    setElevenLabsVoiceId(undefined)
  }, [id])

  const handleRotateTTSToken = useCallback(async (): Promise<{ obsUrl: string }> => {
    const result = await overlaysApi.rotateTTSToken(id)
    setObsUrl(result.obsUrl)
    return result
  }, [id])

  const handleTestTTSKey = useCallback(async (): Promise<TestKeyResult> => {
    const result = await overlaysApi.testTTSKey(id)
    if (result.ok && result.audioBlob) {
      // Autoplay is user-initiated (Test key click), so the browser grants playback.
      try {
        const audio = new Audio(URL.createObjectURL(result.audioBlob))
        await audio.play()
        audio.onended = (): void => {
          URL.revokeObjectURL(audio.src)
        }
      } catch {
        // Swallow — the quota number still renders, and the caller shows
        // verbose errors for non-ok cases.
      }
    }
    return result
  }, [id])

  const handleFetchTTSVoices = useCallback(async (): Promise<ElevenLabsVoice[]> => {
    return overlaysApi.getTTSVoices(id)
  }, [id])

  const handlePreviewTTSVoices = useCallback(
    async (apiKey: string): Promise<ElevenLabsVoice[]> => {
      return overlaysApi.previewTTSVoices(id, apiKey)
    },
    [id],
  )

  // --- sendCustomCssToIframe: post the full theme CSS to the embed preview ---
  const sendCustomCssToIframe = useCallback((css: string) => {
    iframeRef.current?.contentWindow?.postMessage(
      { type: 'CUSTOM_CSS_UPDATE', css },
      '*'
    )
  }, [])

  // --- applyThemeImmediately: atomically apply CSS + parsed settings ---
  const applyThemeImmediately = useCallback(
    (css: string, parsed: Partial<VisualSettings>) => {
      setCustomCss(css)
      setUseCustomCss(true)
      setVisualSettings(parsed)
      setParsedThemeSettings(parsed)
      sendCssToIframe(parsed)
      sendCustomCssToIframe(css)
    },
    [sendCssToIframe, sendCustomCssToIframe]
  )

  // --- handleResetToTheme: restore visualSettings to parsedThemeSettings (or {}) ---
  const handleResetToTheme = useCallback(() => {
    setVisualSettings(parsedThemeSettings)
    sendCssToIframe(parsedThemeSettings)
  }, [parsedThemeSettings, sendCssToIframe])

  // --- handleThemeApply: apply a theme CSS string from ThemeContent ---
  // Prompts for confirmation if visual settings are already customized.
  function handleThemeApply(css: string): void {
    const parsed = parseCssToVisualSettings(css)
    const hasExisting = Object.keys(visualSettings).length > 0
    if (hasExisting) {
      setPendingTheme({ css, parsed })
      setShowThemeConfirm(true)
    } else {
      applyThemeImmediately(css, parsed)
    }
  }

  // --- EMBED_READY: re-send CSS, filter settings, sound settings, and TTS settings when embed page signals its listener is registered ---
  useEffect(() => {
    const handleEmbedReady = (event: MessageEvent) => {
      if (event.data?.type !== 'EMBED_READY') return
      sendCssToIframe(visualSettingsRef.current)
      // Also send current filter settings to the embed on ready
      sendFilterSettingsToIframe(filterSettingsRef.current)
      // Also send current sound settings to the embed on ready
      sendSoundSettingsToIframe(soundSettingsRef.current)
      // Phase 13: send current TTS settings to the embed on ready (D-22)
      sendTtsSettingsToIframe(ttsSettingsRef.current)
    }
    window.addEventListener('message', handleEmbedReady)
    return () => window.removeEventListener('message', handleEmbedReady)
  }, [sendCssToIframe, sendFilterSettingsToIframe, sendSoundSettingsToIframe, sendTtsSettingsToIframe])

  // --- handleIframeReady: store iframe ref, send initial CSS, and query visibility defaults ---
  const handleIframeReady = useCallback((iframe: HTMLIFrameElement) => {
    iframeRef.current = iframe
    sendCssToIframe(visualSettingsRef.current)
    const doc = iframe.contentDocument ?? iframe.contentWindow?.document
    if (doc) {
      const style = getComputedStyle(doc.documentElement)
      const visFields: Array<[keyof VisualSettings, string]> = [
        ['showAvatars', '--chat-show-avatars'],
        ['showBadges', '--chat-show-badges'],
        ['showTimestamps', '--chat-show-timestamps'],
        ['showPlatformBadge', '--chat-show-platform-badge'],
        ['showEmotes', '--chat-show-emotes'],
        ['showUsername', '--chat-show-username'],
      ]
      const defaults: Partial<VisualSettings> = {}
      for (const [field, cssVar] of visFields) {
        const v = style.getPropertyValue(cssVar).trim()
        if (v) {
          ;(defaults as Record<string, string>)[field] = v
        }
      }
      setIframeVisibilityDefaults(defaults)
    }
  }, [sendCssToIframe])

  // Load overlay, sources, accepted shares and config
  // Note: `router` is intentionally excluded from deps — it is only used for the
  // `!token` redirect guard which ProtectedRoute already handles. Including router
  // causes loadData to re-run whenever the Next.js router object reference changes
  // (e.g. during API proxy processing), which would overwrite user-set extension
  // overlay state with stale DB data.
  useEffect(() => {
    if (!token) {
      router.push('/')
      return
    }

    let cancelled = false

    const loadData = async () => {
      try {
        const overlayData = await overlaysApi.get(id)
        if (cancelled) return
        setOverlay(overlayData)
        setIsPublicForViewers(overlayData.is_public_for_viewers)

        try {
          const sourcesData = await overlaysApi.getSources(id)
          if (cancelled) return
          setSources(sourcesData)
        } catch {
          if (!cancelled) setSources([])
        }

        // Load accepted shares — non-critical
        try {
          const sharesData = await sharesApi.getAcceptedShares()
          if (!cancelled) setAcceptedShares(sharesData)
        } catch {
          // Non-critical
        }

        // Load overlay config for customization defaults
        try {
          const config = await overlaysApi.getConfig(id)
          if (cancelled) return
          const display = config.display_settings || {}

          if (typeof display.max_messages === 'number') setMaxMessages(display.max_messages)
          if (typeof display.message_duration === 'number')
            setMessageDuration(display.message_duration)
          if (typeof display.disable_message_fade === 'boolean')
            setDisableMessageFade(display.disable_message_fade)
          setInvertMessageOrder(display.invert_message_order === true)

          if (typeof config.enable_7tv === 'boolean') setEnable7tv(config.enable_7tv)
          if (typeof config.enable_bttv === 'boolean') setEnableBttv(config.enable_bttv)
          if (typeof config.enable_ffz === 'boolean') setEnableFfz(config.enable_ffz)
          if (typeof config.seventv_emote_set_id === 'string') {
            const saved = config.seventv_emote_set_id
            setSeventvOverrideInput(saved)
            setSeventvOverrideSavedID(saved)
            // Auto-resolve the saved set so the editor can show "Currently
            // active: <name> (N emotes)" instead of a raw ULID — otherwise
            // the user has no way to tell what's attached.
            if (saved.trim() !== '') {
              overlaysApi
                .resolveSevenTV(id, saved)
                .then((result) => {
                  if (cancelled) return
                  setSeventvSavedDescriptor({
                    name: result.name,
                    emoteCount: result.emote_count,
                  })
                })
                .catch(() => {
                  // Best-effort: if 7TV is unreachable or the saved ID is
                  // stale, fall back silently to showing just the ID.
                  if (cancelled) return
                  setSeventvSavedDescriptor(null)
                })
            } else {
              setSeventvSavedDescriptor(null)
            }
          }

          const css = config.custom_css || ''
          setCustomCss(css)
          setUseCustomCss(Boolean(css.trim().length))

          // Parse theme CSS — always set parsedThemeSettings so "Reset to theme defaults" works
          // after save+reload (when visual_settings is non-empty, theme defaults are still needed)
          const savedVisual = config.visual_settings as Partial<VisualSettings> | null
          const hasNoSavedVisual = !savedVisual || Object.keys(savedVisual).length === 0
          const parsedFromCss = css.trim() ? parseCssToVisualSettings(css) : {}
          if (css.trim()) {
            setParsedThemeSettings(parsedFromCss)
          }

          // Migrate platform badge settings from display_settings to visual_settings
          const platformBadgeDefaults: Partial<VisualSettings> = {}
          if (typeof display.show_platform_badge === 'boolean') {
            platformBadgeDefaults.showPlatformBadge = display.show_platform_badge ? 'inline' : 'none'
          }
          if (display.platform_badge_position === 'before' || display.platform_badge_position === 'after') {
            platformBadgeDefaults.platformBadgePosition = display.platform_badge_position
          }
          if (display.platform_badge_style === 'text' || display.platform_badge_style === 'icon') {
            platformBadgeDefaults.platformBadgeStyle = display.platform_badge_style
          }

          const merged: Partial<VisualSettings> = {
            ...platformBadgeDefaults,
            ...parsedFromCss,
            ...(savedVisual ?? {}),
            // migrate legacy font_size to visualSettings.fontSize if not already present
            fontSize:
              savedVisual?.fontSize ??
              parsedFromCss.fontSize ??
              (typeof display.font_size === 'number' ? `${display.font_size}px` : undefined),
          }
          setVisualSettings(merged)
          sendCssToIframe(merged)

          // Phase 11: Load filter settings
          if (config.filter_settings) {
            setFilterSettings(config.filter_settings)
          }

          // Phase 12: Load sound settings from display_settings
          if (config.display_settings) {
            const d = config.display_settings
            const loaded: Partial<DisplaySettings> = {}
            if (typeof d.notification_sound_enabled === 'boolean') loaded.notification_sound_enabled = d.notification_sound_enabled
            if (typeof d.notification_sound_preset === 'string') loaded.notification_sound_preset = d.notification_sound_preset
            if (typeof d.notification_sound_volume === 'number') loaded.notification_sound_volume = d.notification_sound_volume
            if (typeof d.notification_sound_cooldown === 'number') loaded.notification_sound_cooldown = d.notification_sound_cooldown
            if (typeof d.notification_sound_url === 'string') loaded.notification_sound_url = d.notification_sound_url
            setSoundSettings(loaded)
          }

          // Phase 13: Load TTS display_settings (Plan 01 fields — 20 total)
          if (config.display_settings) {
            const d = config.display_settings
            const tts: Partial<DisplaySettings> = {}
            if (typeof d.tts_enabled === 'boolean') tts.tts_enabled = d.tts_enabled
            if (d.tts_provider === 'browser' || d.tts_provider === 'elevenlabs') tts.tts_provider = d.tts_provider
            if (typeof d.tts_volume === 'number') tts.tts_volume = d.tts_volume
            if (typeof d.tts_voice_uri === 'string') tts.tts_voice_uri = d.tts_voice_uri
            if (typeof d.tts_rate === 'number') tts.tts_rate = d.tts_rate
            if (typeof d.tts_pitch === 'number') tts.tts_pitch = d.tts_pitch
            if (d.tts_filter_mode === 'all' || d.tts_filter_mode === 'sample' || d.tts_filter_mode === 'priority_only') {
              tts.tts_filter_mode = d.tts_filter_mode
            }
            if (typeof d.tts_sample_rate === 'number') tts.tts_sample_rate = d.tts_sample_rate
            if (typeof d.tts_max_queue === 'number') tts.tts_max_queue = d.tts_max_queue
            if (typeof d.tts_messages_per_minute === 'number') tts.tts_messages_per_minute = d.tts_messages_per_minute
            if (typeof d.tts_user_cooldown_seconds === 'number') tts.tts_user_cooldown_seconds = d.tts_user_cooldown_seconds
            if (typeof d.tts_staleness_seconds === 'number') tts.tts_staleness_seconds = d.tts_staleness_seconds
            if (typeof d.tts_priority_events === 'boolean') tts.tts_priority_events = d.tts_priority_events
            if (typeof d.tts_priority_bits_min === 'number') tts.tts_priority_bits_min = d.tts_priority_bits_min
            if (typeof d.tts_read_username === 'boolean') tts.tts_read_username = d.tts_read_username
            if (typeof d.tts_read_platform === 'boolean') tts.tts_read_platform = d.tts_read_platform
            if (typeof d.tts_max_message_chars === 'number') tts.tts_max_message_chars = d.tts_max_message_chars
            if (typeof d.tts_skip_emote_only === 'boolean') tts.tts_skip_emote_only = d.tts_skip_emote_only
            if (typeof d.tts_skip_links === 'boolean') tts.tts_skip_links = d.tts_skip_links
            if (Array.isArray(d.tts_enabled_platforms)) {
              tts.tts_enabled_platforms = d.tts_enabled_platforms.filter(
                (p: unknown): p is string => typeof p === 'string',
              )
            }
            setTtsSettings(tts)
          }

          // Phase 13 Plan 03: Load ElevenLabs config metadata (has-key + OBS URL).
          // Non-fatal — a 404 on first edit or a non-premium user simply leaves
          // the Advanced block in its unsaved-empty state.
          try {
            const meta: TTSConfigMetadata = await overlaysApi.getTTSConfig(id)
            if (!cancelled) {
              setHasElevenLabsConfig(meta.has_elevenlabs_config)
              setObsUrl(meta.obs_url)
              setElevenLabsVoiceId(meta.voice_id)
            }
          } catch (e) {
            console.warn('[OverlayEditor] getTTSConfig failed:', e)
          }
        } catch (err) {
          console.warn('[OverlayEditor] Failed to load config', err)
        } finally {
          if (!cancelled) setConfigLoaded(true)
        }
      } catch {
        if (!cancelled) setOverlay(null)
      } finally {
        if (!cancelled) setIsLoading(false)
      }
    }

    loadData()

    return () => {
      cancelled = true
    }
  }, [id, token]) // eslint-disable-line react-hooks/exhaustive-deps

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
      setSources((prev) => prev.filter((s) => s.id !== sourceId))
      toastManager.add({ title: 'Source removed', type: 'success' })
    } catch {
      toastManager.add({
        title: 'Failed to remove source',
        description: 'Please try again.',
        type: 'error',
      })
    }
  }

  // Starts the Twitch add-source OAuth reflow so the streamer can (re)grant the EventSub
  // chat scopes for their own channel. The add-source endpoint now requests
  // user:read:chat + user:bot + channel:bot; once granted, the channel moves to the
  // EventSub listener on the next sync.
  async function handleReconnectTwitchChat() {
    try {
      const res = await fetch(`/api/v1/auth/twitch/add-source/${id}`, {
        headers: { Authorization: `Bearer ${token}` },
      })
      const data = await res.json()
      if (data.auth_url) {
        window.location.href = data.auth_url
      } else {
        console.error('No auth_url returned for Twitch reconnect', data)
      }
    } catch (err) {
      console.error('Failed to start Twitch chat reconnect', err)
    }
  }

  async function handleAddTikTokSource(username: string) {
    try {
      const source = await overlaysApi.addSource(id, {
        platform: 'tiktok',
        channel_id: username,
      })
      setSources((prev) => [...prev, source])
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
      setSources((prev) => [...prev, source])
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

  // --- Discord relay ---

  function handleRelayConfigSaved(updated: ChatSource) {
    setSources((prev) => prev.map((s) => (s.id === updated.id ? updated : s)))
  }

  // --- OBS URL ---

  function handleCopyObsUrl() {
    const url = `${window.location.origin}/overlay/${id}`
    navigator.clipboard.writeText(url).then(() => {
      trackEvent('obs_url_copied', { surface: 'editor' })
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

  async function handleCloneOverlay() {
    setIsCloning(true)
    try {
      const cloned = await overlaysApi.clone(id)
      toastManager.add({ title: 'Overlay cloned', type: 'success' })
      router.push(`/overlays/${cloned.id}`)
    } catch {
      toastManager.add({ title: 'Failed to clone overlay', type: 'error' })
    } finally {
      setIsCloning(false)
    }
  }

  async function handleConfirmResetOverlayId() {
    setIsResetting(true)
    try {
      const cloned = await overlaysApi.clone(id)
      if (overlay) {
        await overlaysApi.update(cloned.id, { name: overlay.name })
      }
      await overlaysApi.delete(id)
      toastManager.add({ title: 'Overlay ID reset — redirecting…', type: 'success' })
      router.push(`/overlays/${cloned.id}`)
    } catch {
      toastManager.add({ title: 'Failed to reset overlay ID', type: 'error' })
      setIsResetting(false)
    }
    setShowResetConfirm(false)
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
    value: MockMessageFormState[K]
  ) {
    setMockForm((prev) => ({ ...prev, [field]: value }))
  }

  const resolveMockTarget = (requestedPlatform?: ChatMessage['platform']) => {
    const preferred = sources.find((source) =>
      requestedPlatform ? source.platform === requestedPlatform : true
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
          mockForm.username || mockForm.displayName.toLowerCase().replace(/\s+/g, '') || 'mockuser',
        display_name: mockForm.displayName || mockForm.username || 'Mock Viewer',
        avatar_url: mockForm.avatarUrl || undefined,
        color: mockForm.color || undefined,
        metadata: { mock: true, source: 'editor-form' },
      })
      setMockForm((prev) => ({ ...prev, message: '' }))
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
        await new Promise((resolve) => setTimeout(resolve, 800))
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
          font_size: parseInt(visualSettings.fontSize ?? '16') || 16,
          message_duration: messageDuration,
          max_messages: maxMessages,
          disable_message_fade: disableMessageFade,
          invert_message_order: invertMessageOrder,
          platform_badge_position: visualSettings.platformBadgePosition ?? 'before',
          platform_badge_style: visualSettings.platformBadgeStyle ?? 'text',
          show_platform_badge: visualSettings.showPlatformBadge !== 'none',
          notification_sound_enabled: soundSettings.notification_sound_enabled ?? false,
          notification_sound_preset: soundSettings.notification_sound_preset ?? 'chime',
          notification_sound_volume: soundSettings.notification_sound_volume ?? 0.5,
          notification_sound_cooldown: soundSettings.notification_sound_cooldown ?? 500,
          ...(soundSettings.notification_sound_url ? { notification_sound_url: soundSettings.notification_sound_url } : {}),
          // Phase 13: persist all 20 tts_* fields (ElevenLabs key/voice live in overlay_tts_configs, NOT here)
          ...ttsSettings,
        },
        enable_7tv: enable7tv,
        enable_bttv: enableBttv,
        enable_ffz: enableFfz,
        custom_css: useCustomCss ? customCss : '',
        visual_settings: visualSettings,
        filter_settings: filterSettings,
        // Send the (already-resolved) override only when the user touched it.
        // Sending an empty string clears the override server-side.
        ...(seventvOverrideInput !== seventvOverrideSavedID || seventvOverrideInput === ''
          ? { seventv_emote_set_id: seventvOverrideInput.trim() }
          : {}),
      })
      const newSavedID = seventvOverrideInput.trim()
      setSeventvOverrideSavedID(newSavedID)
      // Mirror the descriptor to whatever the user just verified, so the
      // "Currently active" line refreshes without re-resolving.
      if (newSavedID === '') {
        setSeventvSavedDescriptor(null)
      } else if (
        seventvResolveState.status === 'resolved' &&
        seventvResolveState.setID === newSavedID
      ) {
        setSeventvSavedDescriptor({
          name: seventvResolveState.name,
          emoteCount: seventvResolveState.emoteCount,
        })
      }
      setConfigAlert({ type: 'success', message: 'Configuration saved!' })
    } catch (error) {
      console.error('[Editor] Failed to save config', error)
      setConfigAlert({ type: 'error', message: 'Failed to save configuration' })
    } finally {
      setIsSavingConfig(false)
      setTimeout(() => setConfigAlert(null), 5000)
    }
  }

  async function handleSetAsExtensionOverlay() {
    // Optimistic update — show Active state immediately
    setIsPublicForViewers(true)
    try {
      const updated = await overlaysApi.update(id, { is_public_for_viewers: true })
      setOverlay(updated)
      toastManager.add({
        title: 'Extension overlay set',
        description: 'This overlay will be shown in the browser extension.',
        type: 'success',
      })
    } catch {
      // Roll back optimistic update on failure
      setIsPublicForViewers(false)
      toastManager.add({ title: 'Failed to update overlay', type: 'error' })
    }
  }

  async function handleUnsetExtensionOverlay() {
    // Optimistic update — show inactive state immediately
    setIsPublicForViewers(false)
    try {
      const updated = await overlaysApi.update(id, { is_public_for_viewers: false })
      setOverlay(updated)
      toastManager.add({ title: 'Extension overlay deactivated', type: 'success' })
    } catch {
      // Roll back optimistic update on failure
      setIsPublicForViewers(true)
      toastManager.add({ title: 'Failed to update overlay', type: 'error' })
    }
  }

  // --- Loading / error states ---

  if (isLoading) {
    return (
      <div className="min-h-screen bg-bg">
        <AppNav />
        <div className="flex h-[calc(100vh-60px)] items-center justify-center">
          <div className="w-64 space-y-3">
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
            <p className="text-destructive mb-4 text-lg">Overlay not found</p>
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
      <SplitView overlayId={id} onIframeReady={handleIframeReady}>
        {/* Config panel content */}
        <div className="max-w-none space-y-6 p-6">
          {/* 1. Header */}
          <div className="flex items-start justify-between">
            <div>
              <button
                onClick={() => router.push('/dashboard')}
                className="mb-1 flex items-center gap-1 rounded text-sm text-text-sub hover:text-text focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
              >
                <ChevronLeft className="size-4" />
                Back
              </button>
              <h1 className="text-xl font-bold text-text">{overlay.name}</h1>
              {overlay.description && (
                <p className="mt-0.5 text-sm text-text-sub">{overlay.description}</p>
              )}
            </div>
            <div className="ml-4 flex shrink-0 flex-wrap gap-2">
              <Button
                variant="outline"
                size="sm"
                onClick={() => window.open(`/overlay/${id}/view`, '_blank', 'noopener,noreferrer')}
                title="Open the readable chat & activity monitor in a new tab"
              >
                Monitor View
              </Button>
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
              <Button
                variant="outline"
                size="sm"
                disabled={isCloning}
                onClick={() => void handleCloneOverlay()}
              >
                {isCloning ? 'Cloning…' : 'Clone'}
              </Button>
            </div>
          </div>

          {/* 2. Copy OBS URL */}
          <Button
            variant="outline"
            className="flex w-full items-center justify-center gap-2"
            onClick={handleCopyObsUrl}
          >
            <Clipboard className="size-4" />
            {copiedObs ? 'Copied!' : 'Copy OBS URL'}
          </Button>

          {/* 2b. Share overlay */}
          <Button
            variant="outline"
            className="flex w-full items-center justify-center gap-2"
            onClick={handleShareClick}
          >
            <Share2 className="size-4" />
            Share Overlay
          </Button>

          {/* 2c. Extension overlay */}
          <Card className="p-4">
            <div className="flex items-start gap-3">
              <Puzzle className="mt-0.5 size-4 shrink-0 text-twitch" />
              <div className="min-w-0 flex-1">
                <div className="mb-0.5 flex items-center gap-2">
                  <p className="text-sm font-semibold text-text">Browser Extension Overlay</p>
                  {isPublicForViewers && (
                    <span className="inline-flex items-center rounded border border-twitch/30 bg-twitch/15 px-1.5 py-0.5 text-[10px] font-semibold text-twitch">
                      Active
                    </span>
                  )}
                </div>
                <p className="text-xs text-text-sub">
                  {isPublicForViewers
                    ? 'This overlay is shown to viewers via the browser extension at allch.at/c/caesarlp.'
                    : 'Set this as the overlay shown to viewers via the browser extension.'}
                </p>
              </div>
              {isPublicForViewers ? (
                <Button
                  variant="ghost"
                  size="sm"
                  className="hover:text-destructive shrink-0 text-xs text-text-sub"
                  onClick={handleUnsetExtensionOverlay}
                >
                  Deactivate
                </Button>
              ) : (
                <Button
                  variant="outline"
                  size="sm"
                  className="shrink-0 text-xs"
                  onClick={handleSetAsExtensionOverlay}
                >
                  Set Active
                </Button>
              )}
            </div>
          </Card>

          {/* Premium required dialog */}
          {showPremiumRequired && (
            <div
              className="fixed inset-0 z-50 flex items-center justify-center bg-black/50"
              onClick={() => setShowPremiumRequired(false)}
            >
              <div
                className="mx-4 w-full max-w-sm rounded-xl border border-border bg-surface p-6 shadow-xl"
                onClick={(e) => e.stopPropagation()}
              >
                <div className="mb-3 flex items-start justify-between">
                  <h2 className="text-lg font-semibold text-text">Premium Feature</h2>
                  <button
                    onClick={() => setShowPremiumRequired(false)}
                    className="text-text-sub hover:text-text"
                  >
                    <X className="size-4" />
                  </button>
                </div>
                <p className="mb-4 text-sm text-text-sub">
                  Sharing your overlay is a premium feature. Upgrade your account to share your chat
                  with other streamers.
                </p>
                <p className="mb-5 text-sm text-text-sub">
                  For more information and to get access, join our Discord community.
                </p>
                <div className="flex gap-2">
                  <Button
                    variant="outline"
                    className="flex-1"
                    onClick={() => setShowPremiumRequired(false)}
                  >
                    Close
                  </Button>
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
            <div
              className="fixed inset-0 z-50 flex items-center justify-center bg-black/50"
              onClick={() => setShowShareModal(false)}
            >
              <div
                className="mx-4 w-full max-w-sm rounded-xl border border-border bg-surface p-6 shadow-xl"
                onClick={(e) => e.stopPropagation()}
              >
                <div className="mb-3 flex items-start justify-between">
                  <h2 className="text-lg font-semibold text-text">Share Overlay</h2>
                  <button
                    onClick={() => setShowShareModal(false)}
                    className="text-text-sub hover:text-text"
                  >
                    <X className="size-4" />
                  </button>
                </div>
                <p className="mb-4 text-sm text-text-sub">
                  Enter the Twitch username of the person you want to share{' '}
                  <strong>{overlay?.name}</strong> with. They&apos;ll receive a request they can
                  accept or decline.
                </p>
                <div className="mb-4">
                  <label className="mb-1 block text-xs text-text-sub">Twitch username</label>
                  <input
                    ref={shareInputRef}
                    type="text"
                    value={shareRecipient}
                    onChange={(e) => setShareRecipient(e.target.value)}
                    onKeyDown={(e) => e.key === 'Enter' && handleSendShareRequest()}
                    placeholder="e.g. somestreamer"
                    className="w-full rounded-lg border border-border bg-surface-2 px-3 py-2 text-sm text-text placeholder:text-text-sub focus-visible:ring-2 focus-visible:ring-twitch/50 focus-visible:outline-none"
                    disabled={shareLoading}
                  />
                </div>
                <div className="flex gap-2">
                  <Button
                    variant="outline"
                    className="flex-1"
                    onClick={() => setShowShareModal(false)}
                    disabled={shareLoading}
                  >
                    Cancel
                  </Button>
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

          {/* 5-section collapsible editor panel */}
          {/* sticky footer uses position:sticky bottom-0 — works because split-view-config has overflow-y-auto */}
          <div className="relative">
            <div className="divide-y divide-border">
              {/* Theme section — first and open by default */}
              <CollapsibleSection
                id="theme"
                title="Theme"
                storageKey="editor-panel-sections-v1"
                defaultOpen={true}
              >
                <ThemeContent onApply={handleThemeApply} isAdmin={user?.is_admin === true} />
                <button
                  type="button"
                  className="mt-3 text-xs text-text-sub underline-offset-2 hover:text-text hover:underline"
                  onClick={handleResetToTheme}
                >
                  Reset to theme defaults
                </button>
              </CollapsibleSection>

              {/* Appearance section */}
              <CollapsibleSection
                id="appearance"
                title="Appearance"
                storageKey="editor-panel-sections-v1"
                defaultOpen={false}
              >
                <AppearancePanel
                  visualSettings={visualSettings}
                  onChange={handleVisualSettingsChange}
                  visibilityDefaults={iframeVisibilityDefaults}
                  filterSettings={filterSettings}
                  onFilterChange={handleFilterSettingsChange}
                  displaySettings={{ ...soundSettings, ...ttsSettings }}
                  onSoundChange={handleSoundSettingsChange}
                  isPremium={user?.is_premium ?? false}
                  onTTSChange={handleTTSSettingsChange}
                  overlayId={id}
                  hasElevenLabsConfig={hasElevenLabsConfig}
                  obsUrl={obsUrl}
                  onTTSPreview={() => {
                    // Browser Web Speech API preview — fires the fixed sample phrase through the
                    // current rate/pitch/volume/voice_uri. Click again mid-speech cancels.
                    if (typeof window === 'undefined' || typeof window.speechSynthesis === 'undefined') return
                    const synth = window.speechSynthesis
                    if (synth.speaking) {
                      synth.cancel()
                      return
                    }
                    const u = new SpeechSynthesisUtterance(
                      'Hello, this is how your chat will sound.',
                    )
                    u.volume = ttsSettings.tts_volume ?? 0.8
                    u.rate = ttsSettings.tts_rate ?? 1.0
                    u.pitch = ttsSettings.tts_pitch ?? 1.0
                    const savedUri = ttsSettings.tts_voice_uri
                    if (savedUri) {
                      const match = synth.getVoices().find((v) => v.voiceURI === savedUri)
                      if (match) u.voice = match
                    }
                    synth.speak(u)
                  }}
                  onTTSPreviewStop={() => {
                    if (typeof window !== 'undefined' && window.speechSynthesis) {
                      window.speechSynthesis.cancel()
                    }
                  }}
                  onSaveTTSKey={handleSaveTTSKey}
                  onSaveTTSVoice={handleSaveTTSVoice}
                  savedTTSVoiceId={elevenLabsVoiceId}
                  onTestTTSKey={handleTestTTSKey}
                  onRotateTTSToken={handleRotateTTSToken}
                  onRemoveTTSKey={handleRemoveTTSKey}
                  onFetchTTSVoices={handleFetchTTSVoices}
                  onPreviewTTSVoices={handlePreviewTTSVoices}
                />
              </CollapsibleSection>

              {/* Sources section — open by default */}
              <CollapsibleSection
                id="sources"
                title="Sources"
                storageKey="editor-panel-sections-v1"
                defaultOpen={true}
              >
                {isLoading ? (
                  <SourceListSkeleton />
                ) : (
                  <div className="mb-4 space-y-3">
                    {sources.map((source) => (
                      <div key={source.id}>
                        <SourceCard
                          source={source}
                          onRemove={handleRemoveSource}
                          onRevoke={setRevokeTarget}
                          onConfigureRelay={(s) =>
                            setRelayExpandedSourceId((prev) => (prev === s.id ? null : s.id))
                          }
                          onConfigureStreamSelect={(s) =>
                            setStreamSelectExpandedSourceId((prev) => (prev === s.id ? null : s.id))
                          }
                          isOwnChannel={
                            user?.auth_provider === 'twitch' &&
                            !!user?.username &&
                            user.username.toLowerCase() === source.channel_id.toLowerCase()
                          }
                          onReconnectChat={handleReconnectTwitchChat}
                        />
                        {source.id === relayExpandedSourceId && source.platform === 'discord' && (
                          <RelayPanel
                            source={source}
                            overlayId={id}
                            onSaved={handleRelayConfigSaved}
                          />
                        )}
                        {source.id === streamSelectExpandedSourceId && source.platform === 'youtube' && (
                          <StreamSelectionPanel
                            source={source}
                            overlayId={id}
                            isPremium={user?.is_premium ?? false}
                            onSaved={async () => {
                              const updated = await overlaysApi.getSources(id)
                              setSources(updated)
                            }}
                          />
                        )}
                      </div>
                    ))}
                    {sources.length === 0 && (
                      <p className="py-2 text-sm text-text-sub">
                        No sources added yet. Add a platform below.
                      </p>
                    )}
                  </div>
                )}

                {/* Accepted shared overlays — add as source */}
                {acceptedShares.length > 0 && (
                  <div className="mb-4 border-t border-border pt-4">
                    <h3 className="mb-3 text-sm font-medium text-text">Shared Overlays</h3>
                    <div className="space-y-2">
                      {acceptedShares.map((share) => (
                        <button
                          key={share.share_id}
                          onClick={() => handleAddSharedOverlay(share)}
                          className="flex w-full items-center justify-between rounded-lg border border-border bg-surface px-3 py-2 text-sm text-text transition-colors hover:bg-surface-2"
                        >
                          <span>{share.sender_display_name}&apos;s overlay</span>
                          <span className="text-xs text-twitch">+ Add</span>
                        </button>
                      ))}
                    </div>
                  </div>
                )}

                <AddSourceForm
                  overlayId={id}
                  token={token ?? ''}
                  onAddTikTok={handleAddTikTokSource}
                  onAddManual={handleAddManual}
                  onSourceAdded={() => overlaysApi.getSources(id).then(setSources).catch(console.error)}
                  isAdmin={user?.is_admin === true}
                />
              </CollapsibleSection>

              {/* Behavior section */}
              <CollapsibleSection
                id="behavior"
                title="Behavior"
                storageKey="editor-panel-sections-v1"
                defaultOpen={false}
              >
                <div className="space-y-5">
                  {/* Max Messages */}
                  <div>
                    <label className="mb-1 block text-xs text-text-sub">
                      Max Messages: <span className="text-twitch">{maxMessages}</span>
                    </label>
                    <input
                      type="range"
                      min="10"
                      max="100"
                      value={maxMessages}
                      onChange={(e) => setMaxMessages(parseInt(e.target.value))}
                      className="w-full accent-twitch"
                    />
                  </div>

                  {/* Message Duration */}
                  <div>
                    <label className="mb-1 block text-xs text-text-sub">
                      Message Duration: <span className="text-twitch">{messageDuration}s</span>
                    </label>
                    <input
                      type="range"
                      min="5"
                      max="60"
                      value={messageDuration}
                      onChange={(e) => setMessageDuration(parseInt(e.target.value))}
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
                        onChange={(e) => setDisableMessageFade(e.target.checked)}
                        className="accent-twitch"
                      />
                      Disable Message Fade Out
                    </label>
                    <p className="mt-1 ml-5 text-xs text-text-sub">
                      Messages stay visible until max is reached
                    </p>
                  </div>

                  {/* Invert message order */}
                  <div>
                    <label className="flex items-center gap-2 text-xs text-text-sub">
                      <input
                        type="checkbox"
                        checked={invertMessageOrder}
                        onChange={(e) => setInvertMessageOrder(e.target.checked)}
                        className="accent-twitch"
                      />
                      Invert Message Order
                    </label>
                    <p className="mt-1 ml-5 text-xs text-text-sub">
                      Show newest messages at the top instead of the bottom
                    </p>
                  </div>

                  {/* Emote Providers */}
                  <div>
                    <p className="mb-2 text-xs text-text-sub">Emote Providers</p>
                    <div className="space-y-1.5">
                      <label className="flex items-center gap-2 text-xs text-text-sub">
                        <input
                          type="checkbox"
                          checked={enable7tv}
                          onChange={(e) => setEnable7tv(e.target.checked)}
                          className="accent-twitch"
                        />
                        7TV
                      </label>
                      <label className="flex items-center gap-2 text-xs text-text-sub">
                        <input
                          type="checkbox"
                          checked={enableBttv}
                          onChange={(e) => setEnableBttv(e.target.checked)}
                          className="accent-twitch"
                        />
                        BetterTTV
                      </label>
                      <label className="flex items-center gap-2 text-xs text-text-sub">
                        <input
                          type="checkbox"
                          checked={enableFfz}
                          onChange={(e) => setEnableFfz(e.target.checked)}
                          className="accent-twitch"
                        />
                        FrankerFaceZ
                      </label>
                    </div>
                  </div>

                  {/* 7TV emote-set override — useful when no Twitch source exists */}
                  <div>
                    <p className="mb-1 text-xs text-text-sub">7TV Emote Set</p>
                    <p className="mb-2 text-[11px] text-text-sub/70">
                      Optional. Paste a 7TV emote-set ID, an emote-set URL, or your
                      7TV profile URL to attach those emotes to this overlay
                      regardless of which platforms you stream on.
                    </p>
                    {/* Saved-state pill: shows what's actually attached right now,
                        with a one-click Remove. Hidden while nothing is saved. */}
                    {seventvOverrideSavedID !== '' && (
                      <div className="mb-2 flex items-center justify-between gap-2 rounded-md border border-border bg-surface-2 px-2 py-1.5 text-[11px]">
                        <span className="truncate text-text">
                          <span className="text-text-sub">Currently active: </span>
                          {seventvSavedDescriptor?.name ? (
                            <>
                              <span className="font-medium">&quot;{seventvSavedDescriptor.name}&quot;</span>
                              {typeof seventvSavedDescriptor.emoteCount === 'number'
                                ? ` (${seventvSavedDescriptor.emoteCount} emotes)`
                                : ''}
                            </>
                          ) : (
                            <span className="font-mono">{seventvOverrideSavedID}</span>
                          )}
                        </span>
                        <Button
                          type="button"
                          variant="ghost"
                          size="sm"
                          className="shrink-0 text-[11px] text-red-500 hover:text-red-400"
                          disabled={seventvRemoving || isSavingConfig}
                          onClick={async () => {
                            setSeventvRemoving(true)
                            try {
                              await overlaysApi.updateConfig(id, {
                                seventv_emote_set_id: '',
                              })
                              setSeventvOverrideInput('')
                              setSeventvOverrideSavedID('')
                              setSeventvSavedDescriptor(null)
                              setSeventvResolveState({ status: 'idle' })
                              setConfigAlert({
                                type: 'success',
                                message: '7TV emote set removed',
                              })
                              setTimeout(() => setConfigAlert(null), 5000)
                            } catch (err) {
                              const message =
                                err instanceof Error
                                  ? err.message
                                  : 'Failed to remove 7TV emote set'
                              setConfigAlert({ type: 'error', message })
                              setTimeout(() => setConfigAlert(null), 5000)
                            } finally {
                              setSeventvRemoving(false)
                            }
                          }}
                        >
                          {seventvRemoving ? 'Removing…' : 'Remove'}
                        </Button>
                      </div>
                    )}
                    <div className="flex gap-2">
                      <input
                        type="text"
                        value={seventvOverrideInput}
                        onChange={(e) => {
                          setSeventvOverrideInput(e.target.value)
                          setSeventvResolveState({ status: 'idle' })
                        }}
                        placeholder={
                          seventvOverrideSavedID !== ''
                            ? 'Paste a new ID/URL to replace…'
                            : 'https://7tv.app/users/...'
                        }
                        className="flex-1 rounded-md border border-border bg-bg px-2 py-1 text-xs text-text"
                      />
                      <Button
                        type="button"
                        variant="outline"
                        size="sm"
                        className="text-xs"
                        disabled={
                          seventvResolveState.status === 'resolving' ||
                          seventvOverrideInput.trim() === '' ||
                          seventvOverrideInput.trim() === seventvOverrideSavedID
                        }
                        onClick={async () => {
                          setSeventvResolveState({ status: 'resolving' })
                          try {
                            const result = await overlaysApi.resolveSevenTV(
                              id,
                              seventvOverrideInput.trim(),
                            )
                            setSeventvResolveState({
                              status: 'resolved',
                              setID: result.emote_set_id,
                              name: result.name,
                              emoteCount: result.emote_count,
                            })
                          } catch (err) {
                            const message =
                              err instanceof Error ? err.message : 'Could not resolve 7TV reference'
                            setSeventvResolveState({ status: 'error', message })
                          }
                        }}
                      >
                        {seventvResolveState.status === 'resolving' ? 'Checking…' : 'Verify'}
                      </Button>
                    </div>
                    {seventvResolveState.status === 'resolved' &&
                      seventvResolveState.setID !== seventvOverrideSavedID && (
                        <p className="mt-1 text-[11px] text-green-500">
                          Resolved
                          {seventvResolveState.name ? ` to "${seventvResolveState.name}"` : ''}
                          {typeof seventvResolveState.emoteCount === 'number'
                            ? ` (${seventvResolveState.emoteCount} emotes)`
                            : ''}
                          {' — click Save Configuration to apply.'}
                        </p>
                    )}
                    {seventvResolveState.status === 'error' && (
                      <p className="mt-1 text-[11px] text-red-500">
                        {seventvResolveState.message}
                      </p>
                    )}
                  </div>
                </div>
              </CollapsibleSection>

              {/* Expert section — collapsed by default */}
              <CollapsibleSection
                id="expert"
                title="Expert"
                storageKey="editor-panel-sections-v1"
                defaultOpen={false}
              >
                {/* Custom CSS */}
                <div className="space-y-3">
                  <div className="flex items-center gap-3">
                    <span className="text-xs font-medium text-text">Custom CSS</span>
                    <label className="flex cursor-pointer items-center gap-2 text-xs text-text-sub">
                      <input
                        type="checkbox"
                        checked={useCustomCss}
                        onChange={(e) => setUseCustomCss(e.target.checked)}
                        className="accent-twitch"
                      />
                      Enable
                    </label>
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      className="ml-auto text-xs"
                      onClick={() => {
                        setCustomCss('')
                        setUseCustomCss(false)
                      }}
                    >
                      Reset
                    </Button>
                  </div>

                  <MonacoCSSEditor
                    value={customCss}
                    onChange={setCustomCss}
                    height="300px"
                    placeholder="/* Enter your custom CSS here */"
                  />

                  <p className="text-xs text-text-sub">
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
                </div>

                {/* Mock Messages */}
                <div className="mt-6 space-y-3">
                  <p className="text-xs font-medium text-text">Mock Messages</p>
                  <div>
                    <label className="mb-1 block text-xs text-text-sub">Platform</label>
                    <select
                      value={mockForm.platform}
                      onChange={(e) =>
                        handleMockInputChange(
                          'platform',
                          e.target.value as MockMessageFormState['platform']
                        )
                      }
                      className="w-full rounded-lg border border-border bg-surface px-2 py-2 text-sm text-text focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
                    >
                      <option value="twitch">Twitch</option>
                      <option value="youtube">YouTube</option>
                      <option value="kick">Kick</option>
                      <option value="tiktok">TikTok</option>
                    </select>
                  </div>
                  <div className="grid grid-cols-2 gap-2">
                    <div>
                      <label className="mb-1 block text-xs text-text-sub">Display Name</label>
                      <input
                        type="text"
                        value={mockForm.displayName}
                        onChange={(e) => handleMockInputChange('displayName', e.target.value)}
                        className="w-full rounded-lg border border-border bg-surface px-2 py-2 text-sm text-text focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
                      />
                    </div>
                    <div>
                      <label className="mb-1 block text-xs text-text-sub">Username</label>
                      <input
                        type="text"
                        value={mockForm.username}
                        onChange={(e) => handleMockInputChange('username', e.target.value)}
                        className="w-full rounded-lg border border-border bg-surface px-2 py-2 text-sm text-text focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
                      />
                    </div>
                  </div>
                  <div className="grid grid-cols-2 gap-2">
                    <div>
                      <label className="mb-1 block text-xs text-text-sub">Avatar URL</label>
                      <input
                        type="text"
                        value={mockForm.avatarUrl}
                        onChange={(e) => handleMockInputChange('avatarUrl', e.target.value)}
                        placeholder="https://..."
                        className="w-full rounded-lg border border-border bg-surface px-2 py-2 text-sm text-text focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
                      />
                    </div>
                    <div>
                      <label className="mb-1 block text-xs text-text-sub">Name Color</label>
                      <input
                        type="color"
                        value={mockForm.color}
                        onChange={(e) => handleMockInputChange('color', e.target.value)}
                        className="h-9 w-full rounded-lg border border-border bg-surface px-2 py-1.5"
                      />
                    </div>
                  </div>
                  <div>
                    <label className="mb-1 block text-xs text-text-sub">Message</label>
                    <textarea
                      value={mockForm.message}
                      onChange={(e) => handleMockInputChange('message', e.target.value)}
                      className="h-16 w-full resize-none rounded-lg border border-border bg-surface px-2 py-2 text-sm text-text focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
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
                      className="flex-1 border-yellow-600/40 text-xs text-yellow-400 hover:bg-yellow-900/20"
                      onClick={() => void handleAddSampleEvents()}
                    >
                      ⭐ Sample Events
                    </Button>
                  </div>
                </div>
              </CollapsibleSection>

              {/* Danger Zone section */}
              <CollapsibleSection
                id="danger-zone"
                title="Danger Zone"
                storageKey="editor-panel-sections-v1"
                defaultOpen={false}
              >
                <div className="space-y-3">
                  <p className="text-xs text-text-sub">
                    Reset your overlay ID to revoke any leaked OBS URLs. A new overlay with the same
                    configuration will be created and you will be redirected to it. The old overlay and
                    its URL will be permanently deleted.
                  </p>
                  <Button
                    type="button"
                    variant="outline"
                    className="w-full border-destructive/50 text-destructive hover:bg-destructive/10"
                    onClick={() => setShowResetConfirm(true)}
                    disabled={isResetting}
                  >
                    {isResetting ? 'Resetting…' : 'Reset Overlay ID'}
                  </Button>
                </div>
              </CollapsibleSection>
            </div>

            {/* Sticky Save footer — position:sticky works inside overflow-y-auto split-view-config container */}
            <div className="sticky bottom-0 z-10 -mx-6 border-t border-border bg-bg/95 p-4 backdrop-blur-sm">
              <Button
                onClick={() => void handleSaveConfiguration()}
                disabled={!configLoaded || isSavingConfig}
                className="w-full"
              >
                {isSavingConfig ? 'Saving...' : 'Save Configuration'}
              </Button>
              {configAlert && (
                <p
                  className={cn(
                    'mt-2 text-center text-sm',
                    configAlert.type === 'success' ? 'text-green-400' : 'text-destructive'
                  )}
                >
                  {configAlert.message}
                </p>
              )}
            </div>
          </div>
        </div>
      </SplitView>

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

      {/* Theme apply confirm dialog */}
      <Dialog.Root open={showThemeConfirm} onOpenChange={setShowThemeConfirm}>
        <Dialog.Content size="sm" showCloseButton={false}>
          <Dialog.Title>Apply theme?</Dialog.Title>
          <Dialog.Description>
            Loading this theme will reset your visual customizations. Continue?
          </Dialog.Description>
          <div className="mt-4 flex justify-end gap-2">
            <Dialog.Close
              render={
                <Button type="button" variant="outline" size="sm">
                  Cancel
                </Button>
              }
            />
            <Button
              type="button"
              size="sm"
              onClick={() => {
                if (pendingTheme) {
                  applyThemeImmediately(pendingTheme.css, pendingTheme.parsed)
                  setPendingTheme(null)
                }
                setShowThemeConfirm(false)
              }}
            >
              Continue
            </Button>
          </div>
        </Dialog.Content>
      </Dialog.Root>

      {/* Reset Overlay ID confirm dialog */}
      <Dialog.Root open={showResetConfirm} onOpenChange={setShowResetConfirm}>
        <Dialog.Content size="sm" showCloseButton={false}>
          <Dialog.Title>Reset Overlay ID?</Dialog.Title>
          <Dialog.Description>
            This will create a new overlay with a fresh ID and permanently delete this one.
            Any existing OBS URLs will stop working — update your browser source after the reset.
          </Dialog.Description>
          <div className="mt-4 flex justify-end gap-2">
            <Dialog.Close
              render={
                <Button type="button" variant="outline" size="sm">
                  Cancel
                </Button>
              }
            />
            <Button
              type="button"
              size="sm"
              className="border-destructive bg-destructive text-destructive-foreground hover:bg-destructive/90"
              disabled={isResetting}
              onClick={() => void handleConfirmResetOverlayId()}
            >
              {isResetting ? 'Resetting…' : 'Reset ID'}
            </Button>
          </div>
        </Dialog.Content>
      </Dialog.Root>
    </div>
  )
}
