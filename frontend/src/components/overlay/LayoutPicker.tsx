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

interface LayoutPickerProps {
  layout: ViewLayout
  onChange: (layout: ViewLayout) => void
}

/** The four presets, each with an icon hinting where the chat panel sits. */
const OPTIONS: ReadonlyArray<{ value: ViewLayout; label: string; Icon: LucideIcon }> = [
  { value: 'chat-left', label: 'Chat left, events right', Icon: PanelLeft },
  { value: 'chat-right', label: 'Chat right, events left', Icon: PanelRight },
  { value: 'chat-top', label: 'Chat top, events below', Icon: PanelTop },
  { value: 'events-top', label: 'Events top, chat below', Icon: PanelBottom },
]

/**
 * Compact segmented control for the overlay monitor's layout. Picks one of four
 * Chat | Activity arrangements; the page wires the choice into ResizableSplit and
 * persists it. Visual style matches the other header controls (ViewSettingsBar,
 * Details, theme toggle).
 */
export function LayoutPicker({ layout, onChange }: LayoutPickerProps) {
  return (
    <div
      role="radiogroup"
      aria-label="Panel layout"
      className="flex items-center rounded-lg border border-border p-0.5"
    >
      {OPTIONS.map(({ value, label, Icon }) => {
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
              active
                ? 'bg-surface-2 text-text'
                : 'text-text-sub hover:text-text',
            )}
          >
            <Icon className="h-3.5 w-3.5" />
          </button>
        )
      })}
    </div>
  )
}
