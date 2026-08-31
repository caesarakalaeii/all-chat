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
import { useTranslations } from '@/lib/i18n'
import type { VisualSettings } from '@/lib/types/visual-settings'
import { ToggleSwitch } from './ToggleSwitch'
import { SliderControl } from './SliderControl'

export interface EventsGroupProps {
  visualSettings: Partial<VisualSettings>
  onChange: (patch: Partial<VisualSettings>) => void
}

// The row's label lives in the catalog under `showField`, so the two lists
// cannot drift: a field without a key fails tsc.
const EVENT_ROWS: Array<{
  showField: 'showSuperChat' | 'showSubscriptions' | 'showRaids' | 'showBits' | 'showMembershipGift'
  sizeField: keyof VisualSettings
}> = [
  { showField: 'showSuperChat', sizeField: 'superChatSizeModifier' },
  { showField: 'showSubscriptions', sizeField: 'subscriptionSizeModifier' },
  { showField: 'showRaids', sizeField: 'raidSizeModifier' },
  { showField: 'showBits', sizeField: 'bitsSizeModifier' },
  { showField: 'showMembershipGift', sizeField: 'membershipGiftSizeModifier' },
]

export function EventsGroup({ visualSettings, onChange }: EventsGroupProps): React.ReactElement {
  const t = useTranslations()
  const settings = visualSettings as Record<string, string | undefined>

  return (
    <div className="space-y-4">
      {EVENT_ROWS.map((row) => {
        const showValue = settings[row.showField as string]
        const checked = showValue !== 'none'
        const sizeValue = parseFloat((settings[row.sizeField as string] as string) ?? '1')

        return (
          <div key={row.showField} className="space-y-2">
            <ToggleSwitch
              label={t(`overlayEditor.events.${row.showField}`)}
              checked={checked}
              onChange={(next) => onChange({ [row.showField]: next ? 'block' : 'none' })}
            />
            <SliderControl
              label={t('overlayEditor.events.sizeModifier')}
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
