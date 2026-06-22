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
  DEFAULT_VIEW_LAYOUT,
  LAYOUT_CONFIG,
  layoutStorageKey,
  loadViewLayout,
  saveViewLayout,
} from '../viewLayout'

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

describe('viewLayout', () => {
  it('defaults to chat-left when nothing is stored', () => {
    expect(loadViewLayout('o1')).toBe(DEFAULT_VIEW_LAYOUT)
    expect(DEFAULT_VIEW_LAYOUT).toBe('chat-left')
  })

  it('keys storage per overlay id', () => {
    expect(layoutStorageKey('abc')).toBe('overlay-view-layout-abc')
  })

  it('round-trips a saved layout through save → load', () => {
    saveViewLayout('o1', 'events-top')
    expect(loadViewLayout('o1')).toBe('events-top')
    // A different overlay is unaffected.
    expect(loadViewLayout('o2')).toBe(DEFAULT_VIEW_LAYOUT)
  })

  it('falls back to the default for an unknown stored value', () => {
    localStorage.setItem(layoutStorageKey('o1'), 'garbage')
    expect(loadViewLayout('o1')).toBe(DEFAULT_VIEW_LAYOUT)
  })

  it('maps each preset to the expected orientation/reversed pair', () => {
    expect(LAYOUT_CONFIG['chat-left']).toEqual({ orientation: 'horizontal', reversed: false })
    expect(LAYOUT_CONFIG['chat-right']).toEqual({ orientation: 'horizontal', reversed: true })
    expect(LAYOUT_CONFIG['events-top']).toEqual({ orientation: 'vertical', reversed: true })
    expect(LAYOUT_CONFIG['chat-top']).toEqual({ orientation: 'vertical', reversed: false })
  })
})
