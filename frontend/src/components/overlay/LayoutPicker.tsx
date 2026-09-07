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

import clsx from 'clsx'
import { PanelBottom, PanelLeft, PanelRight, PanelTop, type LucideIcon } from 'lucide-react'

import type { ViewLayout } from '@/app/overlay/[id]/view/viewLayout'
import { useTranslations } from '@/lib/i18n'

interface LayoutPickerProps {
  layout: ViewLayout
  onChange: (layout: ViewLayout) => void
}

/** The four presets, each with an icon hinting where the chat panel sits. */
// The wire value paired with its catalog key stem, so the label is looked up as
// t(`viewerOverlay.layoutPicker.${messageStem}`). `as const satisfies` rather
// than a type annotation: an annotation widens the stems to string and a typo
// would stop failing tsc.
const OPTIONS = [
  { value: 'chat-left', messageStem: 'chatLeft', Icon: PanelLeft },
  { value: 'chat-right', messageStem: 'chatRight', Icon: PanelRight },
  { value: 'chat-top', messageStem: 'chatTop', Icon: PanelTop },
  { value: 'events-top', messageStem: 'eventsTop', Icon: PanelBottom },
] as const satisfies ReadonlyArray<{ value: ViewLayout; messageStem: string; Icon: LucideIcon }>

/**
 * Compact segmented control for the overlay monitor's layout. Picks one of four
 * Chat | Activity arrangements; the page wires the choice into ResizableSplit and
 * persists it. Visual style matches the other header controls (ViewSettingsBar,
 * Details, theme toggle).
 */
export function LayoutPicker({ layout, onChange }: LayoutPickerProps) {
  const t = useTranslations()
  return (
    <div
      role="radiogroup"
      aria-label={t('viewerOverlay.layoutPicker.groupLabel')}
      className="flex items-center rounded-lg border border-border p-0.5"
    >
      {OPTIONS.map(({ value, messageStem, Icon }) => {
        const label = t(`viewerOverlay.layoutPicker.${messageStem}`)
        const active = layout === value
        return (
          <button
            key={value}
            type="button"
            role="radio"
            aria-checked={active}
            aria-label={label}
            title={label}
            onClick={() => onChange(value)}
            className={clsx(
              'flex items-center rounded-md px-1.5 py-1 transition-colors focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none',
              active ? 'bg-surface-2 text-text' : 'text-text-sub hover:text-text'
            )}
          >
            <Icon className="h-3.5 w-3.5" />
          </button>
        )
      })}
    </div>
  )
}
