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

import React, { useId } from 'react'

export interface SliderControlProps {
  label: string
  value: number
  min: number
  max: number
  step: number
  unit?: string
  onChange: (v: number) => void
}

export function SliderControl({
  label,
  value,
  min,
  max,
  step,
  unit,
  onChange,
}: SliderControlProps): React.ReactElement {
  // Explicit label/control association so the range input has an accessible
  // name; the native range element keeps 2.1.1/2.5.7 handled by the browser.
  const id = useId()
  return (
    <div className="flex items-center gap-2">
      <label htmlFor={id} className="w-28 shrink-0 text-sm text-text-sub">
        {label}
      </label>
      <input
        id={id}
        type="range"
        min={min}
        max={max}
        step={step}
        value={value}
        onChange={(e) => onChange(parseFloat(e.target.value))}
        className="flex-1 accent-current"
      />
      <span
        aria-hidden="true"
        className="w-12 shrink-0 text-right text-sm text-text-dim tabular-nums"
      >
        {value}
        {unit ?? ''}
      </span>
    </div>
  )
}
