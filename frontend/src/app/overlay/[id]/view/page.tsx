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
 * Overlay Observability View (/overlay/[id]/view) — public, no auth.
 *
 * A Twitch-dashboard-inspired monitor for streamers: a resizable live Chat
 * panel + Activity feed, platform connection indicators, an overlay-config
 * summary, and its own light/dark mode. It reuses the exact realtime pipeline
 * the OBS overlay speaks (useOverlayStream) but renders a readable, animation-
 * free dashboard that ignores the overlay's CSS themes entirely.
 */

'use client'

import clsx from 'clsx'
import { ExternalLink, SlidersHorizontal } from 'lucide-react'
import Link from 'next/link'
import { use, useCallback, useEffect, useRef, useState } from 'react'

import PlatformStatusIndicators from '@/components/PlatformStatusIndicators'
import { ActivityPanel } from '@/components/overlay/ActivityPanel'
import { ChatPanel } from '@/components/overlay/ChatPanel'
import { ConnectionBadge } from '@/components/overlay/ConnectionBadge'
import { ObservabilitySummary } from '@/components/overlay/ObservabilitySummary'
import { OverlayViewThemeToggle } from '@/components/overlay/OverlayViewThemeToggle'
import { ResizableSplit } from '@/components/ResizableSplit'
import { useOverlayStream } from '@/hooks/useOverlayStream'
import type { ChatMessage, DeletionMetadata } from '@/lib/types/message'
import type { EventSettings } from '@/lib/types/overlay'
import {
  applyModerationMark,
  mergeByAgg,
  partitionItems,
  toModEntry,
  type ModEntry,
  type ViewItem,
} from '@/lib/utils/overlayViewModel'

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
  const modSeqRef = useRef(0)

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

  // Fetch per-overlay event toggles (public route; degrades gracefully if the
  // gateway hasn't enabled it — setState happens in a promise callback, so this
  // is not a synchronous set-state-in-effect).
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

      {showDetails && (
        <ObservabilitySummary
          config={config}
          sources={sources}
          activeChannels={activeChannels}
          eventSettings={eventSettings}
          observedEventTypes={observedEventTypes}
        />
      )}

      {/* Resizable Chat | Activity */}
      <ResizableSplit
        storageKey={`overlay-view-split-${id}`}
        left={<ChatPanel items={chat} />}
        right={<ActivityPanel events={events} system={system} moderationLog={moderationLog} />}
      />
    </div>
  )
}
