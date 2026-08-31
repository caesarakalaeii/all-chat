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
import { cn } from '@/lib/utils'
import { useTranslations } from '@/lib/i18n'
import type { VisualSettings } from '@/lib/types/visual-settings'
import { ToggleSwitch } from './ToggleSwitch'
import { ColorPickerControl } from './ColorPickerControl'

export interface VisibilityGroupProps {
  visualSettings: Partial<VisualSettings>
  onChange: (patch: Partial<VisualSettings>) => void
  visibilityDefaults?: Partial<VisualSettings>
}

type DisplayValue = 'inline' | 'block' | 'none'

function isVisible(val: DisplayValue | undefined, defaultOn: boolean): boolean {
  if (val === undefined) return defaultOn
  return val !== 'none'
}

function toDisplayValue(field: keyof VisualSettings, checked: boolean): DisplayValue {
  if (!checked) return 'none'
  if (field === 'showTimestamps' || field === 'showPlatformIndicators') return 'block'
  return 'inline'
}

// The toggle's label lives in the catalog keyed by the field, so a new field
// without a key fails tsc rather than shipping an unlabelled toggle.
const ROWS: ReadonlyArray<
  'showAvatars' | 'showBadges' | 'showTimestamps' | 'showEmotes' | 'showUsername'
> = ['showAvatars', 'showBadges', 'showTimestamps', 'showEmotes', 'showUsername']

interface RadioOption<T extends string> {
  value: T
  label: string
}

function RadioGroup<T extends string>({
  name,
  options,
  value,
  onChange,
  disabled,
}: {
  name: string
  options: ReadonlyArray<RadioOption<T>>
  value: T
  onChange: (val: T) => void
  disabled?: boolean
}): React.ReactElement {
  return (
    <div className="flex gap-4">
      {options.map((opt) => (
        <label
          key={opt.value}
          className="flex cursor-pointer items-center gap-1.5 text-xs text-text-sub"
        >
          <input
            type="radio"
            name={name}
            value={opt.value}
            checked={value === opt.value}
            onChange={() => onChange(opt.value)}
            className="accent-twitch"
            disabled={disabled}
          />
          {opt.label}
        </label>
      ))}
    </div>
  )
}

export function VisibilityGroup({
  visualSettings,
  onChange,
  visibilityDefaults = {},
}: VisibilityGroupProps): React.ReactElement {
  const t = useTranslations()

  // Built here rather than at module scope because the labels come from the
  // catalog, which is read through a hook.
  const positionOptions: ReadonlyArray<RadioOption<'before' | 'after'>> = [
    { value: 'before', label: t('overlayEditor.visibility.beforeUsername') },
    { value: 'after', label: t('overlayEditor.visibility.afterUsername') },
  ]
  const styleOptions: ReadonlyArray<RadioOption<'text' | 'icon'>> = [
    { value: 'text', label: t('overlayEditor.visibility.styleText') },
    { value: 'icon', label: t('overlayEditor.visibility.styleIcon') },
  ]

  const platformBadgeVisible = isVisible(
    (visualSettings as Record<string, DisplayValue | undefined>)['showPlatformBadge'],
    isVisible(
      (visibilityDefaults as Record<string, DisplayValue | undefined>)['showPlatformBadge'],
      true
    )
  )

  const platformIndicatorsVisible = isVisible(
    (visualSettings as Record<string, DisplayValue | undefined>)['showPlatformIndicators'],
    isVisible(
      (visibilityDefaults as Record<string, DisplayValue | undefined>)['showPlatformIndicators'],
      true
    )
  )

  const badgePosition = visualSettings.platformBadgePosition ?? 'before'
  const badgeStyle = visualSettings.platformBadgeStyle ?? 'text'

  const pronounsVisible = isVisible(
    (visualSettings as Record<string, DisplayValue | undefined>)['showPronouns'],
    isVisible(
      (visibilityDefaults as Record<string, DisplayValue | undefined>)['showPronouns'],
      true
    )
  )
  const pronounPosition = visualSettings.pronounPosition ?? 'after'
  const pronounColor = visualSettings.pronounColor ?? '#7B68EE'

  return (
    <div className="space-y-3">
      {ROWS.map((field) => {
        const settings = visualSettings as Record<string, DisplayValue | undefined>
        const defaults = visibilityDefaults as Record<string, DisplayValue | undefined>
        const defaultChecked = isVisible(defaults[field], true)
        const checked = isVisible(settings[field], defaultChecked)
        return (
          // data-setting-anchor: jump target for the editor settings search (ADR-0042)
          <div key={field} data-setting-anchor={field}>
            <ToggleSwitch
              label={t(`overlayEditor.visibility.${field}`)}
              checked={checked}
              onChange={(next) => onChange({ [field]: toDisplayValue(field, next) })}
            />
          </div>
        )
      })}

      {/* Platform Badge */}
      <div className="border-t border-border pt-3" data-setting-anchor="showPlatformBadge">
        <ToggleSwitch
          label={t('overlayEditor.visibility.showPlatformBadge')}
          checked={platformBadgeVisible}
          onChange={(next) => onChange({ showPlatformBadge: next ? 'inline' : 'none' })}
        />
        <div
          className={cn(
            'mt-2 space-y-2 pl-2',
            !platformBadgeVisible && 'pointer-events-none opacity-40'
          )}
        >
          <div>
            <p className="mb-1 text-xs text-text-sub">{t('overlayEditor.visibility.position')}</p>
            <RadioGroup
              name="platformBadgePosition"
              options={positionOptions}
              value={badgePosition}
              onChange={(val) => onChange({ platformBadgePosition: val })}
              disabled={!platformBadgeVisible}
            />
          </div>
          <div>
            <p className="mb-1 text-xs text-text-sub">{t('overlayEditor.visibility.style')}</p>
            <RadioGroup
              name="platformBadgeStyle"
              options={styleOptions}
              value={badgeStyle}
              onChange={(val) => onChange({ platformBadgeStyle: val })}
              disabled={!platformBadgeVisible}
            />
          </div>
        </div>
      </div>

      {/* Platform Indicators */}
      <div className="border-t border-border pt-3" data-setting-anchor="showPlatformIndicators">
        <ToggleSwitch
          label={t('overlayEditor.visibility.showPlatformIndicators')}
          checked={platformIndicatorsVisible}
          onChange={(next) => onChange({ showPlatformIndicators: next ? 'block' : 'none' })}
        />
      </div>

      {/* Pronouns — Phase 9 */}
      <div className="border-t border-border pt-3" data-setting-anchor="showPronouns">
        <ToggleSwitch
          label={t('overlayEditor.visibility.showPronouns')}
          checked={pronounsVisible}
          onChange={(next) => onChange({ showPronouns: next ? 'inline' : 'none' })}
        />
        <div
          className={cn(
            'mt-2 space-y-2 pl-2',
            !pronounsVisible && 'pointer-events-none opacity-40'
          )}
        >
          <div>
            <p className="mb-1 text-xs text-text-sub">{t('overlayEditor.visibility.position')}</p>
            <RadioGroup
              name="pronounPosition"
              options={positionOptions}
              value={pronounPosition}
              onChange={(val) => onChange({ pronounPosition: val })}
              disabled={!pronounsVisible}
            />
          </div>
          <div>
            <ColorPickerControl
              label={t('overlayEditor.visibility.pronounPillColor')}
              value={pronounColor}
              onChange={(hex) => onChange({ pronounColor: hex })}
            />
          </div>
        </div>
      </div>
    </div>
  )
}
