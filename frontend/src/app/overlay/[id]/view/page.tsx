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
 * Overlay Monitor View (/overlay/[id]/view) — authenticated dashboard.
 *
 * A Twitch-dashboard-inspired monitor for streamers: a resizable live Chat
 * panel + Activity feed, platform connection indicators, an overlay-config
 * summary, its own light/dark mode, view-local display toggles, and per-message
 * / per-user moderation controls for the overlay's owner. It reuses the exact
 * realtime pipeline the OBS overlay speaks (useOverlayStream) but renders a
 * readable, animation-free dashboard that ignores the overlay's CSS themes.
 *
 * Auth is enforced by the route's layout (ProtectedRoute via OverlayViewGuard);
 * moderation is further gated on overlay ownership + per-source capabilities.
 */

'use client'

import clsx from 'clsx'
import { ExternalLink, Info, SlidersHorizontal } from 'lucide-react'
import Link from 'next/link'
import { use, useCallback, useEffect, useMemo, useRef, useState } from 'react'
import toast from 'react-hot-toast'

import { MaintenanceInfoButton } from '@/components/MaintenanceInfoButton'
import PlatformStatusIndicators from '@/components/PlatformStatusIndicators'
import { ActivityPanel } from '@/components/overlay/ActivityPanel'
import { ChatPanel, type ChatPanelModeration } from '@/components/overlay/ChatPanel'
import { ChatSendBar } from '@/components/overlay/ChatSendBar'
import { ConnectionBadge } from '@/components/overlay/ConnectionBadge'
import { LayoutPicker } from '@/components/overlay/LayoutPicker'
import { ObservabilitySummary } from '@/components/overlay/ObservabilitySummary'
import { OverlayViewThemeToggle } from '@/components/overlay/OverlayViewThemeToggle'
import { ViewSettingsBar } from '@/components/overlay/ViewSettingsBar'
import { ResizableSplit } from '@/components/ResizableSplit'
import { useOverlayStream } from '@/hooks/useOverlayStream'
import { authApi } from '@/lib/api/auth'
import { startDiscordModerationReinvite } from '@/lib/api/discord'
import {
  buildBanRequest,
  buildDeleteRequest,
  buildTimeoutRequest,
  buildUnbanRequest,
  moderationApi,
} from '@/lib/api/moderation'
import type { ChatMessage, DeletionMetadata } from '@/lib/types/message'
import type { ModerationCapabilities, SourceCapability } from '@/lib/types/moderation'
import type { EventSettings } from '@/lib/types/overlay'
import {
  applyModerationMark,
  deletionSignature,
  mergeByAgg,
  partitionItems,
  toModEntry,
  type ModEntry,
  type ViewItem,
} from '@/lib/utils/overlayViewModel'

import {
  DEFAULT_VIEW_LAYOUT,
  LAYOUT_CONFIG,
  loadViewLayout,
  saveViewLayout,
  type ViewLayout,
} from './viewLayout'
import {
  DEFAULT_VIEW_PREFS,
  loadViewPrefs,
  saveViewPrefs,
  type MonitorViewPrefs,
} from './viewPrefs'

const MAX_ITEMS = 500
const MAX_MOD_LOG = 200
const THEME_KEY = 'overlay-view-theme'

export default function OverlayMonitorView({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params)

  const [items, setItems] = useState<ViewItem[]>([])
  const [moderationLog, setModerationLog] = useState<ModEntry[]>([])
  const [observedEventTypes, setObservedEventTypes] = useState<Set<string>>(new Set())
  const [eventSettings, setEventSettings] = useState<EventSettings | null>(null)
  const [light, setLight] = useState(false)
  const [showDetails, setShowDetails] = useState(false)
  const [prefs, setPrefs] = useState<MonitorViewPrefs>(DEFAULT_VIEW_PREFS)
  const [layout, setLayout] = useState<ViewLayout>(DEFAULT_VIEW_LAYOUT)
  const [capabilities, setCapabilities] = useState<ModerationCapabilities | null>(null)
  const modSeqRef = useRef(0)
  // Signatures of deletions we applied optimistically, awaiting their WS echo —
  // so the server-pushed confirmation doesn't double-log the action.
  const pendingDeletionsRef = useRef<Set<string>>(new Set())
  // Live mirror of `items` so optimistic moderation can compute exactly which
  // rows it struck through (for rollback) without an impure state updater.
  const itemsRef = useRef<ViewItem[]>([])
  useEffect(() => {
    itemsRef.current = items
  }, [items])

  // --- Stream callbacks ----------------------------------------------------

  const onChat = useCallback((message: ChatMessage) => {
    setItems((prev) => [...prev, message].slice(-MAX_ITEMS))
    const type = message.event?.type
    if (type) {
      setObservedEventTypes((prev) => (prev.has(type) ? prev : new Set(prev).add(type)))
    }
  }, [])

  const onMessageUpdate = useCallback((message: ChatMessage) => {
    setItems((prev) => mergeByAgg(prev, message, MAX_ITEMS))
  }, [])

  const onDeletion = useCallback((deletion: DeletionMetadata, source: 'replay' | 'live') => {
    // Observability: keep the message visible (struck-through) and log the action.
    setItems((prev) => applyModerationMark(prev, deletion))
    // Dedup: if this confirms an optimistic action we already logged, swallow it.
    const sig = deletionSignature(deletion)
    if (pendingDeletionsRef.current.delete(sig)) return
    setModerationLog((prev) =>
      [
        ...prev,
        { id: (modSeqRef.current += 1), ...toModEntry(deletion, source, Date.now()) },
      ].slice(-MAX_MOD_LOG)
    )
  }, [])

  const { config, sources, activeChannels, channelStatuses, connectionStatus, reconnectAttempts } =
    useOverlayStream(id, {
      onChat,
      onMessageUpdate,
      onDeletion,
    })

  // Fetch per-overlay event toggles (degrades gracefully if disabled — setState
  // happens in a promise callback, so this is not a synchronous set-state-in-effect).
  useEffect(() => {
    let cancelled = false
    fetch(`/api/v1/overlays/public/${id}/event-settings`)
      .then((r) => (r.ok ? r.json() : null))
      .then((data) => {
        if (!cancelled && data) setEventSettings(data as EventSettings)
      })
      .catch(() => {
        /* event-settings panel falls back to observed event types */
      })
    return () => {
      cancelled = true
    }
  }, [id])

  // Fetch moderation capabilities once the user is known. A non-owner gets
  // { is_owner:false, sources:[] }; any failure leaves moderation disabled.
  useEffect(() => {
    let cancelled = false
    moderationApi
      .getCapabilities(id)
      .then((data) => {
        if (!cancelled) setCapabilities(data)
      })
      .catch(() => {
        if (!cancelled) setCapabilities({ is_owner: false, enabled: false, sources: [] })
      })
    return () => {
      cancelled = true
    }
  }, [id])

  // Restore saved view prefs once on mount (localStorage; client only).
  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- one-time restore from localStorage
    setPrefs(loadViewPrefs())
  }, [])

  const updatePrefs = useCallback((next: MonitorViewPrefs) => {
    setPrefs(next)
    saveViewPrefs(next)
  }, [])

  // Restore the saved panel layout after mount (per-overlay; localStorage,
  // client only — guards against SSR hydration mismatch like the prefs/theme).
  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- one-time restore from localStorage
    setLayout(loadViewLayout(id))
  }, [id])

  const updateLayout = useCallback(
    (next: ViewLayout) => {
      setLayout(next)
      saveViewLayout(id, next)
    },
    [id]
  )

  // Start the opt-in moderation setup (ADR-0017). For the OAuth platforms this fetches a
  // consent URL (auth-service requests only the minimal moderation scopes for the actions
  // the platform supports) and redirects to it: Twitch grants all four actions; Kick
  // supports timeout/ban/unban (no single-message delete); YouTube is ban-only. Discord
  // is different — its moderation authority is a guild-level BOT permission, so "enabling"
  // it is a bot RE-INVITE with the elevated permissions, not an OAuth re-consent.
  const enableModeration = useCallback(
    async (platform: string) => {
      try {
        if (platform === 'discord') {
          await startDiscordModerationReinvite()
          return
        }
        let url: string | null = null
        if (platform === 'twitch') {
          url = await moderationApi.getTwitchConsentUrl(id, ['delete', 'timeout', 'ban', 'unban'])
        } else if (platform === 'kick') {
          url = await moderationApi.getKickConsentUrl(id, ['timeout', 'ban', 'unban'])
        } else if (platform === 'youtube') {
          url = await moderationApi.getYouTubeConsentUrl(id, ['ban'])
        }
        if (url) window.location.href = url
      } catch {
        toast.error('Could not start moderation setup. Please try again.')
      }
    },
    [id]
  )

  // Re-login when a send fails with `reauth_required` (the platform OAuth token
  // was revoked). Re-runs the platform's OAuth login (the canonical entry the
  // landing page uses); falls back to Twitch when no platform is reported.
  const reauthenticate = useCallback(async (platform?: string) => {
    const p =
      platform === 'twitch' || platform === 'youtube' || platform === 'kick' ? platform : 'twitch'
    try {
      const url = await authApi.getLoginUrl(p)
      window.location.href = url
    } catch {
      toast.error('Could not start re-login. Please try again.')
    }
  }, [])

  // Restore the saved theme once on mount.
  useEffect(() => {
    const stored = localStorage.getItem(THEME_KEY)
    // eslint-disable-next-line react-hooks/set-state-in-effect -- one-time restore from localStorage
    if (stored === 'light') setLight(true)
  }, [])

  // Apply the theme: flip the body background (the layout paints dark by default)
  // and persist. Side-effects only — no setState.
  useEffect(() => {
    document.body.style.setProperty('background', light ? '#f8f9fa' : '#07070a', 'important')
    try {
      localStorage.setItem(THEME_KEY, light ? 'light' : 'dark')
    } catch {
      /* storage unavailable */
    }
  }, [light])

  // --- Moderation capability lookups ---------------------------------------

  const isOwner = capabilities?.is_owner === true
  // The moderation feature gate (ADR-0008): an owner outside the rollout cohort can
  // view the dashboard but gets no controls (the endpoints would 403 anyway).
  const moderationEnabled = isOwner && capabilities?.enabled === true
  const featureGated = isOwner && capabilities?.enabled === false
  const capabilitiesByChannel = useMemo(() => {
    const map = new Map<string, SourceCapability>()
    capabilities?.sources.forEach((s) => map.set(s.channel_id, s))
    return map
  }, [capabilities])

  // Sources the owner could moderate but hasn't granted the scope for.
  const missingScopeSources = useMemo(
    () => (capabilities?.sources ?? []).filter((s) => s.reason === 'missing_scope'),
    [capabilities]
  )

  // Whether any source can currently send chat from the monitor (gates the send
  // bar). TikTok/Discord have no send path, so only twitch/youtube/kick count.
  const hasSendableSource = useMemo(
    () =>
      (capabilities?.sources ?? []).some(
        (s) =>
          s.can_send === true &&
          (s.platform === 'twitch' || s.platform === 'youtube' || s.platform === 'kick')
      ),
    [capabilities]
  )

  // --- Optimistic moderation actions ---------------------------------------

  // Apply an optimistic mark + log entry, fire the API, and roll back on error.
  const runModeration = useCallback(
    async (meta: DeletionMetadata, call: () => Promise<unknown>, successMsg: string) => {
      const sig = deletionSignature(meta)
      const clientId = crypto.randomUUID()
      const entryId = (modSeqRef.current += 1)

      // Snapshot which items this mark touches so we can revert exactly them.
      const before = itemsRef.current
      const after = applyModerationMark(before, meta)
      const touched = after.filter((it, i) => it !== before[i]).map((it) => it.id)
      setItems((prev) => applyModerationMark(prev, meta))
      pendingDeletionsRef.current.add(sig)
      setModerationLog((prev) =>
        [...prev, { id: entryId, clientId, ...toModEntry(meta, 'live', Date.now()) }].slice(
          -MAX_MOD_LOG
        )
      )

      try {
        await call()
        toast.success(successMsg)
      } catch {
        // Roll back: drop the optimistic entry + clear the dedup signature, and
        // un-mark exactly the items we struck through.
        pendingDeletionsRef.current.delete(sig)
        setModerationLog((prev) => prev.filter((e) => e.clientId !== clientId))
        const touchedSet = new Set(touched)
        setItems((prev) =>
          prev.map((it) => (touchedSet.has(it.id) ? { ...it, _moderated: undefined } : it))
        )
        toast.error('Moderation action failed')
      }
    },
    []
  )

  const handleDelete = useCallback(
    (item: ViewItem) => {
      const req = buildDeleteRequest(item)
      const meta: DeletionMetadata = {
        deletion_type: 'single',
        target_uuid: req.target_uuid,
        target_msg_id: req.native_message_id,
      }
      void runModeration(meta, () => moderationApi.deleteMessage(id, req), 'Message deleted')
    },
    [id, runModeration]
  )

  const handleTimeout = useCallback(
    (item: ViewItem, durationSeconds: number) => {
      const req = buildTimeoutRequest(item, durationSeconds)
      const meta: DeletionMetadata = {
        deletion_type: 'batch',
        target_user_id: req.target_user_id,
        target_username: req.target_username,
        ban_duration: durationSeconds,
      }
      void runModeration(
        meta,
        () => moderationApi.timeoutUser(id, req),
        `Timed out ${req.target_username || 'user'}`
      )
    },
    [id, runModeration]
  )

  const handleBan = useCallback(
    (item: ViewItem) => {
      const req = buildBanRequest(item)
      const meta: DeletionMetadata = {
        deletion_type: 'batch',
        target_user_id: req.target_user_id,
        target_username: req.target_username,
        ban_duration: 0,
      }
      void runModeration(
        meta,
        () => moderationApi.banUser(id, req),
        `Banned ${req.target_username || 'user'}`
      )
    },
    [id, runModeration]
  )

  // Unban has no message-level visual mark; just fire and toast.
  const handleUnban = useCallback(
    (item: ViewItem) => {
      const name = item.user?.display_name || item.user?.username || 'user'
      moderationApi
        .unbanUser(id, buildUnbanRequest(item))
        .then(() => toast.success(`Unbanned ${name}`))
        .catch(() => toast.error('Unban failed'))
    },
    [id]
  )

  // Only owners in the rollout cohort get live action callbacks; everyone else
  // (non-owners, or owners the feature gate hasn't reached) views read-only.
  const moderation: ChatPanelModeration | undefined = moderationEnabled
    ? {
        onDelete: handleDelete,
        onTimeout: handleTimeout,
        onBan: handleBan,
        onUnban: handleUnban,
      }
    : undefined

  const { chat, events, system } = partitionItems(items)
  const sourceNames = Array.from(sources.values()).map((s) => s.channelName)
  const title = sourceNames.length > 0 ? sourceNames.join(' · ') : 'Overlay Monitor'

  return (
    <div
      id="overlay-view-root"
      className={clsx('overlay-view flex h-screen min-h-0 flex-col', light && 'light')}
    >
      {/* Header */}
      <header className="flex flex-wrap items-center gap-3 border-b border-border bg-surface px-4 py-2">
        <div className="flex min-w-0 items-center gap-3">
          <h1 className="min-w-0 truncate text-sm font-semibold text-text" title={title}>
            {title}
          </h1>
          <ConnectionBadge status={connectionStatus} attempts={reconnectAttempts} />
        </div>

        <div className="ml-auto flex flex-wrap items-center gap-2">
          {sources.size > 0 && (
            <PlatformStatusIndicators
              configuredSources={sources}
              activeChannels={activeChannels}
              channelStatuses={channelStatuses}
              variant="inline"
            />
          )}
          <MaintenanceInfoButton />
          <button
            onClick={() => setShowDetails((v) => !v)}
            aria-pressed={showDetails}
            className={clsx(
              'flex items-center gap-2 rounded-lg border px-3 py-1.5 text-xs font-medium transition-colors focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none',
              showDetails
                ? 'border-border-md bg-surface-2 text-text'
                : 'border-border text-text-sub hover:border-border-md hover:text-text'
            )}
          >
            <SlidersHorizontal className="h-3.5 w-3.5" />
            Details
          </button>
          <LayoutPicker layout={layout} onChange={updateLayout} />
          <ViewSettingsBar prefs={prefs} onChange={updatePrefs} />
          <OverlayViewThemeToggle light={light} onToggle={() => setLight((v) => !v)} />
          <Link
            href={`/overlay/${id}`}
            target="_blank"
            rel="noreferrer"
            className="flex items-center gap-2 rounded-lg border border-border px-3 py-1.5 text-xs font-medium text-text-sub transition-colors hover:border-border-md hover:text-text focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
          >
            <ExternalLink className="h-3.5 w-3.5" />
            OBS overlay
          </Link>
        </div>
      </header>

      {/* Non-owner notice: viewing is allowed, moderation is not. */}
      {capabilities && !isOwner && (
        <div className="flex items-center gap-2 border-b border-border bg-surface-2 px-4 py-2 text-xs text-text-sub">
          <Info className="h-3.5 w-3.5 shrink-0 text-text-dim" />
          You can view this monitor but aren&apos;t its owner — moderation is disabled.
        </div>
      )}

      {/* Feature-gated notice: owner is outside the moderation rollout cohort. */}
      {featureGated && (
        <div className="flex flex-wrap items-center gap-2 border-b border-border bg-surface-2 px-4 py-2 text-xs text-text-sub">
          <Info className="h-3.5 w-3.5 shrink-0 text-text-dim" />
          <span>Chat moderation is a premium feature.</span>
          <Link
            href="/upgrade"
            className="font-medium text-twitch hover:underline focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
          >
            Upgrade to moderate from your overlay
          </Link>
        </div>
      )}

      {/* Missing-scope notices: owner must grant permissions per platform. */}
      {moderationEnabled &&
        missingScopeSources.map((s) => (
          <div
            key={s.channel_id}
            className="flex flex-wrap items-center gap-2 border-b border-border bg-surface-2 px-4 py-2 text-xs text-text-sub"
          >
            <Info className="h-3.5 w-3.5 shrink-0 text-text-dim" />
            <span>
              {s.platform === 'discord'
                ? `Re-invite the bot with moderation permissions to enable mod actions for ${s.platform}`
                : `Grant moderation permissions to enable mod actions for ${s.platform}`}
              {s.channel_name ? ` (${s.channel_name})` : ''}.
            </span>
            {s.platform === 'twitch' ||
            s.platform === 'kick' ||
            s.platform === 'youtube' ||
            s.platform === 'discord' ? (
              <button
                type="button"
                onClick={() => enableModeration(s.platform)}
                className="font-medium text-twitch hover:underline focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
              >
                {s.platform === 'discord'
                  ? 'Re-invite the bot'
                  : 'Enable moderation & chat sending'}
              </button>
            ) : (
              <span className="text-text-dim">(coming soon for {s.platform})</span>
            )}
          </div>
        ))}

      {showDetails && (
        <ObservabilitySummary
          config={config}
          sources={sources}
          activeChannels={activeChannels}
          eventSettings={eventSettings}
          observedEventTypes={observedEventTypes}
        />
      )}

      {/* Resizable Chat | Activity — orientation/order driven by the layout picker. */}
      <ResizableSplit
        storageKey={`overlay-view-split-${id}`}
        orientation={LAYOUT_CONFIG[layout].orientation}
        reversed={LAYOUT_CONFIG[layout].reversed}
        left={
          <ChatPanel
            items={chat}
            prefs={prefs}
            capabilities={capabilitiesByChannel}
            moderation={moderation}
          />
        }
        right={<ActivityPanel events={events} system={system} moderationLog={moderationLog} />}
      />

      {/* Send bar — owner only, and only when ≥1 platform source can send. */}
      {isOwner && hasSendableSource && capabilities && (
        <ChatSendBar
          sources={capabilities.sources}
          onEnable={enableModeration}
          onReauth={reauthenticate}
        />
      )}
    </div>
  )
}
