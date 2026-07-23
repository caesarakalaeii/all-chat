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


import React from 'react'
import { ToggleSwitch } from './ToggleSwitch'
import { SliderControl } from './SliderControl'
import { AdvancedDisclosure } from '@/components/editor/AdvancedDisclosure'
import { PremiumBadge } from '@/components/PremiumBadge'
import { PremiumUpsellLink } from '@/components/PremiumUpsellLink'
import { PRESET_NAMES } from '@/lib/utils/soundPlayer'
import type { DisplaySettings } from '@/lib/types/overlay'
import { trackEvent } from '@/lib/analytics'

export const SOUND_PRESETS = PRESET_NAMES

export interface SoundGroupProps {
  displaySettings: Partial<DisplaySettings>
  onChange: (patch: Partial<DisplaySettings>) => void
  isPremium: boolean
  onPreview?: () => void
}

function capitalize(s: string): string {
  return s.charAt(0).toUpperCase() + s.slice(1)
}

export function SoundGroup({ displaySettings, onChange, isPremium, onPreview }: SoundGroupProps): React.ReactElement {
  const enabled = displaySettings.notification_sound_enabled ?? false
  const preset = displaySettings.notification_sound_preset ?? 'chime'
  const volume = displaySettings.notification_sound_volume ?? 0.5
  const cooldown = displaySettings.notification_sound_cooldown ?? 500
  const customUrl = displaySettings.notification_sound_url ?? ''

  return (
    <div className="space-y-4">
      <ToggleSwitch
        label="Enable notification sounds"
        checked={enabled}
        onChange={(checked) => {
          if (checked) trackEvent('sound_enabled')
          onChange({ notification_sound_enabled: checked })
        }}
      />

      {enabled && (
        <>
          <div>
            <p className="mb-1 text-sm text-text-sub">Sound preset</p>
            <select
              aria-label="Sound preset"
              value={preset}
              onChange={(e) => onChange({ notification_sound_preset: e.target.value })}
              className="rounded-lg border border-border bg-surface px-3 py-1.5 text-sm text-text"
            >
              {SOUND_PRESETS.map((name) => (
                <option key={name} value={name}>
                  {capitalize(name)}
                </option>
              ))}
            </select>
          </div>

          <SliderControl
            label="Volume"
            value={volume}
            min={0}
            max={1}
            step={0.05}
            unit=""
            onChange={(v) => onChange({ notification_sound_volume: v })}
          />

          {onPreview && (
            <button
              type="button"
              onClick={onPreview}
              className="rounded-lg border border-border bg-surface px-3 py-1.5 text-sm text-text hover:bg-surface-alt"
            >
              Test sound
            </button>
          )}

          {/* Low-traffic fine-tuning lives behind Advanced (ADR-0042) */}
          <AdvancedDisclosure count={2}>
            <SliderControl
              label="Cooldown"
              value={cooldown}
              min={100}
              max={5000}
              step={100}
              unit=" ms"
              onChange={(v) => onChange({ notification_sound_cooldown: v })}
            />

            <div>
              <div className="mb-1 flex items-center gap-2">
                {!isPremium && <PremiumBadge />}
                <p className="text-sm text-text-sub">
                  Custom sound URL
                  {!isPremium && (
                    <span className="ml-1 text-xs text-text-dim">
                      — Upload your own notification sound (
                      <PremiumUpsellLink>Premium</PremiumUpsellLink>)
                    </span>
                  )}
                </p>
              </div>
              <input
                type="url"
                aria-label="Custom sound URL"
                placeholder="https://example.com/sound.mp3"
                value={customUrl}
                disabled={!isPremium}
                onChange={(e) => onChange({ notification_sound_url: e.target.value })}
                className="w-full rounded-lg border border-border bg-surface px-3 py-1.5 text-sm text-text placeholder:text-text-dim disabled:cursor-not-allowed disabled:opacity-50"
              />
            </div>
          </AdvancedDisclosure>
        </>
      )}
    </div>
  )
}
