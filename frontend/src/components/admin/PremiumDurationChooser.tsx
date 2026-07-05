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

import { useState } from 'react'
import clsx from 'clsx'
import { Input } from '@/components/ui/input'

export const DAY_SECONDS = 86400
// Mirrors the backend cap (~10 years) in share-service / auth-service.
export const MAX_DAYS = 3650

interface Preset {
  label: string
  seconds: number | null // null = permanent
}

const PRESETS: Preset[] = [
  { label: 'Permanent', seconds: null },
  { label: '1 day', seconds: 1 * DAY_SECONDS },
  { label: '7 days', seconds: 7 * DAY_SECONDS },
  { label: '30 days', seconds: 30 * DAY_SECONDS },
  { label: '90 days', seconds: 90 * DAY_SECONDS },
]

function presetKey(seconds: number | null): string {
  return seconds === null ? 'permanent' : String(seconds)
}

/**
 * customDaysToSeconds parses the custom day-count field into a grant duration.
 * A blank, non-numeric, non-positive, or over-cap value is invalid (seconds=null,
 * valid=false) so the caller can block submission; otherwise seconds is the exact
 * duration to send as duration_seconds. Pure — unit-tested independently of render.
 */
export function customDaysToSeconds(daysStr: string): { seconds: number | null; valid: boolean } {
  const days = Number(daysStr)
  const valid = daysStr.trim() !== '' && Number.isFinite(days) && days >= 1 && days <= MAX_DAYS
  return { seconds: valid ? Math.round(days * DAY_SECONDS) : null, valid }
}

export interface PremiumDurationChooserProps {
  /**
   * Fired whenever the selection changes. seconds=null means a permanent grant
   * (no duration_seconds sent). valid=false means the custom input is empty or
   * out of range — the caller should block submission until it is valid again.
   */
  onChange: (seconds: number | null, valid: boolean) => void
  disabled?: boolean
}

/**
 * PremiumDurationChooser lets an admin pick how long a premium grant lasts
 * (ADR-0027): a set of quick presets plus a custom day count. It defaults to
 * Permanent, matching the historical grant behaviour. It is uncontrolled — it owns
 * its own selection and reports changes via onChange; mount it fresh (e.g. inside a
 * dialog that unmounts on close) to reset it.
 */
export function PremiumDurationChooser({ onChange, disabled }: PremiumDurationChooserProps) {
  const [choice, setChoice] = useState<string>('permanent')
  const [customDays, setCustomDays] = useState('')

  function selectPreset(preset: Preset) {
    setChoice(presetKey(preset.seconds))
    onChange(preset.seconds, true)
  }

  function selectCustom(daysStr: string) {
    setChoice('custom')
    setCustomDays(daysStr)
    const { seconds, valid } = customDaysToSeconds(daysStr)
    onChange(seconds, valid)
  }

  const chipClass = (active: boolean) =>
    clsx(
      'rounded border px-3 py-1 text-sm transition-colors',
      active
        ? 'border-amber-400 bg-amber-400/10 text-amber-400'
        : 'border-border text-text-sub hover:border-amber-500/40 hover:text-text'
    )

  return (
    <div className="mt-4">
      <p className="mb-2 text-xs font-medium text-text-sub">Duration</p>
      <div className="flex flex-wrap gap-2">
        {PRESETS.map((preset) => (
          <button
            key={presetKey(preset.seconds)}
            type="button"
            disabled={disabled}
            onClick={() => selectPreset(preset)}
            className={chipClass(choice === presetKey(preset.seconds))}
          >
            {preset.label}
          </button>
        ))}
        <button
          type="button"
          disabled={disabled}
          onClick={() => selectCustom(customDays)}
          className={chipClass(choice === 'custom')}
        >
          Custom
        </button>
      </div>
      {choice === 'custom' && (
        <div className="mt-3 flex items-center gap-2">
          <Input
            type="number"
            min={1}
            max={MAX_DAYS}
            size="sm"
            value={customDays}
            disabled={disabled}
            placeholder="days"
            aria-label="Custom duration in days"
            onChange={(e) => selectCustom(e.target.value)}
            className="w-24"
          />
          <span className="text-xs text-text-dim">days (1&ndash;{MAX_DAYS})</span>
        </div>
      )}
    </div>
  )
}
