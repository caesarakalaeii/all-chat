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
import { Select } from '@base-ui/react/select'
import type { VisualSettings } from '@/lib/types/visual-settings'
import { FontFamilyCombobox } from './FontFamilyCombobox'
import { SliderControl } from './SliderControl'
import { AdvancedDisclosure } from '@/components/editor/AdvancedDisclosure'

interface FontWeightOption {
  label: string
  value: string
}

const FONT_WEIGHT_OPTIONS: FontWeightOption[] = [
  { label: '100 Thin', value: '100' },
  { label: '300 Light', value: '300' },
  { label: '400 Regular', value: '400' },
  { label: '500 Medium', value: '500' },
  { label: '600 SemiBold', value: '600' },
  { label: '700 Bold', value: '700' },
  { label: '800 ExtraBold', value: '800' },
  { label: '900 Black', value: '900' },
]

export interface TypographyGroupProps {
  visualSettings: Partial<VisualSettings>
  onChange: (patch: Partial<VisualSettings>) => void
}

export function TypographyGroup({
  visualSettings,
  onChange,
}: TypographyGroupProps): React.ReactElement {
  const fieldId = useId()
  const lineHeight = parseFloat(visualSettings.lineHeight ?? '1.5')
  const letterSpacing = parseFloat(visualSettings.letterSpacing?.replace('px', '') ?? '0')

  return (
    <div className="space-y-3">
      {/* Body Font Family */}
      <div className="flex flex-col gap-1">
        <span className="text-sm text-text-sub">Body Font</span>
        <FontFamilyCombobox
          value={visualSettings.fontFamily ?? null}
          onChange={(value) => onChange({ fontFamily: value ?? undefined })}
          aria-label="Body Font"
        />
      </div>

      {/* Username Font Family */}
      <div className="flex flex-col gap-1">
        <span className="text-sm text-text-sub">Username Font</span>
        <FontFamilyCombobox
          value={visualSettings.usernameFontFamily ?? null}
          onChange={(value) => onChange({ usernameFontFamily: value ?? undefined })}
          aria-label="Username Font"
        />
      </div>

      {/* Timestamp Font Family */}
      <div className="flex flex-col gap-1">
        <span className="text-sm text-text-sub">Timestamp Font</span>
        <FontFamilyCombobox
          value={visualSettings.timestampFontFamily ?? null}
          onChange={(value) => onChange({ timestampFontFamily: value ?? undefined })}
          aria-label="Timestamp Font"
        />
      </div>

      {/* Font Weight */}
      <div className="flex flex-col gap-1">
        <label htmlFor={`${fieldId}-font-weight`} className="text-sm text-text-sub">
          Font Weight
        </label>
        <Select.Root
          value={visualSettings.fontWeight ?? null}
          onValueChange={(value) => onChange({ fontWeight: value ?? undefined })}
        >
          <Select.Trigger
            id={`${fieldId}-font-weight`}
            className="flex w-full items-center justify-between rounded border border-border bg-bg px-2 py-1.5 text-sm text-text focus-visible:ring-1 focus-visible:ring-border focus-visible:outline-none"
          >
            <Select.Value placeholder="Select weight…" />
            <Select.Icon className="text-text-dim">
              <svg className="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M19 9l-7 7-7-7"
                />
              </svg>
            </Select.Icon>
          </Select.Trigger>
          <Select.Portal>
            <Select.Positioner className="z-[200]">
              <Select.Popup className="rounded border border-border bg-surface py-1 shadow-lg">
                <Select.List>
                  {FONT_WEIGHT_OPTIONS.map((opt) => (
                    <Select.Item
                      key={opt.value}
                      value={opt.value}
                      className="hover:bg-subtle data-[highlighted]:bg-subtle flex cursor-pointer items-center gap-2 px-3 py-1.5 text-sm text-text"
                    >
                      <Select.ItemIndicator className="w-4">
                        <svg className="h-3 w-3" fill="currentColor" viewBox="0 0 20 20">
                          <path d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" />
                        </svg>
                      </Select.ItemIndicator>
                      <Select.ItemText>{opt.label}</Select.ItemText>
                    </Select.Item>
                  ))}
                </Select.List>
              </Select.Popup>
            </Select.Positioner>
          </Select.Portal>
        </Select.Root>
      </div>

      {/* Body Font Size */}
      <div className="flex items-center gap-2">
        <label htmlFor={`${fieldId}-body-size`} className="w-28 shrink-0 text-sm text-text-sub">
          Body Size
        </label>
        <input
          id={`${fieldId}-body-size`}
          type="number"
          min={10}
          max={32}
          value={visualSettings.fontSize?.replace('px', '') ?? ''}
          onChange={(e) => onChange({ fontSize: `${e.target.value}px` })}
          aria-describedby={`${fieldId}-body-size-unit`}
          className="w-16 rounded border border-border bg-bg px-2 py-1 text-sm text-text focus-visible:ring-1 focus-visible:ring-border focus-visible:outline-none"
        />
        <span id={`${fieldId}-body-size-unit`} className="text-sm text-text-dim">
          px
        </span>
      </div>

      {/* Username Font Size */}
      <div className="flex items-center gap-2">
        <label htmlFor={`${fieldId}-username-size`} className="w-28 shrink-0 text-sm text-text-sub">
          Username Size
        </label>
        <input
          id={`${fieldId}-username-size`}
          type="number"
          value={visualSettings.usernameFontSize?.replace('px', '') ?? ''}
          onChange={(e) => onChange({ usernameFontSize: `${e.target.value}px` })}
          aria-describedby={`${fieldId}-username-size-unit`}
          className="w-16 rounded border border-border bg-bg px-2 py-1 text-sm text-text focus-visible:ring-1 focus-visible:ring-border focus-visible:outline-none"
        />
        <span id={`${fieldId}-username-size-unit`} className="text-sm text-text-dim">
          px
        </span>
      </div>

      {/* Timestamp Font Size */}
      <div className="flex items-center gap-2">
        <label
          htmlFor={`${fieldId}-timestamp-size`}
          className="w-28 shrink-0 text-sm text-text-sub"
        >
          Timestamp Size
        </label>
        <input
          id={`${fieldId}-timestamp-size`}
          type="number"
          value={visualSettings.timestampFontSize?.replace('px', '') ?? ''}
          onChange={(e) => onChange({ timestampFontSize: `${e.target.value}px` })}
          aria-describedby={`${fieldId}-timestamp-size-unit`}
          className="w-16 rounded border border-border bg-bg px-2 py-1 text-sm text-text focus-visible:ring-1 focus-visible:ring-border focus-visible:outline-none"
        />
        <span id={`${fieldId}-timestamp-size-unit`} className="text-sm text-text-dim">
          px
        </span>
      </div>

      {/* Low-traffic fine-tuning lives behind Advanced (ADR-0042) */}
      <AdvancedDisclosure count={2}>
        {/* Line Height */}
        <SliderControl
          label="Line Height"
          value={lineHeight}
          min={1.0}
          max={2.5}
          step={0.1}
          onChange={(v) => onChange({ lineHeight: String(v) })}
        />

        {/* Letter Spacing */}
        <SliderControl
          label="Letter Spacing"
          value={letterSpacing}
          min={-2}
          max={8}
          step={0.5}
          unit="px"
          onChange={(v) => onChange({ letterSpacing: `${v}px` })}
        />
      </AdvancedDisclosure>
    </div>
  )
}
