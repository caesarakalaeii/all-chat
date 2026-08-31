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
import { useTranslations } from '@/lib/i18n'
import { emphasise } from '@/lib/i18n/emphasise'
import type { DisplaySettings } from '@/lib/types/overlay'
import { trackEvent } from '@/lib/analytics'

export const SOUND_PRESETS = PRESET_NAMES

export interface SoundGroupProps {
  displaySettings: Partial<DisplaySettings>
  onChange: (patch: Partial<DisplaySettings>) => void
  isPremium: boolean
  onPreview?: () => void
}

export function SoundGroup({
  displaySettings,
  onChange,
  isPremium,
  onPreview,
}: SoundGroupProps): React.ReactElement {
  const t = useTranslations()
  const enabled = displaySettings.notification_sound_enabled ?? false
  const preset = displaySettings.notification_sound_preset ?? 'chime'
  const volume = displaySettings.notification_sound_volume ?? 0.5
  const cooldown = displaySettings.notification_sound_cooldown ?? 500
  const customUrl = displaySettings.notification_sound_url ?? ''

  return (
    <div className="space-y-4">
      <p className="text-xs text-text-dim">{t('overlayEditor.sounds.scopeNote')}</p>
      <ToggleSwitch
        label={t('overlayEditor.sounds.enable')}
        checked={enabled}
        onChange={(checked) => {
          if (checked) trackEvent('sound_enabled')
          onChange({ notification_sound_enabled: checked })
        }}
      />

      {enabled && (
        <>
          <div>
            <p className="mb-1 text-sm text-text-sub">{t('overlayEditor.sounds.preset')}</p>
            <select
              aria-label={t('overlayEditor.sounds.preset')}
              value={preset}
              onChange={(e) => onChange({ notification_sound_preset: e.target.value })}
              className="rounded-lg border border-border bg-surface px-3 py-1.5 text-sm text-text"
            >
              {SOUND_PRESETS.map((name) => (
                <option key={name} value={name}>
                  {t(`common.soundPresets.${name}`)}
                </option>
              ))}
            </select>
          </div>

          <SliderControl
            label={t('overlayEditor.sounds.volume')}
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
              className="rounded-lg border border-border bg-surface px-3 py-1.5 text-sm text-text hover:bg-surface-2"
            >
              {t('overlayEditor.sounds.test')}
            </button>
          )}

          {/* Low-traffic fine-tuning lives behind Advanced (ADR-0042) */}
          <AdvancedDisclosure count={2}>
            <SliderControl
              label={t('overlayEditor.sounds.cooldown')}
              value={cooldown}
              min={100}
              max={5000}
              step={100}
              unit={t('overlayEditor.sounds.millisecondsUnit')}
              onChange={(v) => onChange({ notification_sound_cooldown: v })}
            />

            <div>
              <div className="mb-1 flex items-center gap-2">
                {!isPremium && <PremiumBadge />}
                <p className="text-sm text-text-sub">
                  {t('overlayEditor.sounds.customUrl')}
                  {!isPremium && (
                    <span className="ml-1 text-xs text-text-dim">
                      {emphasise(
                        t('overlayEditor.sounds.customUrlUpsell', {
                          emphasis: t('overlayEditor.sounds.customUrlUpsellEmphasis'),
                        }),
                        t('overlayEditor.sounds.customUrlUpsellEmphasis'),
                        (run) => (
                          <PremiumUpsellLink>{run}</PremiumUpsellLink>
                        )
                      )}
                    </span>
                  )}
                </p>
              </div>
              <input
                type="url"
                aria-label={t('overlayEditor.sounds.customUrl')}
                placeholder={t('overlayEditor.sounds.customUrlPlaceholder')}
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
