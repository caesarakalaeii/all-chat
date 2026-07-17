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

import React, { useState } from 'react'
import { Combobox } from '@base-ui/react/combobox'
import { Check } from 'lucide-react'

interface FontOption {
  label: string
  value: string
}

const SYSTEM_FONTS: FontOption[] = [
  { label: 'Inter', value: 'Inter' },
  { label: 'Arial', value: 'Arial' },
  { label: 'Helvetica', value: 'Helvetica' },
  { label: 'Georgia', value: 'Georgia' },
  { label: 'Courier New', value: 'Courier New' },
  { label: 'Impact', value: 'Impact' },
  { label: 'Trebuchet MS', value: 'Trebuchet MS' },
  { label: 'Verdana', value: 'Verdana' },
  { label: 'Tahoma', value: 'Tahoma' },
]

const GOOGLE_FONTS: FontOption[] = [
  { label: 'Bebas Neue', value: 'Bebas Neue' },
  { label: 'Oswald', value: 'Oswald' },
  { label: 'Rajdhani', value: 'Rajdhani' },
  { label: 'Barlow Condensed', value: 'Barlow Condensed' },
  { label: 'Exo 2', value: 'Exo 2' },
  { label: 'Nunito', value: 'Nunito' },
  { label: 'Poppins', value: 'Poppins' },
  { label: 'Roboto', value: 'Roboto' },
  { label: 'Open Sans', value: 'Open Sans' },
  { label: 'Montserrat', value: 'Montserrat' },
]

export const GOOGLE_FONT_NAMES: Set<string> = new Set(GOOGLE_FONTS.map((f) => f.value))

export interface FontFamilyComboboxProps {
  value: string | null
  onChange: (value: string | null) => void
  placeholder?: string
  /** Accessible name for the text input — the placeholder is only a hint. */
  'aria-label'?: string
}

export function FontFamilyCombobox({
  value,
  onChange,
  placeholder = 'Select font…',
  'aria-label': ariaLabel = 'Font family',
}: FontFamilyComboboxProps): React.ReactElement {
  const [inputValue, setInputValue] = useState<string>('')

  const filterFonts = (fonts: FontOption[]): FontOption[] => {
    if (!inputValue) return fonts
    const lower = inputValue.toLowerCase()
    return fonts.filter((f) => f.label.toLowerCase().includes(lower))
  }

  const filteredSystem = filterFonts(SYSTEM_FONTS)
  const filteredGoogle = filterFonts(GOOGLE_FONTS)
  const hasResults = filteredSystem.length > 0 || filteredGoogle.length > 0

  return (
    <Combobox.Root
      value={value}
      onValueChange={onChange}
      onInputValueChange={(nextInput) => setInputValue(nextInput)}
      autoHighlight
    >
      <div className="relative">
        <Combobox.Input
          className="w-full rounded border border-border bg-bg px-2 py-1.5 text-sm text-text placeholder:text-text-dim focus-visible:ring-1 focus-visible:ring-border focus-visible:outline-none"
          placeholder={placeholder}
          aria-label={ariaLabel}
        />
        <Combobox.Trigger
          className="absolute inset-y-0 right-0 flex items-center px-2 text-text-dim"
          aria-label="Open font picker"
        >
          <svg className="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
          </svg>
        </Combobox.Trigger>
      </div>
      <Combobox.Portal>
        <Combobox.Positioner className="z-[200]">
          <Combobox.Popup className="max-h-60 overflow-y-auto rounded border border-border bg-surface py-1 shadow-lg">
            {!hasResults && (
              <Combobox.Empty className="px-3 py-2 text-sm text-text-dim">
                No fonts found
              </Combobox.Empty>
            )}
            {filteredSystem.length > 0 && (
              <Combobox.Group>
                <Combobox.GroupLabel className="px-3 py-1 text-xs font-semibold tracking-wide text-text-dim uppercase">
                  System Fonts
                </Combobox.GroupLabel>
                {filteredSystem.map((font) => (
                  <Combobox.Item
                    key={font.value}
                    value={font.value}
                    className="hover:bg-subtle data-[highlighted]:bg-subtle flex cursor-pointer items-center gap-2 px-3 py-1.5 text-sm text-text"
                  >
                    <Combobox.ItemIndicator className="w-4">
                      <Check className="h-3 w-3" />
                    </Combobox.ItemIndicator>
                    <span style={{ fontFamily: font.value }}>{font.label}</span>
                  </Combobox.Item>
                ))}
              </Combobox.Group>
            )}
            {filteredGoogle.length > 0 && (
              <Combobox.Group>
                <Combobox.GroupLabel className="px-3 py-1 text-xs font-semibold tracking-wide text-text-dim uppercase">
                  Google Fonts
                </Combobox.GroupLabel>
                {filteredGoogle.map((font) => (
                  <Combobox.Item
                    key={font.value}
                    value={font.value}
                    className="hover:bg-subtle data-[highlighted]:bg-subtle flex cursor-pointer items-center gap-2 px-3 py-1.5 text-sm text-text"
                  >
                    <Combobox.ItemIndicator className="w-4">
                      <Check className="h-3 w-3" />
                    </Combobox.ItemIndicator>
                    <span style={{ fontFamily: font.value }}>{font.label}</span>
                  </Combobox.Item>
                ))}
              </Combobox.Group>
            )}
          </Combobox.Popup>
        </Combobox.Positioner>
      </Combobox.Portal>
    </Combobox.Root>
  )
}
