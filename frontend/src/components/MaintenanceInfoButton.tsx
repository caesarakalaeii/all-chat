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
import { Info } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'

import { maintenanceApi } from '@/lib/api/maintenance'
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

/**
 * Compact info "i" button for the overlay monitor header. Mirrors the dashboard's
 * MaintenanceBanner content, but stays out of the way: it renders nothing when there
 * are no announcements, and reveals the full maintenance notice(s) in a popover on
 * click. Shown to everyone viewing the monitor (owners and moderators alike).
 */
export function MaintenanceInfoButton() {
  const [windows, setWindows] = useState<MaintenanceWindow[]>([])
  const [open, setOpen] = useState(false)
  const containerRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    maintenanceApi
      .upcoming()
      .then(setWindows)
      .catch(() => {
        // Non-critical: if the announcement fetch fails, show no icon at all.
      })
  }, [])

  useEffect(() => {
    if (!open) return
    const handler = (e: MouseEvent) => {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        setOpen(false)
      }
    }
    document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [open])

  if (windows.length === 0) return null

  const hasActive = windows.some(isActive)

  return (
    <div className="relative" ref={containerRef}>
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        aria-expanded={open}
        aria-label="Service announcements"
        title="Service announcements"
        className={clsx(
          'flex items-center justify-center rounded-lg border p-1.5 transition-colors focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none',
          open
            ? 'border-border-md bg-surface-2 text-text'
            : hasActive
              ? 'border-amber-500/30 text-amber-300 hover:border-amber-500/50'
              : 'border-border text-text-sub hover:border-border-md hover:text-text'
        )}
      >
        <Info className="h-3.5 w-3.5" />
      </button>

      {open && (
        <div className="absolute right-0 z-50 mt-2 w-72 rounded-lg border border-border bg-surface p-3 shadow-lg">
          <p className="mb-2 text-[10px] font-semibold tracking-wide text-text-dim uppercase">
            Service announcements
          </p>
          <div className="flex flex-col gap-3">
            {windows.map((mw) => {
              const active = isActive(mw)
              return (
                <div key={mw.id} className="text-xs">
                  <p className={clsx('font-semibold', active ? 'text-amber-300' : 'text-blue-300')}>
                    {active ? 'Maintenance in progress' : 'Scheduled maintenance'}
                  </p>
                  <p className="mt-0.5 font-medium text-text">{mw.title}</p>
                  <p className="mt-0.5 text-text-sub">
                    {active
                      ? `Expected completion: ${DATE_FORMAT.format(new Date(mw.ends_at))}`
                      : `${DATE_FORMAT.format(new Date(mw.starts_at))} to ${DATE_FORMAT.format(
                          new Date(mw.ends_at)
                        )}`}
                  </p>
                  {mw.description && (
                    <p className="mt-1 leading-relaxed text-text-sub">{mw.description}</p>
                  )}
                </div>
              )
            })}
          </div>
        </div>
      )}
    </div>
  )
}
