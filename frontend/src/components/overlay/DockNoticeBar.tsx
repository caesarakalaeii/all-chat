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

import { ChevronDown, ChevronUp, Info } from 'lucide-react'
import { useState } from 'react'

import { useTranslations } from '@/lib/i18n'

/**
 * Compact container for the monitor's notice strips in dock mode.
 *
 * The wide view stacks each notice as its own full-width bar — eight of them
 * are possible at once (reconnecting, truncated replay, no role, feature gate,
 * per-source consent, Discord link, missing scopes, mod-log opt-in, re-auth),
 * and each is a ~33px row. At dock width that is the entire panel, so several
 * notices collapse behind one summary row here.
 *
 * Collapsing, never dropping: a notice a moderator cannot see is worse than a
 * tall header, so the expanded bar contains exactly the children the wide view
 * would have stacked, and `count` decides only how they are presented.
 */
export function DockNoticeBar({
  count,
  children,
}: {
  /** How many notices `children` contains. Drives presentation only. */
  count: number
  children: React.ReactNode
}) {
  const t = useTranslations()
  const [expanded, setExpanded] = useState(false)

  if (count === 0) return null

  // One notice is a single short line: summarising it would cost a click to
  // learn something that fits on screen anyway.
  if (count === 1) {
    return <div className="border-b border-border bg-surface-2">{children}</div>
  }

  return (
    <div className="border-b border-border bg-surface-2">
      <button
        type="button"
        onClick={() => setExpanded((v) => !v)}
        aria-expanded={expanded}
        className="flex w-full items-center gap-2 px-4 py-2 text-xs text-text-sub focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
      >
        <Info className="h-3.5 w-3.5 shrink-0 text-text-dim" />
        <span>{t('viewerOverlay.monitor.dock.noticesSummary', { count })}</span>
        {expanded ? (
          <ChevronUp className="ml-auto h-3.5 w-3.5 shrink-0" />
        ) : (
          <ChevronDown className="ml-auto h-3.5 w-3.5 shrink-0" />
        )}
      </button>
      {/* Capped and scrollable: expanding eight notices must not push chat off
          the panel either. */}
      {expanded && (
        <div
          aria-label={t('viewerOverlay.monitor.dock.noticesLabel')}
          className="max-h-48 overflow-y-auto"
        >
          {children}
        </div>
      )}
    </div>
  )
}
