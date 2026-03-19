'use client'

import React from 'react'
import type { VisualSettings } from '@/lib/types/visual-settings'
import { SliderControl } from './SliderControl'

export interface SizingGroupProps {
  visualSettings: Partial<VisualSettings>
  onChange: (patch: Partial<VisualSettings>) => void
}

export function SizingGroup({ visualSettings, onChange }: SizingGroupProps): React.ReactElement {
  return (
    <div className="space-y-3">
      <SliderControl
        label="Avatar size"
        value={parseFloat(visualSettings.avatarSize ?? '32')}
        min={16}
        max={64}
        step={2}
        unit="px"
        onChange={(v) => onChange({ avatarSize: `${v}px` })}
      />
      <SliderControl
        label="Badge size"
        value={parseFloat(visualSettings.badgeSize ?? '18')}
        min={12}
        max={32}
        step={2}
        unit="px"
        onChange={(v) => onChange({ badgeSize: `${v}px` })}
      />
      <SliderControl
        label="Emote scale"
        value={parseFloat(visualSettings.emoteScale ?? '1')}
        min={0.5}
        max={3.0}
        step={0.1}
        unit="×"
        onChange={(v) => onChange({ emoteScale: `${v}` })}
      />
      <p className="text-xs text-text-dim">
        Emote scale applies to third-party emotes (7TV, BTTV, FFZ). Standard emoji are not affected.
      </p>
    </div>
  )
}
