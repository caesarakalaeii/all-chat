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

// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import {
  DEFAULT_VIEW_PREFS,
  VIEW_PREFS_KEY,
  loadViewPrefs,
  saveViewPrefs,
  type MonitorViewPrefs,
} from '../viewPrefs'

// jsdom in this project does not ship a working localStorage; stub one.
function stubLocalStorage() {
  const store: Record<string, string> = {}
  vi.stubGlobal('localStorage', {
    getItem: (k: string) => store[k] ?? null,
    setItem: (k: string, v: string) => {
      store[k] = v
    },
    removeItem: (k: string) => {
      delete store[k]
    },
    clear: () => {
      Object.keys(store).forEach((k) => delete store[k])
    },
  })
}

beforeEach(() => stubLocalStorage())
afterEach(() => vi.unstubAllGlobals())

describe('viewPrefs', () => {
  it('returns defaults when nothing is stored: display toggles on, activity sound off', () => {
    expect(loadViewPrefs()).toEqual(DEFAULT_VIEW_PREFS)
    // Display toggles default on so the readable monitor shows everything…
    const displayToggles: Array<keyof MonitorViewPrefs> = [
      'showAvatars',
      'showBadges',
      'showPronouns',
      'showTimestamps',
      'showPlatformGlyph',
      'showModeration',
    ]
    expect(displayToggles.every((k) => DEFAULT_VIEW_PREFS[k] === true)).toBe(true)
    // …but the activity sound is opt-in, so the dashboard is silent by default.
    expect(DEFAULT_VIEW_PREFS.activitySoundEnabled).toBe(false)
  })

  it('merges a partial stored payload over the defaults', () => {
    localStorage.setItem(VIEW_PREFS_KEY, JSON.stringify({ showAvatars: false }))
    const prefs = loadViewPrefs()
    expect(prefs.showAvatars).toBe(false)
    // Every other key keeps its default.
    expect(prefs.showBadges).toBe(true)
    expect(prefs.showModeration).toBe(true)
    expect(prefs.showTimestamps).toBe(true)
  })

  it('gives an older payload (no sound keys) the activity-sound defaults', () => {
    // A payload saved before the activity sound existed has none of its keys.
    localStorage.setItem(VIEW_PREFS_KEY, JSON.stringify({ showAvatars: false }))
    const prefs = loadViewPrefs()
    expect(prefs.activitySoundEnabled).toBe(DEFAULT_VIEW_PREFS.activitySoundEnabled)
    expect(prefs.activitySoundVolume).toBe(DEFAULT_VIEW_PREFS.activitySoundVolume)
    expect(prefs.activitySoundPreset).toBe(DEFAULT_VIEW_PREFS.activitySoundPreset)
  })

  it('preserves valid stored activity-sound settings', () => {
    localStorage.setItem(
      VIEW_PREFS_KEY,
      JSON.stringify({ activitySoundEnabled: true, activitySoundVolume: 0.25, activitySoundPreset: 'pop' })
    )
    const prefs = loadViewPrefs()
    expect(prefs.activitySoundEnabled).toBe(true)
    expect(prefs.activitySoundVolume).toBe(0.25)
    expect(prefs.activitySoundPreset).toBe('pop')
  })

  it('sanitizes an unknown preset back to the default', () => {
    localStorage.setItem(VIEW_PREFS_KEY, JSON.stringify({ activitySoundPreset: 'airhorn' }))
    expect(loadViewPrefs().activitySoundPreset).toBe(DEFAULT_VIEW_PREFS.activitySoundPreset)
  })

  it('clamps an out-of-range or non-numeric volume', () => {
    localStorage.setItem(VIEW_PREFS_KEY, JSON.stringify({ activitySoundVolume: 5 }))
    expect(loadViewPrefs().activitySoundVolume).toBe(1)
    localStorage.setItem(VIEW_PREFS_KEY, JSON.stringify({ activitySoundVolume: -2 }))
    expect(loadViewPrefs().activitySoundVolume).toBe(0)
    localStorage.setItem(VIEW_PREFS_KEY, JSON.stringify({ activitySoundVolume: 'loud' }))
    expect(loadViewPrefs().activitySoundVolume).toBe(DEFAULT_VIEW_PREFS.activitySoundVolume)
  })

  it('falls back to defaults on corrupt JSON', () => {
    localStorage.setItem(VIEW_PREFS_KEY, '{not valid json')
    expect(loadViewPrefs()).toEqual(DEFAULT_VIEW_PREFS)
  })

  it('falls back to defaults when the stored value is not an object', () => {
    localStorage.setItem(VIEW_PREFS_KEY, JSON.stringify('nope'))
    expect(loadViewPrefs()).toEqual(DEFAULT_VIEW_PREFS)
  })

  it('round-trips a full prefs object through save → load', () => {
    const custom: MonitorViewPrefs = {
      showAvatars: false,
      showBadges: true,
      showPronouns: false,
      showTimestamps: false,
      showPlatformGlyph: true,
      showModeration: false,
      activitySoundEnabled: true,
      activitySoundVolume: 0.8,
      activitySoundPreset: 'chime',
    }
    saveViewPrefs(custom)
    expect(loadViewPrefs()).toEqual(custom)
  })
})
