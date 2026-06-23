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

import { ArrowDown } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'

import { ChatRow, type ChatRowModeration } from '@/components/overlay/ChatRow'
import type { MonitorViewPrefs } from '@/app/overlay/[id]/view/viewPrefs'
import type { SourceCapability } from '@/lib/types/moderation'
import { shouldAutoScroll, type ViewItem } from '@/lib/utils/overlayViewModel'

/** Moderation wiring forwarded to each row, minus the per-row capability. */
export type ChatPanelModeration = Omit<ChatRowModeration, 'capability'>

interface ChatPanelProps {
  items: ViewItem[]
  /** View-local display prefs; omitted = all-on (existing callers stay unchanged). */
  prefs?: MonitorViewPrefs
  /** Per-source moderation capability, keyed by channel_id. */
  capabilities?: Map<string, SourceCapability>
  /** Moderation action callbacks; omit to render rows without mod controls. */
  moderation?: ChatPanelModeration
}

/**
 * Scrollable live-chat panel with Twitch-style pin-to-bottom: auto-scrolls to
 * the newest message unless the user has scrolled up to read history, in which
 * case a "Jump to latest" affordance appears. No smooth-scroll (instant, professional).
 */
export function ChatPanel({ items, prefs, capabilities, moderation }: ChatPanelProps) {
  const scrollRef = useRef<HTMLDivElement>(null)
  const pinnedRef = useRef(true)
  const [showJump, setShowJump] = useState(false)

  const handleScroll = () => {
    const el = scrollRef.current
    if (!el) return
    const pinned = shouldAutoScroll({
      scrollHeight: el.scrollHeight,
      scrollTop: el.scrollTop,
      clientHeight: el.clientHeight,
    })
    pinnedRef.current = pinned
    setShowJump(!pinned)
  }

  useEffect(() => {
    const el = scrollRef.current
    if (el && pinnedRef.current) el.scrollTop = el.scrollHeight
  }, [items])

  const jumpToLatest = () => {
    const el = scrollRef.current
    if (!el) return
    el.scrollTop = el.scrollHeight
    pinnedRef.current = true
    setShowJump(false)
  }

  return (
    <section className="relative flex h-full min-h-0 flex-col bg-bg">
      <header className="flex items-center justify-between border-b border-border px-3 py-2">
        <span className="text-xs font-semibold tracking-wide text-text-sub uppercase">Chat</span>
        <span className="text-xs text-text-dim tabular-nums">{items.length}</span>
      </header>
      <div ref={scrollRef} onScroll={handleScroll} className="min-h-0 flex-1 overflow-y-auto">
        {items.length === 0 ? (
          <p className="px-3 py-6 text-center text-sm text-text-dim">No chat messages yet.</p>
        ) : (
          items.map((item, i) => (
            <ChatRow
              key={`${item.id}-${i}`}
              item={item}
              prefs={prefs}
              moderation={
                moderation
                  ? { ...moderation, capability: capabilities?.get(item.channel_id) }
                  : undefined
              }
            />
          ))
        )}
      </div>
      {showJump && (
        <button
          onClick={jumpToLatest}
          className="absolute bottom-3 left-1/2 flex -translate-x-1/2 items-center gap-1.5 rounded-full border border-border-md bg-surface px-3 py-1.5 text-xs font-medium text-text shadow-lg hover:bg-surface-2 focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
        >
          <ArrowDown className="h-3.5 w-3.5" />
          Jump to latest
        </button>
      )}
    </section>
  )
}
