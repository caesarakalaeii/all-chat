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

import { useEffect, useState } from 'react'
import { Wrench, X } from 'lucide-react'
import { maintenanceApi } from '@/lib/api/maintenance'
import { cn } from '@/lib/utils'
import type { MaintenanceWindow } from '@/lib/types/maintenance'

const DATE_FORMAT = new Intl.DateTimeFormat(undefined, {
  month: 'short',
  day: 'numeric',
  hour: '2-digit',
  minute: '2-digit',
})

function isActive(mw: MaintenanceWindow): boolean {
  const now = Date.now()
  return new Date(mw.starts_at).getTime() <= now && now <= new Date(mw.ends_at).getTime()
}

export function MaintenanceBanner() {
  const [windows, setWindows] = useState<MaintenanceWindow[]>([])
  const [dismissed, setDismissed] = useState<Set<string>>(new Set())

  useEffect(() => {
    maintenanceApi
      .upcoming()
      .then(setWindows)
      .catch(() => {
        // Silently fail — maintenance banner is non-critical
      })
  }, [])

  const visible = windows.filter((mw) => !dismissed.has(mw.id))

  if (visible.length === 0) return null

  return (
    <div className="mb-4 space-y-2">
      {visible.map((mw) => {
        const active = isActive(mw)
        return (
          <div
            key={mw.id}
            role="status"
            className={cn(
              'flex items-center gap-3 rounded-lg px-4 py-3 text-sm',
              active
                ? 'border border-amber-500/20 bg-amber-500/10 text-amber-300'
                : 'border border-blue-500/20 bg-blue-500/10 text-blue-300'
            )}
          >
            <Wrench className="size-4 shrink-0" aria-hidden="true" />
            <span className="min-w-0 flex-1">
              {active ? (
                <>
                  <strong>Maintenance in progress:</strong> {mw.title} — Expected completion:{' '}
                  {DATE_FORMAT.format(new Date(mw.ends_at))}
                </>
              ) : (
                <>
                  <strong>Scheduled maintenance:</strong> {mw.title} —{' '}
                  {DATE_FORMAT.format(new Date(mw.starts_at))} to{' '}
                  {DATE_FORMAT.format(new Date(mw.ends_at))}
                </>
              )}
              {mw.description && (
                <span className="ml-1 text-xs opacity-75">({mw.description})</span>
              )}
            </span>
            <button
              type="button"
              onClick={() => setDismissed((prev) => new Set(prev).add(mw.id))}
              aria-label={`Dismiss maintenance banner: ${mw.title}`}
              className="shrink-0 rounded p-0.5 opacity-60 transition-opacity hover:opacity-100 focus-visible:ring-2 focus-visible:ring-current focus-visible:outline-none"
            >
              <X className="size-3.5" aria-hidden="true" />
            </button>
          </div>
        )
      })}
    </div>
  )
}
