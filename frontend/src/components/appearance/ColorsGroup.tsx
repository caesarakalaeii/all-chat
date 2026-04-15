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
import { ColorPickerControl } from './ColorPickerControl'

export interface ColorsGroupProps {
  visualSettings: Partial<VisualSettings>
  onChange: (patch: Partial<VisualSettings>) => void
}

export function ColorsGroup({ visualSettings, onChange }: ColorsGroupProps): React.ReactElement {
  return (
    <div className="space-y-3">
      <ColorPickerControl
        label="Message color"
        value={visualSettings.messageColor ?? '#ffffff'}
        onChange={(hex) => onChange({ messageColor: hex })}
      />
      <ColorPickerControl
        label="Username color"
        value={visualSettings.usernameColor ?? '#a0a0ff'}
        onChange={(hex) => onChange({ usernameColor: hex })}
      />
      <ColorPickerControl
        label="Timestamp color"
        value={visualSettings.timestampColor ?? '#888888'}
        onChange={(hex) => onChange({ timestampColor: hex })}
      />
    </div>
  )
}
