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

import { SliderControl } from '@/components/appearance/SliderControl'
import { ToggleSwitch } from '@/components/appearance/ToggleSwitch'
import type { MonitorViewPrefs } from '@/app/overlay/[id]/view/viewPrefs'
import { PRESET_NAMES } from '@/lib/utils/soundPlayer'

interface ViewSettingsBarProps {
  prefs: MonitorViewPrefs
  onChange: (prefs: MonitorViewPrefs) => void
  /** Preview the activity sound (also satisfies the browser's autoplay gesture). */
  onTestActivitySound?: () => void
}

/** The boolean-valued keys of MonitorViewPrefs (the plain on/off toggles). */
type BooleanPrefKey = {
  [K in keyof MonitorViewPrefs]: MonitorViewPrefs[K] extends boolean ? K : never
}[keyof MonitorViewPrefs]

/**
 * Display toggles, in the order they appear in the popover. Boolean-valued
 * `MonitorViewPrefs` keys only; the activity-sound controls are rendered
 * separately below because they carry volume/preset, not just on/off.
 */
const TOGGLES: ReadonlyArray<{ key: BooleanPrefKey; label: string }> = [
  { key: 'showAvatars', label: 'Avatars' },
  { key: 'showBadges', label: 'Badges' },
  { key: 'showPronouns', label: 'Pronouns' },
  { key: 'showTimestamps', label: 'Timestamps' },
  { key: 'showPlatformGlyph', label: 'Platform icon' },
  { key: 'showModeration', label: 'Moderation controls' },
]

function capitalize(s: string): string {
  return s.charAt(0).toUpperCase() + s.slice(1)
}

/**
 * Gear button + popover of view-local display toggles for the overlay monitor.
 * These are the moderator's personal preferences (persisted to localStorage by
 * the page) and never touch the overlay's saved visual settings.
 */
export function ViewSettingsBar({ prefs, onChange, onTestActivitySound }: ViewSettingsBarProps) {
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
        <div className="absolute right-0 z-50 mt-2 w-64 rounded-lg border border-border bg-surface p-3 shadow-lg">
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

          <div className="mt-3 border-t border-border pt-3">
            <p className="mb-2 text-[10px] font-semibold tracking-wide text-text-dim uppercase">
              Activity sound
            </p>
            <ToggleSwitch
              label="Sound on new activity"
              checked={prefs.activitySoundEnabled}
              onChange={(checked) => onChange({ ...prefs, activitySoundEnabled: checked })}
            />

            {prefs.activitySoundEnabled && (
              <div className="mt-2.5 flex flex-col gap-2.5">
                <div>
                  <label
                    htmlFor="activity-sound-preset"
                    className="mb-1 block text-sm text-text-sub"
                  >
                    Sound
                  </label>
                  <select
                    id="activity-sound-preset"
                    value={prefs.activitySoundPreset}
                    onChange={(e) =>
                      onChange({
                        ...prefs,
                        activitySoundPreset: e.target.value as MonitorViewPrefs['activitySoundPreset'],
                      })
                    }
                    className="w-full rounded-lg border border-border bg-surface px-3 py-1.5 text-sm text-text"
                  >
                    {PRESET_NAMES.map((name) => (
                      <option key={name} value={name}>
                        {capitalize(name)}
                      </option>
                    ))}
                  </select>
                </div>

                <SliderControl
                  label="Volume"
                  value={prefs.activitySoundVolume}
                  min={0}
                  max={1}
                  step={0.05}
                  unit=""
                  onChange={(v) => onChange({ ...prefs, activitySoundVolume: v })}
                />

                {onTestActivitySound && (
                  <button
                    type="button"
                    onClick={onTestActivitySound}
                    className="self-start rounded-lg border border-border bg-surface px-3 py-1.5 text-sm text-text hover:bg-surface-2 focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
                  >
                    Test sound
                  </button>
                )}
              </div>
            )}

            <p className="mt-2.5 text-xs text-text-dim">
              Plays only here, in this browser, so you notice easy-to-miss activity like
              channel-point redeems or a TikTok Rose. This is separate from your overlay&apos;s
              on-stream notification sounds.
            </p>
          </div>
        </div>
      )}
    </div>
  )
}
