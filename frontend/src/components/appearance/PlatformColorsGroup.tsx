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
import { RotateCcw } from 'lucide-react'
import type { VisualSettings } from '@/lib/types/visual-settings'
import { ColorPickerControl } from './ColorPickerControl'

export interface PlatformColorsGroupProps {
  visualSettings: Partial<VisualSettings>
  onChange: (patch: Partial<VisualSettings>) => void
}

const PLATFORMS: Array<{ field: keyof VisualSettings; label: string; brandDefault: string }> = [
  { field: 'twitchAccent',  label: 'Twitch',  brandDefault: '#9147FF' },
  { field: 'youtubeAccent', label: 'YouTube', brandDefault: '#FF0000' },
  { field: 'kickAccent',    label: 'Kick',    brandDefault: '#53FC18' },
  { field: 'tiktokAccent',  label: 'TikTok',  brandDefault: '#000000' },
  { field: 'discordAccent', label: 'Discord', brandDefault: '#5865F2' },
]

export function PlatformColorsGroup({ visualSettings, onChange }: PlatformColorsGroupProps): React.ReactElement {
  return (
    <div className="space-y-3">
      {PLATFORMS.map((p) => {
        const settings = visualSettings as Record<string, string | undefined>
        return (
          <div key={p.field} className="flex items-center gap-1">
            <ColorPickerControl
              label={p.label}
              value={settings[p.field] ?? p.brandDefault}
              onChange={(hex) => onChange({ [p.field]: hex })}
            />
            <button
              type="button"
              aria-label={`Reset ${p.label} accent`}
              onClick={() => onChange({ [p.field]: undefined })}
              className="rounded p-1 text-text-dim hover:text-text transition-colors"
            >
              <RotateCcw className="h-3 w-3" />
            </button>
          </div>
        )
      })}
    </div>
  )
}
