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
 * Labeled toggle row for the appearance panels. Built on ui/switch inside a
 * Field so the visible label IS the switch's accessible name (the previous
 * hand-rolled version rendered the label as an unassociated sibling span —
 * screen readers announced an unnamed switch). Clicking the label toggles.
 */

import React from 'react'
import { Field } from '@/components/ui/field'
import { Switch } from '@/components/ui/switch'

export interface ToggleSwitchProps {
  label: string
  checked: boolean
  onChange: (checked: boolean) => void
}

export function ToggleSwitch({ label, checked, onChange }: ToggleSwitchProps): React.ReactElement {
  return (
    <Field.Root className="flex-row items-center justify-between gap-2">
      <Field.Label className="text-sm font-normal text-text-sub">{label}</Field.Label>
      <Switch.Root checked={checked} onCheckedChange={onChange}>
        <Switch.Thumb />
      </Switch.Root>
    </Field.Root>
  )
}
