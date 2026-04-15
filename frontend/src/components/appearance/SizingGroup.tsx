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
