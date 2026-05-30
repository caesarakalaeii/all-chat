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

import { Ban, Clock, Eraser, Trash2 } from 'lucide-react'

import { CompactEvent } from '@/components/overlay/CompactEvent'
import type { ModEntry, ModKind, ViewItem } from '@/lib/utils/overlayViewModel'

const MOD_ICON: Record<ModKind, typeof Trash2> = {
  delete: Trash2,
  timeout: Clock,
  ban: Ban,
  clear: Eraser,
}

function modText(entry: ModEntry): string {
  const who = entry.username || entry.targetUserId || 'a user'
  switch (entry.kind) {
    case 'delete':
      return 'Message deleted'
    case 'timeout':
      return `Timed out ${who}${entry.banDuration ? ` for ${entry.banDuration}s` : ''}`
    case 'ban':
      return `Banned ${who}`
    case 'clear':
      return 'Chat cleared'
  }
}

function ModRow({ entry }: { entry: ModEntry }) {
  const Icon = MOD_ICON[entry.kind]
  return (
    <div className="flex items-center gap-2 border-b border-border/60 bg-youtube/5 px-3 py-1.5 text-sm">
      <span className="shrink-0 font-mono text-xs text-text-dim tabular-nums select-none">
        {new Date(entry.at).toLocaleTimeString()}
      </span>
      <Icon className="h-4 w-4 shrink-0 text-youtube" />
      <span className="rounded bg-youtube/15 px-1.5 py-0.5 text-[10px] font-semibold text-youtube uppercase">
        mod
      </span>
      <span className="min-w-0 flex-1 truncate text-text-sub">{modText(entry)}</span>
    </div>
  )
}

interface ActivityPanelProps {
  events: ViewItem[]
  system: ViewItem[]
  moderationLog: ModEntry[]
}

/**
 * Activity feed: audience events, system notices and moderation actions merged
 * into one newest-first chronological list (no scroll pinning needed — new
 * items appear at the top).
 */
export function ActivityPanel({ events, system, moderationLog }: ActivityPanelProps) {
  const entries: Array<{ id: string; at: number; node: React.ReactNode }> = [
    ...[...events, ...system].map((item, i) => ({
      id: `evt-${item.id}-${i}`,
      at: Date.parse(item.timestamp) || 0,
      node: <CompactEvent item={item} />,
    })),
    ...moderationLog.map((entry) => ({
      id: `mod-${entry.id}`,
      at: entry.at,
      node: <ModRow entry={entry} />,
    })),
  ].sort((a, b) => b.at - a.at)

  return (
    <section className="flex h-full min-h-0 flex-col bg-bg">
      <header className="flex items-center justify-between border-b border-border px-3 py-2">
        <span className="text-xs font-semibold tracking-wide text-text-sub uppercase">
          Activity &amp; Events
        </span>
        <span className="text-xs text-text-dim tabular-nums">{entries.length}</span>
      </header>
      <div className="min-h-0 flex-1 overflow-y-auto">
        {entries.length === 0 ? (
          <p className="px-3 py-6 text-center text-sm text-text-dim">No events yet.</p>
        ) : (
          entries.map((entry) => <div key={entry.id}>{entry.node}</div>)
        )}
      </div>
    </section>
  )
}
