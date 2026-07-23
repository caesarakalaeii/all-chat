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
import { cn } from '@/lib/utils'

export interface PreviewBackdropPickerProps {
  /** Current backdrop color, or null for the default app background */
  value: string | null
  onChange: (color: string | null) => void
}

const PRESETS: ReadonlyArray<{ label: string; value: string | null; swatchClass: string }> = [
  { label: 'Preview on app background', value: null, swatchClass: 'bg-bg' },
  { label: 'Preview on light background', value: '#f1f1f3', swatchClass: 'bg-[#f1f1f3]' },
  { label: 'Preview on chroma green', value: '#00b140', swatchClass: 'bg-[#00b140]' },
]

/**
 * Editor-only backdrop control for the live preview pane. The overlay embed
 * renders with a transparent background (OBS keys it), so the pane's
 * background shows through — letting streamers check chat readability
 * against something like their actual stream content. This is a preview aid,
 * NOT part of the overlay config: it is never saved to the overlay.
 */
export function PreviewBackdropPicker({
  value,
  onChange,
}: PreviewBackdropPickerProps): React.ReactElement {
  return (
    <div className="absolute top-3 right-3 z-10 flex items-center gap-1.5 rounded-full border border-border bg-surface/85 px-2.5 py-1.5 shadow-sm backdrop-blur-sm">
      <span className="text-[10px] font-medium tracking-widest text-text-sub uppercase select-none">
        Backdrop
      </span>
      {PRESETS.map((preset) => (
        <button
          key={preset.label}
          type="button"
          aria-label={preset.label}
          aria-pressed={value === preset.value}
          title={preset.label}
          onClick={() => onChange(preset.value)}
          className={cn(
            'h-5 w-5 rounded-full border transition-shadow focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none',
            preset.swatchClass,
            value === preset.value
              ? 'border-twitch ring-1 ring-twitch'
              : 'border-border-md hover:border-text-sub'
          )}
        />
      ))}
      <input
        type="color"
        aria-label="Custom preview background color"
        title="Custom preview background color"
        // A null value means "app background": show a neutral dark in the well
        value={value ?? '#020204'}
        onChange={(e) => onChange(e.target.value)}
        className="h-5 w-5 cursor-pointer rounded-full border border-border-md bg-transparent p-0 focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none [&::-moz-color-swatch]:rounded-full [&::-webkit-color-swatch]:rounded-full [&::-webkit-color-swatch]:border-0 [&::-webkit-color-swatch-wrapper]:p-0"
      />
    </div>
  )
}
