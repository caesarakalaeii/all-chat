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

import { MoreVertical } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'

import { Button } from '@/components/ui/button'
import { useTranslations } from '@/lib/i18n'

/**
 * The one menu the monitor header's controls collapse into in dock mode.
 *
 * The wide header lays ten controls out with `flex-wrap`, which at dock width
 * gives roughly one control per line and leaves the panel with no room for
 * chat. Here the header keeps a single non-wrapping row and everything else
 * lives behind this trigger.
 *
 * Click-outside-to-close is hand-rolled the same way `ViewSettingsBar` does it,
 * so the two menus in this header behave identically.
 */
export function DockOverflowMenu({ children }: { children: React.ReactNode }) {
  const t = useTranslations()
  const [open, setOpen] = useState(false)
  const containerRef = useRef<HTMLDivElement>(null)

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

  return (
    <div className="relative" ref={containerRef}>
      <Button
        type="button"
        onClick={() => setOpen((v) => !v)}
        aria-expanded={open}
        aria-label={t('viewerOverlay.dock.menuLabel')}
        variant="outline"
        size="icon-sm"
      >
        <MoreVertical className="h-3.5 w-3.5" />
      </Button>

      {open && (
        <div className="absolute right-0 z-50 mt-2 flex w-56 flex-col items-stretch gap-2 rounded-lg border border-border bg-surface p-3 shadow-lg">
          {children}
        </div>
      )}
    </div>
  )
}
