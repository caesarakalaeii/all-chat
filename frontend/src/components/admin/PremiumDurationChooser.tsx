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
import { Button } from '@/components/ui/button'
import { useTranslations, type MessageKey } from '@/lib/i18n'

export const DAY_SECONDS = 86400
// Mirrors the backend cap (~10 years) in share-service / auth-service.
export const MAX_DAYS = 3650

interface Preset {
  labelKey: MessageKey
  seconds: number | null // null = permanent
}

// The preset chips. `as const satisfies` rather than an annotation: an
// annotation widens labelKey to string, and a mistyped catalog key would then
// resolve at runtime to a key that echoes itself instead of failing tsc.
const PRESETS = [
  { labelKey: 'admin.premiumDuration.presetPermanent', seconds: null },
  { labelKey: 'admin.premiumDuration.preset1Day', seconds: 1 * DAY_SECONDS },
  { labelKey: 'admin.premiumDuration.preset7Days', seconds: 7 * DAY_SECONDS },
  { labelKey: 'admin.premiumDuration.preset30Days', seconds: 30 * DAY_SECONDS },
  { labelKey: 'admin.premiumDuration.preset90Days', seconds: 90 * DAY_SECONDS },
] as const satisfies readonly Preset[]

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
  const t = useTranslations()
  const [choice, setChoice] = useState<string>('permanent')
  const [customDays, setCustomDays] = useState('')

  function selectPreset(preset: (typeof PRESETS)[number]) {
    setChoice(presetKey(preset.seconds))
    onChange(preset.seconds, true)
  }

  function selectCustom(daysStr: string) {
    setChoice('custom')
    setCustomDays(daysStr)
    const { seconds, valid } = customDaysToSeconds(daysStr)
    onChange(seconds, valid)
  }

  // Layout and focus come from <Button variant="outline" size="xs">; this only
  // carries the selected/unselected colour.
  //
  // The amber is "premium" — the same amber the premium pill in
  // settings/viewer uses — and it is a raw palette colour because the design
  // system has no premium token yet (ADR-0056 left the 329 raw palette classes
  // for a semantic pass). Kept literal rather than mapped onto `warning`, which
  // means something else.
  const chipClass = (active: boolean) =>
    clsx(
      active
        ? 'border-premium bg-premium/10 text-premium'
        : 'border-border text-text-sub hover:border-premium/40 hover:text-text'
    )

  return (
    <div className="mt-4">
      <p className="mb-2 text-xs font-medium text-text-sub">{t('admin.premiumDuration.label')}</p>
      <div className="flex flex-wrap gap-2">
        {PRESETS.map((preset) => (
          <Button
            key={presetKey(preset.seconds)}
            type="button"
            disabled={disabled}
            onClick={() => selectPreset(preset)}
            variant="outline"
            size="xs"
            className={chipClass(choice === presetKey(preset.seconds))}
          >
            {t(preset.labelKey)}
          </Button>
        ))}
        <Button
          type="button"
          disabled={disabled}
          onClick={() => selectCustom(customDays)}
          variant="outline"
          size="xs"
          className={chipClass(choice === 'custom')}
        >
          {t('admin.premiumDuration.presetCustom')}
        </Button>
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
            placeholder={t('admin.premiumDuration.customPlaceholder')}
            aria-label={t('admin.premiumDuration.customFieldLabel')}
            onChange={(e) => selectCustom(e.target.value)}
            className="w-24"
          />
          <span className="text-xs text-text-dim">
            {t('admin.premiumDuration.customRange', { max: MAX_DAYS })}
          </span>
        </div>
      )}
    </div>
  )
}
