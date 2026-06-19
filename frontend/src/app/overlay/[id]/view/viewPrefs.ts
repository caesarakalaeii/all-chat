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
 * View-local display preferences for the overlay monitor (`.../view`).
 *
 * These are a MODERATOR'S personal preferences for how the readable monitor
 * renders — they are persisted only to localStorage and MUST NEVER be written
 * to the overlay's `visual_settings` (which drive the public OBS overlay). They
 * only affect this view in this browser. Mirrors the `THEME_KEY` pattern in
 * `view/page.tsx`.
 */

export interface MonitorViewPrefs {
  showAvatars: boolean
  showBadges: boolean
  showPronouns: boolean
  showTimestamps: boolean
  showPlatformGlyph: boolean
  showModeration: boolean
}

export const VIEW_PREFS_KEY = 'overlay-view-prefs'

export const DEFAULT_VIEW_PREFS: MonitorViewPrefs = {
  showAvatars: true,
  showBadges: true,
  showPronouns: true,
  showTimestamps: true,
  showPlatformGlyph: true,
  showModeration: true,
}

/**
 * Load saved prefs, merged over the defaults so a partial or older payload (or
 * missing/corrupt storage) still yields a complete, valid object.
 */
export function loadViewPrefs(): MonitorViewPrefs {
  if (typeof window === 'undefined') return { ...DEFAULT_VIEW_PREFS }
  try {
    const raw = localStorage.getItem(VIEW_PREFS_KEY)
    if (!raw) return { ...DEFAULT_VIEW_PREFS }
    const parsed = JSON.parse(raw) as Partial<MonitorViewPrefs>
    if (!parsed || typeof parsed !== 'object') return { ...DEFAULT_VIEW_PREFS }
    return { ...DEFAULT_VIEW_PREFS, ...parsed }
  } catch {
    return { ...DEFAULT_VIEW_PREFS }
  }
}

/** Persist prefs to localStorage. No-ops if storage is unavailable. */
export function saveViewPrefs(prefs: MonitorViewPrefs): void {
  if (typeof window === 'undefined') return
  try {
    localStorage.setItem(VIEW_PREFS_KEY, JSON.stringify(prefs))
  } catch {
    /* storage unavailable; prefs stay in-session only */
  }
}
