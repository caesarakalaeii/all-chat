/**
 * This file is part of All-Chat.
 * Copyright (C) 2026 caesarakalaeii
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published
 * by the Free Software Foundation, either version 3 of the License, or
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
  THEME_INIT_SCRIPT,
  THEME_STORAGE_KEY,
  chooseTheme,
  getTheme,
  isThemedPath,
  subscribeToTheme,
  syncTheme,
} from '../theme'

function memoryStorage(): Storage {
  const data = new Map<string, string>()
  return {
    get length() {
      return data.size
    },
    clear: () => data.clear(),
    getItem: (k) => data.get(k) ?? null,
    key: (i) => [...data.keys()][i] ?? null,
    removeItem: (k) => void data.delete(k),
    setItem: (k, v) => void data.set(k, String(v)),
  }
}

const attr = () => document.documentElement.dataset.theme

beforeEach(() => {
  vi.stubGlobal('localStorage', memoryStorage())
  delete document.documentElement.dataset.theme
  window.history.replaceState(null, '', '/')
})

afterEach(() => {
  chooseTheme('dark')
  vi.unstubAllGlobals()
})

describe('isThemedPath', () => {
  it.each(['/', '/legal/terms', '/dashboard', '/overlays/abc', '/overlays/new', '/admin/users'])(
    '%s takes the site theme',
    (path) => expect(isThemedPath(path)).toBe(true)
  )

  // OBS browser sources and their previews are broadcast art: they stay as
  // the streamer styled them whatever this browser chose for the site.
  it.each([
    '/overlay/abc',
    '/overlay/abc/view',
    '/overlay/abc/credits',
    '/overlay/abc/participate',
    '/overlays/abc/preview/embed',
    '/overlays/abc/preview/alerts',
  ])('%s never does', (path) => expect(isThemedPath(path)).toBe(false))
})

describe('chooseTheme', () => {
  it('applies, remembers and announces the choice', () => {
    const listener = vi.fn()
    const unsubscribe = subscribeToTheme(listener)

    chooseTheme('light')
    expect(attr()).toBe('light')
    expect(localStorage.getItem(THEME_STORAGE_KEY)).toBe('light')
    expect(getTheme()).toBe('light')
    expect(listener).toHaveBeenCalledTimes(1)

    chooseTheme('dark')
    expect(attr()).toBeUndefined()
    expect(localStorage.getItem(THEME_STORAGE_KEY)).toBe('dark')
    expect(getTheme()).toBe('dark')
    unsubscribe()
  })

  it('still switches this tab when storage is blocked', () => {
    vi.stubGlobal('localStorage', {
      getItem: () => {
        throw new Error('blocked')
      },
      setItem: () => {
        throw new Error('blocked')
      },
    })
    chooseTheme('light')
    expect(attr()).toBe('light')
  })

  it('leaves an unthemed page dark', () => {
    window.history.replaceState(null, '', '/overlay/abc')
    chooseTheme('light')
    expect(attr()).toBeUndefined()
  })
})

describe('syncTheme', () => {
  it('drops the theme on an unthemed route and restores it on the way back', () => {
    chooseTheme('light')
    syncTheme('/overlay/abc/view')
    expect(attr()).toBeUndefined()
    syncTheme('/dashboard')
    expect(attr()).toBe('light')
  })
})

describe('THEME_INIT_SCRIPT', () => {
  const run = () => new Function(THEME_INIT_SCRIPT)()

  it('applies a stored light choice before the first paint', () => {
    localStorage.setItem(THEME_STORAGE_KEY, 'light')
    window.history.replaceState(null, '', '/legal/terms')
    run()
    expect(attr()).toBe('light')
  })

  it('does nothing on an unthemed route', () => {
    localStorage.setItem(THEME_STORAGE_KEY, 'light')
    window.history.replaceState(null, '', '/overlay/abc')
    run()
    expect(attr()).toBeUndefined()
  })

  it('stays dark without a stored choice or with a junk one', () => {
    run()
    expect(attr()).toBeUndefined()
    localStorage.setItem(THEME_STORAGE_KEY, 'neon')
    run()
    expect(attr()).toBeUndefined()
  })

  it('survives blocked storage', () => {
    vi.stubGlobal('localStorage', {
      getItem: () => {
        throw new Error('blocked')
      },
    })
    expect(run).not.toThrow()
    expect(attr()).toBeUndefined()
  })
})
