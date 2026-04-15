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


import React, { useEffect, useRef, useState } from 'react'
import { HexColorPicker } from 'react-colorful'

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
  const [open, setOpen] = useState(false)
  const containerRef = useRef<HTMLDivElement>(null)
  const buttonRef = useRef<HTMLButtonElement>(null)
  const [popoverPos, setPopoverPos] = useState({ top: 0, left: 0 })

  useEffect(() => {
    if (!open) return
    const handler = (e: MouseEvent) => {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        setOpen(false)
      }
    }
    document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [open])

  useEffect(() => {
    if (!open || !buttonRef.current) return
    const rect = buttonRef.current.getBoundingClientRect()
    setPopoverPos({ top: rect.bottom + 4, left: rect.left })
  }, [open])

  const opacityValue =
    showOpacity && opacity !== undefined
      ? Math.round(parseFloat(opacity) * 100)
      : 100

  return (
    <div className="flex items-center gap-2" ref={containerRef}>
      <span className="w-28 shrink-0 text-sm text-text-sub">{label}</span>

      <div className="relative">
        {/* Color swatch button */}
        <button
          ref={buttonRef}
          type="button"
          data-testid="color-swatch"
          className="h-7 w-10 rounded border border-border"
          style={{ backgroundColor: value }}
          onClick={() => setOpen((prev) => !prev)}
          aria-label={`Pick color for ${label}`}
        />

        {/* Popover with color picker */}
        {open && (
          <div
            className="fixed z-[200] rounded border border-border bg-surface p-2 shadow-lg"
            style={{ top: popoverPos.top, left: popoverPos.left }}
          >
            <HexColorPicker color={value} onChange={onChange} />
          </div>
        )}
      </div>

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
