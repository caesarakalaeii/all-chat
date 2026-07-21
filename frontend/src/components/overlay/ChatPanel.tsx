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

import { ArrowDown, Filter, X } from 'lucide-react'
import { useEffect, useMemo, useRef, useState } from 'react'

import { ChatRow, type ChatRowModeration } from '@/components/overlay/ChatRow'
import type { MonitorViewPrefs } from '@/app/overlay/[id]/view/viewPrefs'
import type { SourceCapability } from '@/lib/types/moderation'
import {
  countNewItems,
  matchesUserFilter,
  shouldAutoScroll,
  userFilterFor,
  type UserFilter,
  type ViewItem,
} from '@/lib/utils/overlayViewModel'

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
 * the newest message unless the user has scrolled up to read history. Scrolling
 * up pauses the feed on a frozen snapshot — without the freeze, a fast chat
 * keeps trimming the capped buffer from the top and scrollback slides out from
 * under the reader. A "Chat paused" pill (with a count of new messages) resumes.
 *
 * Clicking a username narrows the panel to that platform identity (a 1:1
 * conversation view for fast chats); a filter bar shows who is selected and
 * clears it. No smooth-scroll (instant, professional).
 */
export function ChatPanel({ items, prefs, capabilities, moderation }: ChatPanelProps) {
  const scrollRef = useRef<HTMLDivElement>(null)
  const pinnedRef = useRef(true)
  // Frozen scrollback snapshot while the user reads history; null = live feed.
  const [paused, setPaused] = useState<ViewItem[] | null>(null)
  const [userFilter, setUserFilter] = useState<UserFilter | null>(null)

  const live = useMemo(
    () => (userFilter ? items.filter((m) => matchesUserFilter(m, userFilter)) : items),
    [items, userFilter]
  )
  const visible = useMemo(
    () => (paused ? (userFilter ? paused.filter((m) => matchesUserFilter(m, userFilter)) : paused) : live),
    [paused, userFilter, live]
  )
  const newCount = paused ? countNewItems(visible, live) : 0

  const handleScroll = () => {
    const el = scrollRef.current
    if (!el) return
    const pinned = shouldAutoScroll({
      scrollHeight: el.scrollHeight,
      scrollTop: el.scrollTop,
      clientHeight: el.clientHeight,
    })
    if (pinned === pinnedRef.current) return
    pinnedRef.current = pinned
    // Scrolling up freezes what is rendered; scrolling back to the bottom resumes.
    setPaused(pinned ? null : items)
  }

  useEffect(() => {
    const el = scrollRef.current
    if (el && pinnedRef.current) el.scrollTop = el.scrollHeight
  }, [visible])

  const resumeLive = () => {
    pinnedRef.current = true
    setPaused(null)
    const el = scrollRef.current
    if (el) el.scrollTop = el.scrollHeight
  }

  // Toggle the 1:1 filter to the clicked chatter (click the name again to clear)
  // and resume live pinned to the newest of their messages.
  const handleUserClick = (item: ViewItem) => {
    const next = userFilterFor(item)
    if (!next) return
    setUserFilter((prev) => (prev?.key === next.key ? null : next))
    resumeLive()
  }

  const clearFilter = () => {
    setUserFilter(null)
    resumeLive()
  }

  return (
    <section className="relative flex h-full min-h-0 flex-col bg-bg">
      <header className="flex items-center justify-between border-b border-border px-3 py-2">
        <span className="text-xs font-semibold tracking-wide text-text-sub uppercase">Chat</span>
        <span className="text-xs text-text-dim tabular-nums">
          {userFilter ? `${live.length} of ${items.length}` : items.length}
        </span>
      </header>
      {userFilter && (
        <div className="flex items-center gap-2 border-b border-border bg-surface-2 px-3 py-1.5 text-xs text-text-sub">
          <Filter className="h-3.5 w-3.5 shrink-0 text-text-dim" aria-hidden />
          <span className="min-w-0 truncate">
            Showing only messages from{' '}
            <span className="font-semibold text-text">{userFilter.label}</span>
          </span>
          <button
            type="button"
            onClick={clearFilter}
            className="ml-auto flex shrink-0 items-center gap-1 rounded font-medium text-twitch hover:underline focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
          >
            <X className="h-3.5 w-3.5" aria-hidden />
            Show all chat
          </button>
        </div>
      )}
      <div ref={scrollRef} onScroll={handleScroll} className="min-h-0 flex-1 overflow-y-auto">
        {visible.length === 0 ? (
          <p className="px-3 py-6 text-center text-sm text-text-dim">
            {userFilter ? `No messages from ${userFilter.label} yet.` : 'No chat messages yet.'}
          </p>
        ) : (
          visible.map((item, i) => (
            <ChatRow
              key={`${item.id}-${i}`}
              item={item}
              prefs={prefs}
              onUserClick={handleUserClick}
              moderation={
                moderation
                  ? { ...moderation, capability: capabilities?.get(item.channel_id) }
                  : undefined
              }
            />
          ))
        )}
      </div>
      {paused && (
        <button
          onClick={resumeLive}
          className="absolute bottom-3 left-1/2 flex -translate-x-1/2 items-center gap-1.5 rounded-full border border-border-md bg-surface px-3 py-1.5 text-xs font-medium text-text shadow-lg hover:bg-surface-2 focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
        >
          <ArrowDown className="h-3.5 w-3.5" />
          {newCount > 0 ? `Chat paused · ${newCount} new` : 'Chat paused'}
        </button>
      )}
    </section>
  )
}
