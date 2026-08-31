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

import { afterEach, describe, expect, it, vi } from 'vitest'

import { enMessages } from '@/lib/i18n/messages/en'
import { translate } from '@/lib/i18n/translate'

afterEach(() => {
  vi.restoreAllMocks()
})

describe('translate', () => {
  it('resolves a nested key to its string', () => {
    expect(translate(enMessages, 'a11y.skipToMainContent')).toBe('Skip to main content')
  })

  it('interpolates a single-brace placeholder', () => {
    expect(translate(enMessages, 'maintenanceBanner.dismissLabel', { title: 'DB upgrade' })).toBe(
      'Dismiss maintenance banner: DB upgrade'
    )
  })

  it('replaces every occurrence of a placeholder used twice', () => {
    // The catalog has no doubled placeholder, so this asserts the interpolation
    // contract against a stand-in catalog of the same shape.
    const messages = { test: { doubled: '{word} and {word}' } } as const
    expect(translate(messages, 'test.doubled', { word: 'echo' })).toBe('echo and echo')
  })

  it('falls back to the key string when the key is unknown', () => {
    vi.spyOn(console, 'warn').mockImplementation(() => {})
    const messages = { common: { cancel: 'Cancel' } } as const
    // @ts-expect-error - 'common.nope' is not a key of this catalog, which is the
    // compile-time guard: if MessageKey ever widens to string this line stops
    // erroring and tsc --noEmit fails on the unused @ts-expect-error.
    expect(translate(messages, 'common.nope')).toBe('common.nope')
  })

  it('warns outside production when the key is unknown', () => {
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
    const messages = { common: { cancel: 'Cancel' } } as const
    // @ts-expect-error - unknown key, see above.
    translate(messages, 'common.nope')
    expect(warn).toHaveBeenCalledOnce()
  })

  it('leaves the placeholder in place when a param is missing', () => {
    vi.spyOn(console, 'warn').mockImplementation(() => {})
    expect(translate(enMessages, 'maintenanceBanner.dismissLabel')).toBe(
      'Dismiss maintenance banner: {title}'
    )
  })

  it('warns outside production when a param is missing', () => {
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
    translate(enMessages, 'maintenanceBanner.dismissLabel')
    expect(warn).toHaveBeenCalledOnce()
  })

  it('does not throw when the resolved value is a namespace rather than a string', () => {
    vi.spyOn(console, 'warn').mockImplementation(() => {})
    const messages = { common: { cancel: 'Cancel' } } as const
    // @ts-expect-error - 'common' is a namespace, not a leaf key.
    expect(translate(messages, 'common')).toBe('common')
  })
})
