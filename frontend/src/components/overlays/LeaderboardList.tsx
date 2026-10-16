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

'use client'

/**
 * The list overlay renderer (issue #955, ADR-0072): a ranked top-supporter
 * board (bits, gifts or Super Chats). Initial state comes from the public
 * GET /overlays/:id/list/top; `type: "list_update"` frames, debounced by the
 * alert-processor, keep it live. OBS-transparent: only the rows paint.
 */

import { useCallback, useState } from 'react'

import { formatNumber, useTranslations } from '@/lib/i18n'
import type { ListTop, ListUpdateFrame } from '@/lib/types/goal'

import { applyListUpdate } from './goalList'

/**
 * The board, the onListUpdate callback for useOverlayStream, and seed for
 * the initial public read (usePublicSeed).
 */
export function useListBoard() {
  const [top, setTop] = useState<ListTop | null>(null)

  // A frame that arrived before the read is newer than it: keep the frame.
  const seed = useCallback((loaded: ListTop) => setTop((prev) => prev ?? loaded), [])

  // Before the first read lands a frame has no board to apply to; the read
  // then delivers the same state.
  const onListUpdate = useCallback((frame: ListUpdateFrame) => {
    setTop((prev) => (prev ? applyListUpdate(prev, frame) : prev))
  }, [])

  return { top, onListUpdate, seed }
}

export function LeaderboardList({ top }: { top: ListTop | null }) {
  const t = useTranslations()
  if (!top) return null
  const title = top.title || t(`viewerOverlay.listTitles.${top.category}`)

  return (
    <div className="h-screen w-full overflow-hidden bg-transparent p-6 text-white drop-shadow-lg">
      <h2 className="mb-3 truncate text-2xl font-bold">{title}</h2>
      {top.entries.length === 0 ? (
        <p className="text-lg">{t('viewerOverlay.list.empty')}</p>
      ) : (
        <ol className="space-y-2" aria-label={title}>
          {top.entries.map((entry) => (
            <li
              key={`${entry.rank}-${entry.platform}:${entry.name}`}
              className="flex animate-in items-center gap-3 rounded-lg bg-black/40 px-4 py-2 text-xl fade-in"
            >
              <span className="w-8 shrink-0 font-bold tabular-nums">{entry.rank}</span>
              <span className="min-w-0 flex-1 truncate font-semibold">{entry.name}</span>
              <span className="shrink-0 tabular-nums">{formatNumber(entry.amount)}</span>
            </li>
          ))}
        </ol>
      )}
    </div>
  )
}
