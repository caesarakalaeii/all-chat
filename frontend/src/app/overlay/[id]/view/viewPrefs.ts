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

import { PRESET_NAMES, type PresetName } from '@/lib/utils/soundPlayer'

export interface MonitorViewPrefs {
  showAvatars: boolean
  showBadges: boolean
  showPronouns: boolean
  showTimestamps: boolean
  showPlatformGlyph: boolean
  showModeration: boolean
  /**
   * Play a short sound in THIS browser when a new audience-activity item lands
   * in the Activity & Events panel (channel-point redeems, gifts/roses, follows,
   * subs, raids, …). This is a private aid for the moderator so easy-to-miss
   * activity is noticed; it is entirely separate from the overlay's on-stream
   * "Notification Sounds" and never plays on the public OBS overlay. Off by
   * default so the dashboard stays silent until the moderator opts in.
   */
  activitySoundEnabled: boolean
  /** Playback volume (0–1) for the activity sound. */
  activitySoundVolume: number
  /** Which built-in sound to play. Reuses the overlay sound presets. */
  activitySoundPreset: PresetName
}

export const VIEW_PREFS_KEY = 'overlay-view-prefs'

export const DEFAULT_VIEW_PREFS: MonitorViewPrefs = {
  showAvatars: true,
  showBadges: true,
  showPronouns: true,
  showTimestamps: true,
  showPlatformGlyph: true,
  showModeration: true,
  activitySoundEnabled: false,
  activitySoundVolume: 0.5,
  activitySoundPreset: 'ping',
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
    const merged = { ...DEFAULT_VIEW_PREFS, ...parsed }
    // Sanitize the two fields that drive the audio element directly, so a
    // corrupt payload can't request a missing sound file or an out-of-range gain.
    if (!PRESET_NAMES.includes(merged.activitySoundPreset)) {
      merged.activitySoundPreset = DEFAULT_VIEW_PREFS.activitySoundPreset
    }
    if (typeof merged.activitySoundVolume !== 'number' || !Number.isFinite(merged.activitySoundVolume)) {
      merged.activitySoundVolume = DEFAULT_VIEW_PREFS.activitySoundVolume
    } else {
      merged.activitySoundVolume = Math.min(1, Math.max(0, merged.activitySoundVolume))
    }
    return merged
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
