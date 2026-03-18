'use client'

import React from 'react'
import type { VisualSettings } from '@/lib/types/visual-settings'
import { ToggleSwitch } from './ToggleSwitch'

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
  if (field === 'showTimestamps') return 'block'
  return 'inline'
}

const ROWS: Array<{ field: keyof VisualSettings; label: string }> = [
  { field: 'showAvatars', label: 'Show avatars' },
  { field: 'showBadges', label: 'Show badges' },
  { field: 'showTimestamps', label: 'Show timestamps' },
  { field: 'showPlatformBadge', label: 'Show platform badge' },
  { field: 'showEmotes', label: 'Show emotes' },
  { field: 'showUsername', label: 'Show username' },
]

export function VisibilityGroup({ visualSettings, onChange, visibilityDefaults = {} }: VisibilityGroupProps): React.ReactElement {
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
    </div>
  )
}
