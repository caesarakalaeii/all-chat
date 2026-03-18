'use client'

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
          type="button"
          data-testid="color-swatch"
          className="h-7 w-10 rounded border border-border"
          style={{ backgroundColor: value }}
          onClick={() => setOpen((prev) => !prev)}
          aria-label={`Pick color for ${label}`}
        />

        {/* Popover with color picker */}
        {open && (
          <div className="absolute top-full left-0 z-50 mt-1 rounded border border-border bg-surface p-2 shadow-lg">
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
