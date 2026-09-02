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

// No `@vitest-environment jsdom` here, unlike the sibling viewLayout/viewPrefs
// tests: the `unit` project runs in `node`, and four DOM-dependent test files
// already fail there for want of a working `window`. `dockMode` is a pure
// module, so it is testable by stubbing the two globals it touches — a fake
// `window` (the SSR guard) and a fake `localStorage`.
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import {
  DEFAULT_DOCK_TAB,
  dockTabStorageKey,
  isDockMode,
  loadDockTab,
  saveDockTab,
} from '../dockMode'

/** Stub the browser globals the module reads. Returns the backing map. */
function stubStorage(): Record<string, string> {
  const store: Record<string, string> = {}
  vi.stubGlobal('window', {})
  vi.stubGlobal('localStorage', {
    getItem: (key: string) => store[key] ?? null,
    setItem: (key: string, value: string) => {
      store[key] = value
    },
    removeItem: (key: string) => {
      delete store[key]
    },
    clear: () => {
      Object.keys(store).forEach((key) => delete store[key])
    },
  })
  return store
}

/** Stub a storage that throws on every access (Safari private mode, OBS CEF). */
function stubThrowingStorage() {
  vi.stubGlobal('window', {})
  vi.stubGlobal('localStorage', {
    getItem: () => {
      throw new Error('storage disabled')
    },
    setItem: () => {
      throw new Error('storage disabled')
    },
  })
}

beforeEach(() => stubStorage())
afterEach(() => vi.unstubAllGlobals())

describe('isDockMode', () => {
  it('is false when the dock param is absent', () => {
    expect(isDockMode(new URLSearchParams(''))).toBe(false)
    expect(isDockMode(new URLSearchParams('foo=1'))).toBe(false)
  })

  it('is true for dock=1 and dock=true, case-insensitively', () => {
    expect(isDockMode(new URLSearchParams('dock=1'))).toBe(true)
    expect(isDockMode(new URLSearchParams('dock=true'))).toBe(true)
    expect(isDockMode(new URLSearchParams('dock=TRUE'))).toBe(true)
    expect(isDockMode(new URLSearchParams('dock=True'))).toBe(true)
  })

  it('is false for any other value, so a typo does not silently change the view', () => {
    expect(isDockMode(new URLSearchParams('dock=0'))).toBe(false)
    expect(isDockMode(new URLSearchParams('dock=false'))).toBe(false)
    expect(isDockMode(new URLSearchParams('dock='))).toBe(false)
    expect(isDockMode(new URLSearchParams('dock=yes'))).toBe(false)
  })
})

describe('dockTab persistence', () => {
  it('defaults to the chat tab when nothing is stored', () => {
    expect(loadDockTab('o1')).toBe(DEFAULT_DOCK_TAB)
    expect(DEFAULT_DOCK_TAB).toBe('chat')
  })

  it('keys storage per overlay id', () => {
    // Literal prefix is a contract with the page and the E2E spec.
    expect(dockTabStorageKey('abc')).toBe('overlay-view-dock-tab-abc')
  })

  it('round-trips a saved tab through save → load', () => {
    saveDockTab('o1', 'activity')
    expect(loadDockTab('o1')).toBe('activity')
    // A different overlay keeps its own tab.
    expect(loadDockTab('o2')).toBe(DEFAULT_DOCK_TAB)
  })

  it('falls back to the default for an unknown stored value', () => {
    localStorage.setItem(dockTabStorageKey('o1'), 'garbage')
    expect(loadDockTab('o1')).toBe(DEFAULT_DOCK_TAB)
  })

  it('returns the default and does not throw when storage is unavailable', () => {
    stubThrowingStorage()
    expect(loadDockTab('o1')).toBe(DEFAULT_DOCK_TAB)
    expect(() => saveDockTab('o1', 'activity')).not.toThrow()
  })
})
