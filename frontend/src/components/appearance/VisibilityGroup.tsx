'use client'

import React from 'react'
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

const ROWS: Array<{ field: keyof VisualSettings; label: string }> = [
  { field: 'showAvatars', label: 'Show avatars' },
  { field: 'showBadges', label: 'Show badges' },
  { field: 'showTimestamps', label: 'Show timestamps' },
  { field: 'showEmotes', label: 'Show emotes' },
  { field: 'showUsername', label: 'Show username' },
]

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
        <label key={opt.value} className="flex cursor-pointer items-center gap-1.5 text-xs text-text-sub">
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

const POSITION_OPTIONS: ReadonlyArray<RadioOption<'before' | 'after'>> = [
  { value: 'before', label: 'Before username' },
  { value: 'after', label: 'After username' },
]

const STYLE_OPTIONS: ReadonlyArray<RadioOption<'text' | 'icon'>> = [
  { value: 'text', label: 'Text' },
  { value: 'icon', label: 'Icon' },
]

const PRONOUN_POSITION_OPTIONS: ReadonlyArray<RadioOption<'before' | 'after'>> = [
  { value: 'before', label: 'Before username' },
  { value: 'after', label: 'After username' },
]

export function VisibilityGroup({ visualSettings, onChange, visibilityDefaults = {} }: VisibilityGroupProps): React.ReactElement {
  const platformBadgeVisible = isVisible(
    (visualSettings as Record<string, DisplayValue | undefined>)['showPlatformBadge'],
    isVisible((visibilityDefaults as Record<string, DisplayValue | undefined>)['showPlatformBadge'], true),
  )

  const platformIndicatorsVisible = isVisible(
    (visualSettings as Record<string, DisplayValue | undefined>)['showPlatformIndicators'],
    isVisible((visibilityDefaults as Record<string, DisplayValue | undefined>)['showPlatformIndicators'], true),
  )

  const badgePosition = visualSettings.platformBadgePosition ?? 'before'
  const badgeStyle = visualSettings.platformBadgeStyle ?? 'text'

  const pronounsVisible = isVisible(
    (visualSettings as Record<string, DisplayValue | undefined>)['showPronouns'],
    isVisible((visibilityDefaults as Record<string, DisplayValue | undefined>)['showPronouns'], true),
  )
  const pronounPosition = visualSettings.pronounPosition ?? 'after'
  const pronounColor = visualSettings.pronounColor ?? '#7B68EE'

  return (
    <div className="space-y-3">
      {ROWS.map((row) => {
        const settings = visualSettings as Record<string, DisplayValue | undefined>
        const defaults = visibilityDefaults as Record<string, DisplayValue | undefined>
        const defaultChecked = isVisible(defaults[row.field], true)
        const checked = isVisible(settings[row.field], defaultChecked)
        return (
          <ToggleSwitch
            key={row.field}
            label={row.label}
            checked={checked}
            onChange={(next) => onChange({ [row.field]: toDisplayValue(row.field, next) })}
          />
        )
      })}

      {/* Platform Badge */}
      <div className="border-t border-border pt-3">
        <ToggleSwitch
          label="Show platform badge"
          checked={platformBadgeVisible}
          onChange={(next) => onChange({ showPlatformBadge: next ? 'inline' : 'none' })}
        />
        <div className={`mt-2 space-y-2 pl-2 ${!platformBadgeVisible ? 'pointer-events-none opacity-40' : ''}`}>
          <div>
            <p className="mb-1 text-xs text-text-sub">Position</p>
            <RadioGroup
              name="platformBadgePosition"
              options={POSITION_OPTIONS}
              value={badgePosition}
              onChange={(val) => onChange({ platformBadgePosition: val })}
              disabled={!platformBadgeVisible}
            />
          </div>
          <div>
            <p className="mb-1 text-xs text-text-sub">Style</p>
            <RadioGroup
              name="platformBadgeStyle"
              options={STYLE_OPTIONS}
              value={badgeStyle}
              onChange={(val) => onChange({ platformBadgeStyle: val })}
              disabled={!platformBadgeVisible}
            />
          </div>
        </div>
      </div>

      {/* Platform Indicators */}
      <div className="border-t border-border pt-3">
        <ToggleSwitch
          label="Show platform indicators"
          checked={platformIndicatorsVisible}
          onChange={(next) => onChange({ showPlatformIndicators: next ? 'block' : 'none' })}
        />
      </div>

      {/* Pronouns — Phase 9 */}
      <div className="border-t border-border pt-3">
        <ToggleSwitch
          label="Show pronouns"
          checked={pronounsVisible}
          onChange={(next) => onChange({ showPronouns: next ? 'inline' : 'none' })}
        />
        <div className={`mt-2 space-y-2 pl-2 ${!pronounsVisible ? 'pointer-events-none opacity-40' : ''}`}>
          <div>
            <p className="mb-1 text-xs text-text-sub">Position</p>
            <RadioGroup
              name="pronounPosition"
              options={PRONOUN_POSITION_OPTIONS}
              value={pronounPosition}
              onChange={(val) => onChange({ pronounPosition: val })}
              disabled={!pronounsVisible}
            />
          </div>
          <div>
            <ColorPickerControl
              label="Pill color"
              value={pronounColor}
              onChange={(hex) => onChange({ pronounColor: hex })}
            />
          </div>
        </div>
      </div>
    </div>
  )
}
