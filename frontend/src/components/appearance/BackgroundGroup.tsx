'use client'

import React from 'react'
import type { VisualSettings } from '@/lib/types/visual-settings'
import { ColorPickerControl } from './ColorPickerControl'
import { SliderControl } from './SliderControl'

export interface BackgroundGroupProps {
  visualSettings: Partial<VisualSettings>
  onChange: (patch: Partial<VisualSettings>) => void
}

export function BackgroundGroup({ visualSettings, onChange }: BackgroundGroupProps): React.ReactElement {
  return (
    <div className="space-y-3">
      {/* Overlay background */}
      <p className="text-xs font-medium text-text-sub uppercase tracking-wide">Overlay background</p>
      <ColorPickerControl
        label="Overlay background"
        value={visualSettings.overlayBgColor ?? '#000000'}
        onChange={(hex) => onChange({ overlayBgColor: hex })}
        showOpacity
        opacity={visualSettings.overlayBgOpacity ?? '0.7'}
        onOpacityChange={(op) => onChange({ overlayBgOpacity: op })}
      />

      {/* Bubble background */}
      <p className="text-xs font-medium text-text-sub uppercase tracking-wide">Bubble background</p>
      <ColorPickerControl
        label="Bubble background"
        value={visualSettings.bubbleBgColor ?? '#1a1a2e'}
        onChange={(hex) => onChange({ bubbleBgColor: hex })}
        showOpacity
        opacity={visualSettings.bubbleBgOpacity ?? '0.85'}
        onOpacityChange={(op) => onChange({ bubbleBgOpacity: op })}
      />

      {/* Border color */}
      <ColorPickerControl
        label="Border color"
        value={visualSettings.bubbleBorderColor ?? '#333333'}
        onChange={(hex) => onChange({ bubbleBorderColor: hex })}
      />

      {/* Sliders */}
      <SliderControl
        label="Border radius"
        value={parseFloat(visualSettings.bubbleBorderRadius ?? '8')}
        min={0}
        max={24}
        step={1}
        unit="px"
        onChange={(v) => onChange({ bubbleBorderRadius: `${v}px` })}
      />
      <SliderControl
        label="Border width"
        value={parseFloat(visualSettings.bubbleBorderWidth ?? '0')}
        min={0}
        max={8}
        step={1}
        unit="px"
        onChange={(v) => onChange({ bubbleBorderWidth: `${v}px` })}
      />
      <SliderControl
        label="Padding"
        value={parseFloat(visualSettings.bubblePadding ?? '8')}
        min={0}
        max={32}
        step={2}
        unit="px"
        onChange={(v) => onChange({ bubblePadding: `${v}px` })}
      />
      <SliderControl
        label="Message gap"
        value={parseFloat(visualSettings.messageGap ?? '4')}
        min={0}
        max={24}
        step={2}
        unit="px"
        onChange={(v) => onChange({ messageGap: `${v}px` })}
      />
      <SliderControl
        label="Backdrop blur"
        value={parseFloat(visualSettings.backdropBlur ?? '0')}
        min={0}
        max={20}
        step={1}
        unit="px"
        onChange={(v) => onChange({ backdropBlur: `${v}px` })}
      />
    </div>
  )
}
