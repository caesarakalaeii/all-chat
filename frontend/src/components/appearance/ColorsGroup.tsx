'use client'

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
