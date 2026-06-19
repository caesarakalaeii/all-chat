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
import { Settings } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'

import { ToggleSwitch } from '@/components/appearance/ToggleSwitch'
import type { MonitorViewPrefs } from '@/app/overlay/[id]/view/viewPrefs'

interface ViewSettingsBarProps {
  prefs: MonitorViewPrefs
  onChange: (prefs: MonitorViewPrefs) => void
}

/** Labels for each toggle, in the order they appear in the popover. */
const TOGGLES: ReadonlyArray<{ key: keyof MonitorViewPrefs; label: string }> = [
  { key: 'showAvatars', label: 'Avatars' },
  { key: 'showBadges', label: 'Badges' },
  { key: 'showPronouns', label: 'Pronouns' },
  { key: 'showTimestamps', label: 'Timestamps' },
  { key: 'showPlatformGlyph', label: 'Platform icon' },
  { key: 'showModeration', label: 'Moderation controls' },
]

/**
 * Gear button + popover of view-local display toggles for the overlay monitor.
 * These are the moderator's personal preferences (persisted to localStorage by
 * the page) and never touch the overlay's saved visual settings.
 */
export function ViewSettingsBar({ prefs, onChange }: ViewSettingsBarProps) {
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
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        aria-expanded={open}
        aria-label="Display settings"
        className={clsx(
          'flex items-center gap-2 rounded-lg border px-3 py-1.5 text-xs font-medium transition-colors focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none',
          open
            ? 'border-border-md bg-surface-2 text-text'
            : 'border-border text-text-sub hover:border-border-md hover:text-text'
        )}
      >
        <Settings className="h-3.5 w-3.5" />
        Display
      </button>

      {open && (
        <div className="absolute right-0 z-50 mt-2 w-56 rounded-lg border border-border bg-surface p-3 shadow-lg">
          <p className="mb-2 text-[10px] font-semibold tracking-wide text-text-dim uppercase">
            View settings
          </p>
          <div className="flex flex-col gap-2.5">
            {TOGGLES.map(({ key, label }) => (
              <ToggleSwitch
                key={key}
                label={label}
                checked={prefs[key]}
                onChange={(checked) => onChange({ ...prefs, [key]: checked })}
              />
            ))}
          </div>
        </div>
      )}
    </div>
  )
}
