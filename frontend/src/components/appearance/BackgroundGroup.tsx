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
import type { VisualSettings } from '@/lib/types/visual-settings'
import { withLegacyOpacity } from '@/lib/utils/hex-alpha'
import { ColorPickerControl } from './ColorPickerControl'
import { SliderControl } from './SliderControl'
import { AdvancedDisclosure } from '@/components/editor/AdvancedDisclosure'

export interface BackgroundGroupProps {
  visualSettings: Partial<VisualSettings>
  onChange: (patch: Partial<VisualSettings>) => void
}

export function BackgroundGroup({ visualSettings, onChange }: BackgroundGroupProps): React.ReactElement {
  return (
    <div className="space-y-3">
      {/* Overlay background — opacity rides in the color's alpha channel; the
          legacy sibling *BgOpacity setting is folded in on read and cleared on
          write, so a saved value is never dimmed twice (ADR-0050). */}
      <p className="text-xs font-medium text-text-sub uppercase tracking-wide">Overlay background</p>
      <ColorPickerControl
        label="Overlay background"
        value={withLegacyOpacity(
          visualSettings.overlayBgColor ?? '#000000',
          visualSettings.overlayBgOpacity ?? '0.7'
        )}
        onChange={(hex) => onChange({ overlayBgColor: hex, overlayBgOpacity: undefined })}
      />

      {/* Bubble background */}
      <p className="text-xs font-medium text-text-sub uppercase tracking-wide">Bubble background</p>
      <ColorPickerControl
        label="Bubble background"
        value={withLegacyOpacity(
          visualSettings.bubbleBgColor ?? '#1a1a2e',
          visualSettings.bubbleBgOpacity ?? '0.85'
        )}
        onChange={(hex) => onChange({ bubbleBgColor: hex, bubbleBgOpacity: undefined })}
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
      {/* Low-traffic fine-tuning lives behind Advanced (ADR-0042) */}
      <AdvancedDisclosure count={3}>
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
      </AdvancedDisclosure>
    </div>
  )
}
