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
 *
 * Every color carries its own opacity: the row slider writes the alpha channel
 * into the hex value (`#rrggbbaa`, ADR-0050) rather than into a sibling
 * `*Opacity` setting, so themes reading `var(--chat-*-color)` honour it.
 */

import React from 'react'
import { HexColorPicker, HexColorInput } from 'react-colorful'
import { Popover } from '@/components/ui/popover'
import { alphaFromHex, hexWithAlpha, normalizeHex, stripAlpha } from '@/lib/utils/hex-alpha'

export interface ColorPickerControlProps {
  label: string
  value: string
  onChange: (hex: string) => void
}

export function ColorPickerControl({
  label,
  value,
  onChange,
}: ColorPickerControlProps): React.ReactElement {
  const alpha = alphaFromHex(value)
  const opacityPercent = Math.round(alpha * 100)

  // The saturation/hue picker only speaks 6-digit hex; keep the current alpha.
  const handleColorChange = (hex: string): void => {
    onChange(hexWithAlpha(hex, alpha))
  }

  // Typed hex wins when it spells out its own alpha (#rgba / #rrggbbaa),
  // otherwise the slider's opacity is preserved.
  const handleHexInput = (hex: string): void => {
    const normalized = normalizeHex(hex)
    onChange(
      normalized !== undefined && normalized.length === 9 ? normalized : hexWithAlpha(hex, alpha)
    )
  }

  return (
    <div className="flex items-center gap-2">
      <span className="w-28 shrink-0 text-sm text-text-sub">{label}</span>

      <Popover.Root>
        {/* Checkerboard sits behind the swatch so a translucent color reads as translucent */}
        <div className="alpha-checkerboard h-7 w-10 shrink-0 rounded">
          <Popover.Trigger
            data-testid="color-swatch"
            className="h-full w-full rounded border border-border focus-visible:ring-2 focus-visible:ring-twitch focus-visible:ring-offset-2 focus-visible:ring-offset-surface focus-visible:outline-none"
            style={{ backgroundColor: value }}
            aria-label={`Pick color for ${label}`}
          />
        </div>
        <Popover.Content className="space-y-2">
          <Popover.Title className="sr-only">{`Color for ${label}`}</Popover.Title>
          <HexColorPicker color={stripAlpha(value)} onChange={handleColorChange} />
          <HexColorInput
            color={value}
            onChange={handleHexInput}
            prefixed
            alpha
            aria-label={`Hex value for ${label}`}
            className="w-full rounded border border-border bg-bg px-2 py-1.5 font-mono text-sm text-text focus-visible:ring-1 focus-visible:ring-border focus-visible:outline-none"
          />
        </Popover.Content>
      </Popover.Root>

      <input
        type="range"
        min={0}
        max={100}
        step={1}
        value={opacityPercent}
        onChange={(e) => onChange(hexWithAlpha(value, Number(e.target.value) / 100))}
        className="flex-1 accent-current"
        aria-label={`${label} opacity`}
      />
    </div>
  )
}
