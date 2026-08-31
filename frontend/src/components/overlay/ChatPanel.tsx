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

import clsx from 'clsx'
import { ArrowDown, ArrowUp, Filter, X } from 'lucide-react'
import { useEffect, useMemo, useRef, useState } from 'react'

import { ChatRow, type ChatRowModeration } from '@/components/overlay/ChatRow'
import { DEFAULT_VIEW_PREFS, type MonitorViewPrefs } from '@/app/overlay/[id]/view/viewPrefs'
import { useTranslations } from '@/lib/i18n'
import { emphasise } from '@/lib/i18n/emphasise'
import type { SourceCapability } from '@/lib/types/moderation'
import { orderMessages } from '@/lib/utils/feedAnchor'
import {
  chatRowKeys,
  countNewItems,
  isPinnedToLiveEdge,
  matchesUserFilter,
  userFilterFor,
  type UserFilter,
  type ViewItem,
} from '@/lib/utils/overlayViewModel'

/** Moderation wiring forwarded to each row, minus the per-row capability. */
export type ChatPanelModeration = Omit<ChatRowModeration, 'capability'>

interface ChatPanelProps {
  items: ViewItem[]
  /** View-local display prefs; omitted = `DEFAULT_VIEW_PREFS` (existing callers stay unchanged). */
  prefs?: MonitorViewPrefs
  /** Per-source moderation capability, keyed by channel_id. */
  capabilities?: Map<string, SourceCapability>
  /** Moderation action callbacks; omit to render rows without mod controls. */
  moderation?: ChatPanelModeration
}

/**
 * Scrollable live-chat panel with Twitch-style pin-to-the-live-edge:
 * auto-scrolls to the newest message unless the user has scrolled away to read
 * history. Scrolling away pauses the feed on a frozen snapshot — without the
 * freeze, a fast chat keeps trimming the capped buffer and scrollback slides out
 * from under the reader. A "Chat paused" pill (with a count of new messages)
 * resumes.
 *
 * Two orders, chosen by `prefs.newestFirst`:
 *  - off (default): newest message at the BOTTOM, older ones above it, live
 *    edge at the bottom — the Twitch arrangement.
 *  - on: newest message at the TOP, older ones scrolling away downward, live
 *    edge at the top. For reading chat without looking down mid-stream.
 * Everything that depends on "where is the newest message" follows from that:
 * the pin edge, the auto-scroll target, and the side the paused pill sits on.
 *
 * Clicking a username narrows the panel to that platform identity (a 1:1
 * conversation view for fast chats); a filter bar shows who is selected and
 * clears it. No smooth-scroll (instant, professional).
 */
export function ChatPanel({ items, prefs, capabilities, moderation }: ChatPanelProps) {
  const t = useTranslations()
  const newestFirst = (prefs ?? DEFAULT_VIEW_PREFS).newestFirst
  const scrollRef = useRef<HTMLDivElement>(null)
  const pinnedRef = useRef(true)
  // Frozen scrollback snapshot while the user reads history; null = live feed.
  const [paused, setPaused] = useState<ViewItem[] | null>(null)
  const [userFilter, setUserFilter] = useState<UserFilter | null>(null)

  // Flipping the order moves the live edge to the opposite side of the panel, so
  // a snapshot taken under the old order would strand the reader at what is now
  // an arbitrary offset: discard it and resume live (the effect below scrolls to
  // the new edge). The discard has to be one-way. Masking a kept snapshot by
  // comparing orders instead looks equivalent but is not — flipping back
  // re-matches and resurrects it, re-freezing chat with a pill the reader never
  // asked for. This is React's adjust-state-when-a-prop-changes pattern; it
  // cannot be an effect or a render-time ref write, because eslint's
  // `react-hooks/set-state-in-effect` and `react-hooks/refs` forbid both.
  const [orderShown, setOrderShown] = useState(newestFirst)
  if (orderShown !== newestFirst) {
    setOrderShown(newestFirst)
    setPaused(null)
  }

  const live = useMemo(
    () => (userFilter ? items.filter((m) => matchesUserFilter(m, userFilter)) : items),
    [items, userFilter]
  )
  const visible = useMemo(
    () =>
      paused
        ? userFilter
          ? paused.filter((m) => matchesUserFilter(m, userFilter))
          : paused
        : live,
    [paused, userFilter, live]
  )
  const newCount = paused ? countNewItems(visible, live) : 0
  // Rows in render order, each carrying the key it got from the CHRONOLOGICAL
  // list: under newestFirst every arrival is a prepend, so an index-based key
  // would change for every row on every message and remount the whole buffer.
  // Keying before ordering is load-bearing, not stylistic — the row-identity
  // test in ChatPanel.test.tsx fails if these two lines are swapped.
  //
  // The ORDER axis is shared with the OBS overlay's
  // `display_settings.invert_message_order` — same pure helper, so the two
  // surfaces cannot drift on what "newest first" means.
  const rows = useMemo(() => {
    const keys = chatRowKeys(visible)
    return orderMessages(
      visible.map((item, i) => ({ item, key: keys[i] })),
      newestFirst
    )
  }, [visible, newestFirst])

  const handleScroll = () => {
    const el = scrollRef.current
    if (!el) return
    const pinned = isPinnedToLiveEdge(
      {
        scrollHeight: el.scrollHeight,
        scrollTop: el.scrollTop,
        clientHeight: el.clientHeight,
      },
      newestFirst
    )
    if (pinned === pinnedRef.current) return
    pinnedRef.current = pinned
    // Scrolling away from the live edge freezes what is rendered; scrolling back
    // to it resumes.
    setPaused(pinned ? null : items)
  }

  // The flip discards the snapshot (see `orderShown` above); re-pin to match, so
  // the effect below scrolls to the new live edge.
  useEffect(() => {
    pinnedRef.current = true
  }, [newestFirst])

  useEffect(() => {
    const el = scrollRef.current
    if (el && pinnedRef.current) el.scrollTop = newestFirst ? 0 : el.scrollHeight
  }, [visible, newestFirst])

  const resumeLive = () => {
    pinnedRef.current = true
    setPaused(null)
    const el = scrollRef.current
    if (el) el.scrollTop = newestFirst ? 0 : el.scrollHeight
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
        <span className="text-xs font-semibold tracking-wide text-text-sub uppercase">
          {t('viewerOverlay.chatPanel.heading')}
        </span>
        <span className="text-xs text-text-dim tabular-nums">
          {userFilter
            ? t('viewerOverlay.chatPanel.filteredCount', {
                shown: live.length,
                total: items.length,
              })
            : items.length}
        </span>
      </header>
      {userFilter && (
        <div className="flex items-center gap-2 border-b border-border bg-surface-2 px-3 py-1.5 text-xs text-text-sub">
          <Filter className="h-3.5 w-3.5 shrink-0 text-text-dim" aria-hidden />
          <span className="min-w-0 truncate">
            {emphasise(
              t('viewerOverlay.chatPanel.filteredBy', { user: userFilter.label }),
              userFilter.label,
              (run) => (
                <span className="font-semibold text-text">{run}</span>
              )
            )}
          </span>
          <button
            type="button"
            onClick={clearFilter}
            className="ml-auto flex shrink-0 items-center gap-1 rounded font-medium text-twitch hover:underline focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
          >
            <X className="h-3.5 w-3.5" aria-hidden />
            {t('viewerOverlay.chatPanel.showAll')}
          </button>
        </div>
      )}
      <div ref={scrollRef} onScroll={handleScroll} className="min-h-0 flex-1 overflow-y-auto">
        {visible.length === 0 ? (
          <p className="px-3 py-6 text-center text-sm text-text-dim">
            {userFilter
              ? t('viewerOverlay.chatPanel.filteredEmpty', { user: userFilter.label })
              : t('viewerOverlay.chatPanel.empty')}
          </p>
        ) : (
          rows.map(({ item, key }) => (
            <ChatRow
              key={key}
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
          className={clsx(
            'absolute left-1/2 flex -translate-x-1/2 items-center gap-1.5 rounded-full border border-border-md bg-surface px-3 py-1.5 text-xs font-medium text-text shadow-lg hover:bg-surface-2 focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none',
            // The pill belongs next to the live edge it scrolls back to.
            newestFirst ? 'top-3' : 'bottom-3'
          )}
        >
          {newestFirst ? (
            <ArrowUp className="h-3.5 w-3.5" />
          ) : (
            <ArrowDown className="h-3.5 w-3.5" />
          )}
          {newCount > 0 ? `Chat paused · ${newCount} new` : 'Chat paused'}
        </button>
      )}
    </section>
  )
}
