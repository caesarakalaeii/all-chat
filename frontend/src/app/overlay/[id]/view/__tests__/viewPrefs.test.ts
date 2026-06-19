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
  it('returns all-on defaults when nothing is stored', () => {
    expect(loadViewPrefs()).toEqual(DEFAULT_VIEW_PREFS)
    expect(Object.values(DEFAULT_VIEW_PREFS).every((v) => v === true)).toBe(true)
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
    }
    saveViewPrefs(custom)
    expect(loadViewPrefs()).toEqual(custom)
  })
})
