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
 * Layout presets for the overlay monitor's Chat | Activity split (`.../view`).
 *
 * Each preset maps to an `orientation` + `reversed` pair understood by
 * `ResizableSplit`. Like `MonitorViewPrefs`, the chosen preset is a moderator's
 * personal, browser-local preference (persisted only to localStorage) and never
 * touches the overlay's saved settings.
 */

import type { SplitOrientation } from '@/components/ResizableSplit'

/** Identifier for one of the four split layouts offered in the view header. */
export type ViewLayout = 'chat-left' | 'chat-right' | 'events-top' | 'chat-top'

export interface LayoutConfig {
  orientation: SplitOrientation
  reversed: boolean
}

export const DEFAULT_VIEW_LAYOUT: ViewLayout = 'chat-left'

/**
 * Maps each preset to the `ResizableSplit` orientation/reversed pair. Recall
 * the split's first child is `left` when `reversed` is false, else `right`; the
 * page passes Chat as `left` and Activity as `right`.
 */
export const LAYOUT_CONFIG: Record<ViewLayout, LayoutConfig> = {
  // Chat left, events right — the original default.
  'chat-left': { orientation: 'horizontal', reversed: false },
  // Chat right, events left.
  'chat-right': { orientation: 'horizontal', reversed: true },
  // Events on top, chat below (reversed puts Activity first).
  'events-top': { orientation: 'vertical', reversed: true },
  // Chat on top, events below.
  'chat-top': { orientation: 'vertical', reversed: false },
}

const VALID_LAYOUTS: ReadonlySet<string> = new Set<ViewLayout>([
  'chat-left',
  'chat-right',
  'events-top',
  'chat-top',
])

/** localStorage key prefix; the overlay id is appended per the task spec. */
export function layoutStorageKey(overlayId: string): string {
  return `overlay-view-layout-${overlayId}`
}

/** Load the saved layout for an overlay, falling back to the default. */
export function loadViewLayout(overlayId: string): ViewLayout {
  if (typeof window === 'undefined') return DEFAULT_VIEW_LAYOUT
  try {
    const raw = localStorage.getItem(layoutStorageKey(overlayId))
    if (raw && VALID_LAYOUTS.has(raw)) return raw as ViewLayout
  } catch {
    /* storage unavailable — fall back to default */
  }
  return DEFAULT_VIEW_LAYOUT
}

/** Persist the layout for an overlay. No-ops if storage is unavailable. */
export function saveViewLayout(overlayId: string, layout: ViewLayout): void {
  if (typeof window === 'undefined') return
  try {
    localStorage.setItem(layoutStorageKey(overlayId), layout)
  } catch {
    /* storage unavailable; selection stays in-session only */
  }
}
