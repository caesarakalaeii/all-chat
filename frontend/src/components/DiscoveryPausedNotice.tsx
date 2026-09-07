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

import Link from 'next/link'
import { PauseCircle } from 'lucide-react'
import { useTranslations } from '@/lib/i18n'
import type { ChatSource } from '@/lib/types/overlay'

/**
 * hasPausedDiscovery reports whether any source in the list needs the streamer to act.
 *
 * Only an explicit 'paused' counts. An undefined discovery_status means the server could
 * not observe the state at all (no status snapshot, an expired one, or Redis unreadable),
 * and an undefined source list means the fetch has not finished — neither is evidence of
 * a problem, and neither is evidence of health.
 */
export function hasPausedDiscovery(sources: ChatSource[] | undefined): boolean {
  return (sources ?? []).some((source) => source.discovery_status === 'paused')
}

/**
 * DiscoveryPausedNotice warns on an overlay card that one of its channels has parked its
 * stream discovery, and links to the chat monitor — the only place Rediscover exists, and
 * the only thing that clears the park (a browser-source refresh does not).
 *
 * Renders nothing unless a source explicitly reports 'paused'. There is deliberately no
 * "all connected" counterpart: the dashboard cannot open a demand-bearing WebSocket to
 * learn live state without causing the very parking this reports, so silence here means
 * "no known problem", not "healthy".
 */
export function DiscoveryPausedNotice({
  overlayId,
  sources,
}: {
  overlayId: string
  sources: ChatSource[] | undefined
}) {
  const t = useTranslations()
  if (!hasPausedDiscovery(sources)) return null

  return (
    <div
      role="status"
      className="mb-3 flex items-start gap-2 rounded-lg border border-indigo-500/40 bg-indigo-500/10 px-3 py-2 text-xs"
    >
      <PauseCircle className="mt-0.5 size-3.5 shrink-0 text-indigo-400" aria-hidden="true" />
      <div className="min-w-0">
        <p className="font-medium text-text">{t('dashboard.discoveryPaused.title')}</p>
        <p className="mt-0.5 text-text-sub">{t('dashboard.discoveryPaused.body')}</p>
        <Link
          href={`/overlay/${overlayId}/view`}
          onClick={(e: React.MouseEvent) => e.stopPropagation()}
          className="mt-1 inline-block font-medium text-indigo-400 underline hover:text-indigo-300 focus-visible:ring-2 focus-visible:ring-current focus-visible:outline-none"
        >
          {t('dashboard.discoveryPaused.action')}
        </Link>
      </div>
    </div>
  )
}
