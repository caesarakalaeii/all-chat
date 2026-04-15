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
import type { VisualSettings } from '@/lib/types/visual-settings'
import { ToggleSwitch } from './ToggleSwitch'
import { SliderControl } from './SliderControl'

export interface EventsGroupProps {
  visualSettings: Partial<VisualSettings>
  onChange: (patch: Partial<VisualSettings>) => void
}

const EVENT_ROWS: Array<{
  label: string
  showField: keyof VisualSettings
  sizeField: keyof VisualSettings
}> = [
  { label: 'Super Chat',      showField: 'showSuperChat',      sizeField: 'superChatSizeModifier' },
  { label: 'Subscriptions',   showField: 'showSubscriptions',  sizeField: 'subscriptionSizeModifier' },
  { label: 'Raids',           showField: 'showRaids',          sizeField: 'raidSizeModifier' },
  { label: 'Bits',            showField: 'showBits',           sizeField: 'bitsSizeModifier' },
  { label: 'Membership Gift', showField: 'showMembershipGift', sizeField: 'membershipGiftSizeModifier' },
]

export function EventsGroup({ visualSettings, onChange }: EventsGroupProps): React.ReactElement {
  const settings = visualSettings as Record<string, string | undefined>

  return (
    <div className="space-y-4">
      {EVENT_ROWS.map((row) => {
        const showValue = settings[row.showField as string]
        const checked = showValue !== 'none'
        const sizeValue = parseFloat((settings[row.sizeField as string] as string) ?? '1')

        return (
          <div key={row.label} className="space-y-2">
            <ToggleSwitch
              label={row.label}
              checked={checked}
              onChange={(next) => onChange({ [row.showField]: next ? 'block' : 'none' })}
            />
            <SliderControl
              label="Size modifier"
              value={sizeValue}
              min={0.5}
              max={3.0}
              step={0.1}
              unit="×"
              onChange={(v) => onChange({ [row.sizeField]: `${v}` })}
            />
          </div>
        )
      })}
    </div>
  )
}
