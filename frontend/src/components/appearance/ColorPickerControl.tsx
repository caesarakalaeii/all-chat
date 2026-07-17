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

/**
 * Color control for the appearance panels. The swatch opens a ui/popover
 * (focus management, Escape, outside-press handled by the primitive — this
 * replaces the old hand-rolled fixed-position flyout with its own mousedown
 * listener) containing the visual picker plus a hex text input, so the color
 * is settable without dragging (WCAG 2.5.7) and by exact value.
 */

import React from 'react'
import { HexColorPicker, HexColorInput } from 'react-colorful'
import { Popover } from '@/components/ui/popover'

export interface ColorPickerControlProps {
  label: string
  value: string
  onChange: (hex: string) => void
  showOpacity?: boolean
  opacity?: string
  onOpacityChange?: (opacity: string) => void
}

export function ColorPickerControl({
  label,
  value,
  onChange,
  showOpacity,
  opacity,
  onOpacityChange,
}: ColorPickerControlProps): React.ReactElement {
  const opacityValue =
    showOpacity && opacity !== undefined ? Math.round(parseFloat(opacity) * 100) : 100

  return (
    <div className="flex items-center gap-2">
      <span className="w-28 shrink-0 text-sm text-text-sub">{label}</span>

      <Popover.Root>
        <Popover.Trigger
          data-testid="color-swatch"
          className="h-7 w-10 rounded border border-border focus-visible:ring-2 focus-visible:ring-twitch focus-visible:ring-offset-2 focus-visible:ring-offset-surface focus-visible:outline-none"
          style={{ backgroundColor: value }}
          aria-label={`Pick color for ${label}`}
        />
        <Popover.Content className="space-y-2">
          <Popover.Title className="sr-only">{`Color for ${label}`}</Popover.Title>
          <HexColorPicker color={value} onChange={onChange} />
          <HexColorInput
            color={value}
            onChange={onChange}
            prefixed
            aria-label={`Hex value for ${label}`}
            className="w-full rounded border border-border bg-bg px-2 py-1.5 font-mono text-sm text-text focus-visible:ring-1 focus-visible:ring-border focus-visible:outline-none"
          />
        </Popover.Content>
      </Popover.Root>

      {/* Opacity slider — only when showOpacity is true and callbacks are provided */}
      {showOpacity && opacity !== undefined && onOpacityChange && (
        <input
          type="range"
          min={0}
          max={100}
          step={1}
          value={opacityValue}
          onChange={(e) => onOpacityChange(String(Number(e.target.value) / 100))}
          className="flex-1 accent-current"
          aria-label={`${label} opacity`}
        />
      )}
    </div>
  )
}
