'use client'

import React from 'react'

export interface SliderControlProps {
  label: string
  value: number
  min: number
  max: number
  step: number
  unit?: string
  onChange: (v: number) => void
}

export function SliderControl({ label, value, min, max, step, unit, onChange }: SliderControlProps): React.ReactElement {
  return (
    <div className="flex items-center gap-2">
      <span className="w-28 shrink-0 text-sm text-text-sub">{label}</span>
      <input
        type="range"
        min={min}
        max={max}
        step={step}
        value={value}
        onChange={(e) => onChange(parseFloat(e.target.value))}
        className="flex-1 accent-current"
      />
      <span className="w-12 shrink-0 text-right text-sm text-text-dim tabular-nums">
        {value}{unit ?? ''}
      </span>
    </div>
  )
}
