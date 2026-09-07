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
 * Dock mode for the overlay monitor (`.../view?dock=1`).
 *
 * OBS and Streamlabs can render a URL as a custom browser dock — a panel
 * roughly 300-450px wide, docked beside Scenes/Sources. The monitor is the
 * right surface for that, but its wide-viewport chrome (a wrapping header, a
 * stack of full-width notice strips and a two-column `ResizableSplit`) is not.
 * Dock mode is a PRESENTATION mode of that same route: same data path, same
 * moderation, narrower chrome and one panel at a time.
 *
 * The mode is opt-in and keyed on the `dock` query parameter, so every existing
 * link to the monitor keeps today's layout. Like `ViewLayout`, the selected dock
 * tab is a moderator's personal, browser-local preference (localStorage only)
 * and never touches the overlay's saved settings.
 */

/** Which of the two dock panels is showing. */
export type DockTab = 'chat' | 'activity'

/** Chat is what a docked panel is for; Activity is the deliberate switch. */
export const DEFAULT_DOCK_TAB: DockTab = 'chat'

/**
 * Whether the monitor should render in dock mode.
 *
 * Truthy for `dock=1` and `dock=true` only. An unrecognised value (`dock=0`,
 * `dock=yes`, a typo) is NOT dock mode: a mistyped parameter must leave the
 * monitor exactly as it is rather than silently reshaping it.
 */
export function isDockMode(params: URLSearchParams): boolean {
  const raw = params.get('dock')
  if (raw === null) return false
  const value = raw.toLowerCase()
  return value === '1' || value === 'true'
}

const VALID_DOCK_TABS: ReadonlySet<string> = new Set<DockTab>(['chat', 'activity'])

/** localStorage key prefix; the overlay id is appended per the task spec. */
export function dockTabStorageKey(overlayId: string): string {
  return `overlay-view-dock-tab-${overlayId}`
}

/** Load the saved dock tab for an overlay, falling back to the default. */
export function loadDockTab(overlayId: string): DockTab {
  if (typeof window === 'undefined') return DEFAULT_DOCK_TAB
  try {
    const raw = localStorage.getItem(dockTabStorageKey(overlayId))
    if (raw && VALID_DOCK_TABS.has(raw)) return raw as DockTab
  } catch {
    /* storage unavailable — fall back to default */
  }
  return DEFAULT_DOCK_TAB
}

/** Persist the dock tab for an overlay. No-ops if storage is unavailable. */
export function saveDockTab(overlayId: string, tab: DockTab): void {
  if (typeof window === 'undefined') return
  try {
    localStorage.setItem(dockTabStorageKey(overlayId), tab)
  } catch {
    /* storage unavailable; selection stays in-session only */
  }
}
