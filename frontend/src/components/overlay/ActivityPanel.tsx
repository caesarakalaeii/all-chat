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
import { Ban, Clock, Eraser, Gavel, ShieldAlert, Trash2 } from 'lucide-react'

import { CompactEvent } from '@/components/overlay/CompactEvent'
import { type TFunction, formatTime, useTranslations } from '@/lib/i18n'
import type { ModEntry, ModKind, ViewItem } from '@/lib/utils/overlayViewModel'

// Typographic quotation marks around the held message. Punctuation, not copy —
// the text between them is viewer-authored and never translated.
const OPEN_QUOTE = '“'
const CLOSE_QUOTE = '”'

const MOD_ICON: Record<ModKind, typeof Trash2> = {
  delete: Trash2,
  timeout: Clock,
  ban: Ban,
  clear: Eraser,
  automod: ShieldAlert,
  action: Gavel,
}

/**
 * One line describing a moderation action.
 *
 * The moderator, the timeout duration and the AutoMod category are each
 * independently optional, so every reachable combination is its own whole
 * sentence in the catalog rather than a stem with clauses appended. See
 * docs/frontend/I18N.md on why fragments are not concatenated.
 */
function modText(t: TFunction, entry: ModEntry): string {
  const user = entry.username || entry.targetUserId || t('viewerOverlay.activity.someUser')
  const moderator = entry.moderator
  switch (entry.kind) {
    case 'delete':
      return moderator
        ? t('viewerOverlay.activity.deletedBy', { moderator })
        : t('viewerOverlay.activity.deleted')
    case 'timeout': {
      const seconds = entry.banDuration
      if (seconds && moderator)
        return t('viewerOverlay.activity.timedOutForBy', { user, seconds, moderator })
      if (seconds) return t('viewerOverlay.activity.timedOutFor', { user, seconds })
      if (moderator) return t('viewerOverlay.activity.timedOutBy', { user, moderator })
      return t('viewerOverlay.activity.timedOut', { user })
    }
    case 'ban':
      return moderator
        ? t('viewerOverlay.activity.bannedBy', { user, moderator })
        : t('viewerOverlay.activity.banned', { user })
    case 'clear':
      return moderator
        ? t('viewerOverlay.activity.clearedBy', { moderator })
        : t('viewerOverlay.activity.cleared')
    case 'automod':
      if (!entry.resolution) {
        return entry.automodCategory
          ? t('viewerOverlay.activity.automodHeldCategory', {
              user,
              category: entry.automodCategory,
            })
          : t('viewerOverlay.activity.automodHeld', { user })
      }
      // resolvedBy is empty when the hold expired untouched, so there is no one
      // to name; the badge carries the outcome either way.
      return entry.resolvedBy
        ? t('viewerOverlay.activity.automodResolvedBy', {
            resolution: entry.resolution,
            moderator: entry.resolvedBy,
          })
        : t('viewerOverlay.activity.automodResolved', { resolution: entry.resolution })
    case 'action':
      // The action name is Twitch's, verbatim and possibly one we have never
      // seen. Show what the frame gave us rather than nothing. There is no
      // sentence here to translate, so this branch reads no catalog key.
      return [entry.moderator, entry.action, entry.username || entry.targetUserId]
        .filter(Boolean)
        .join(' ')
  }
}

function ModRow({ entry, t }: { entry: ModEntry; t: TFunction }) {
  const Icon = MOD_ICON[entry.kind]
  // An AutoMod hold is the one row that can still be waiting on a decision, so
  // it gets a second badge saying which: 'held' until it resolves, then the
  // outcome. Held text is the message a moderator needs in order to decide.
  // entry.resolution is a wire value (Twitch's own outcome name), so only the
  // still-waiting default is copy.
  const automodState =
    entry.kind === 'automod'
      ? (entry.resolution ?? t('viewerOverlay.activity.automodHeldBadge'))
      : null
  return (
    <div className="border-b border-border/60 bg-youtube/5 px-3 py-1.5 text-sm">
      <div className="flex items-center gap-2">
        <span className="shrink-0 font-mono text-xs text-text-dim tabular-nums select-none">
          {formatTime(new Date(entry.at))}
        </span>
        <Icon className="h-4 w-4 shrink-0 text-youtube" />
        <span className="rounded bg-youtube/15 px-1.5 py-0.5 text-[10px] font-semibold text-youtube uppercase">
          {t('viewerOverlay.activity.modBadge')}
        </span>
        {automodState && (
          <span
            className={clsx(
              'shrink-0 rounded px-1.5 py-0.5 text-[10px] font-semibold uppercase',
              entry.resolution
                ? 'bg-youtube/15 text-youtube'
                : 'border border-border/60 text-text-dim'
            )}
          >
            {automodState}
          </span>
        )}
        <span className="min-w-0 flex-1 truncate text-text-sub">{modText(t, entry)}</span>
      </div>
      {/* The held message is viewer-authored text, so only the quotation marks
          around it belong to the UI. */}
      {entry.heldText && (
        <p className="truncate pl-2 text-xs text-text-dim italic">
          {OPEN_QUOTE}
          {entry.heldText}
          {CLOSE_QUOTE}
        </p>
      )}
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
  const t = useTranslations()
  const entries: Array<{ id: string; at: number; node: React.ReactNode }> = [
    ...[...events, ...system].map((item, i) => ({
      id: `evt-${item.id}-${i}`,
      at: Date.parse(item.timestamp) || 0,
      node: <CompactEvent item={item} />,
    })),
    ...moderationLog.map((entry) => ({
      id: `mod-${entry.id}`,
      at: entry.at,
      node: <ModRow entry={entry} t={t} />,
    })),
  ].sort((a, b) => b.at - a.at)

  return (
    <section className="flex h-full min-h-0 flex-col bg-bg">
      <header className="flex items-center justify-between border-b border-border px-3 py-2">
        <span className="text-xs font-semibold tracking-wide text-text-sub uppercase">
          {t('viewerOverlay.activity.heading')}
        </span>
        <span className="text-xs text-text-dim tabular-nums">{entries.length}</span>
      </header>
      <div className="min-h-0 flex-1 overflow-y-auto">
        {entries.length === 0 ? (
          <p className="px-3 py-6 text-center text-sm text-text-dim">
            {t('viewerOverlay.activity.empty')}
          </p>
        ) : (
          entries.map((entry) => <div key={entry.id}>{entry.node}</div>)
        )}
      </div>
    </section>
  )
}
